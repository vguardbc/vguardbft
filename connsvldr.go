package main

import (
	"encoding/gob"
	"errors"
	"io"
	"net"
	"time"
)

//validators' connections:
func runAsValidator() {
	defer proposerLookup.RUnlock()
	proposerLookup.RLock()

	registerDialConn(proposerLookup.m[OPA], OPA, ListenerPortOPA)
	registerDialConn(proposerLookup.m[OPB], OPB, ListenerPortOPB)
	registerDialConn(proposerLookup.m[CPA], CPA, ListenerPortOCA)
	registerDialConn(proposerLookup.m[CPB], CPB, ListenerPortOCB)

	log.Debugf("... registerDialConn completed ...")

	go receivingOADialMessages(proposerLookup.m[OPA])
	go receivingOBDialMessages(proposerLookup.m[OPB])
	go receivingCADialMessages(proposerLookup.m[CPA])
	go receivingCBDialMessages(proposerLookup.m[CPB])
}

func registerDialConn(coordinatorId ServerId, phaseNumber Phase, portNumber int) {
	coordinatorIp := ServerList[coordinatorId].Ip
	coordinatorListenerPort := ServerList[coordinatorId].Ports[portNumber]
	coordinatorAddress := coordinatorIp + ":" + coordinatorListenerPort

	conn, err := establishDialConn(coordinatorAddress, int(phaseNumber))
	if err != nil {
		log.Errorf("dialog to coordinator %v failed | error: %v", phaseNumber, err)
		return
	}

	log.Infof("dial conn of Phase %d has established | remote addr: %s", phaseNumber, conn.RemoteAddr().String())

	dialogMgr.Lock()
	dialogMgr.conns[phaseNumber][coordinatorId] = ConnDock{
		SID:  coordinatorId,
		conn: conn,
		enc:  gob.NewEncoder(conn),
		dec:  gob.NewDecoder(conn),
	}
	dialogMgr.Unlock()

	log.Infof("dial conn of Phase %d has registered | dialogMgr.conns[phaseNumber: %d][coordinatorId: %d]: localconn: %s, remoteconn: %s",
		phaseNumber, phaseNumber, coordinatorId, dialogMgr.conns[phaseNumber][coordinatorId].conn.LocalAddr().String(),
		dialogMgr.conns[phaseNumber][coordinatorId].conn.RemoteAddr().String())
}

func establishDialConn(coordListenerAddr string, phase int) (*net.TCPConn, error) {
	var e error

	coordTCPListenerAddr, err := net.ResolveTCPAddr("tcp4", coordListenerAddr)
	if err != nil {
		panic(err)
	}

	ServerList[ServerID].RLock()

	var myDialAddr string
	myDialAdrIp := ServerList[ServerID].Ip

	switch phase {
	case OPA:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortOPA]
	case OPB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortOPB]
	case CPA:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortCPA]
	case CPB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortCPB]
	default:
		panic(errors.New("wrong phase name"))
	}

	ServerList[ServerID].RUnlock()

	myTCPDialAddr, err := net.ResolveTCPAddr("tcp4", myDialAddr)

	if err != nil {
		panic(err)
	}

	maxTry := 10
	for i := 0; i < maxTry; i++ {
		conn, err := net.DialTCP("tcp4", myTCPDialAddr, coordTCPListenerAddr)

		if err != nil {
			log.Errorf("Dial Leader failed | err: %v | maxTry: %v | retry: %vth\n", err, maxTry, i)
			time.Sleep(1 * time.Second)
			e = err
			continue
		}
		return conn, nil
	}

	return nil, e
}

func receivingOADialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	postPhaseDialogInfo := dialogMgr.conns[OPA][coordinatorId]
	orderPhaseDialogInfo := dialogMgr.conns[OPB][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerOPAEntry

		err := postPhaseDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v | coordinator closed connection | err: %v", time.Now(), err)
			log.Warnf("Lost connection with the proposer (S%v); quitting program", postPhaseDialogInfo.SID)
			vgInst.Done()
			break
		}

		if err != nil {
			log.Errorf("Gob Decode Err: %v", err)
			continue
		}

		go validatingOAEntry(&m, orderPhaseDialogInfo.enc)
	}
}

func receivingOBDialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	orderPhaseDialogInfo := dialogMgr.conns[OPB][coordinatorId]
	commitPhaseDialogInfo := dialogMgr.conns[CPA][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerOPBEntry

		err := orderPhaseDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%s | coordinator closed connection | err: %v", rpyPhase[OPB], err)
			break
		}

		if err != nil {
			log.Errorf("%s | gob Decode Err: %v", rpyPhase[OPB], err)
			continue
		}

		go validatingOBEntry(&m, commitPhaseDialogInfo.enc)
	}
}

func receivingCADialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	CADialogInfo := dialogMgr.conns[CPA][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerCPAEntry

		err := CADialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v: Coordinator closed connection | err: %v", rpyPhase[CPA], err)
			break
		}

		if err != nil {
			log.Errorf("%v: Gob Decode Err: %v", rpyPhase[CPA], err)
			continue
		}

		go validatingCAEntry(&m, CADialogInfo.enc)
	}
}

func receivingCBDialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	CBDialogInfo := dialogMgr.conns[CPB][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerCPBEntry

		err := CBDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v: Coordinator closed connection | err: %v", rpyPhase[CPB], err)
			break
		}

		if err != nil {
			log.Errorf("%v: Gob Decode Err: %v", rpyPhase[CPB], err)
			continue
		}

		go validatingCBEntry(&m, CBDialogInfo.enc)
	}
}
