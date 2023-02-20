package main

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func runAsProposer(proposerId ServerId) {

	var wg sync.WaitGroup
	wg.Add(NOP)

	for i := 0; i < NOP; i++ {
		go acceptConns(proposerId, &wg, i)
	}

	wg.Wait()
	log.Infof("Network connections are now set | # of phases: %v", NOP)

	prepareBooths(NumOfConn, BoothSize)

	txGenerator(MsgSize)

	for i := 0; i < NumOfValidators; i++ {
		go startOrderingPhaseA(i)
	}

	go startConsensusPhaseA()
}

func closeTCPListener(l *net.TCPListener, phaseNum int) {
	err := (*l).Close()
	if err != nil {
		log.Errorf("close Phase %v TCP listener failed | err: %v", phaseNum, err)
	}
}

func acceptConns(leaderId ServerId, wg *sync.WaitGroup, phase int) {
	addr, err := net.ResolveTCPAddr("tcp4", ServerList[leaderId].Ip+":"+ServerList[leaderId].Ports[phase])
	if err != nil {
		log.Error(err)
		return
	}
	listener, err := net.ListenTCP("tcp4", addr)

	if err != nil {
		log.Error(err)
		return
	}
	defer closeTCPListener(listener, phase)
	log.Infof("%s | listener is up at %s", cmdPhase[phase], listener.Addr().String())

	connected := 0
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Errorf("%s listener err: %v", cmdPhase[phase], err)
			return
		}

		sid, err := connRegistration(*conn, phase)
		if err != nil {
			log.Errorf("server registration err: %v", err)
			return
		}

		switch phase {
		case OPA:
			go handleOPAConns(conn, sid)
		case OPB:
			go handleOPBConns(conn, sid)
		case CPA:
			go handleCPAConns(conn, sid)
		case CPB:
			go handleCPBConns(conn, sid)
		}

		connected++
		if connected == NumOfConn-1 {
			log.Debugf("%s listener | all %d expected servers connected", cmdPhase[phase], connected)
			wg.Done()
			//break
		}
	}
}

func handleOPAConns(sConn *net.TCPConn, sid ServerId) {
	//
	//
}

func handleOPBConns(sConn *net.TCPConn, sid ServerId) {
	for {
		var m ValidatorOPAReply

		if err := concierge.n[OPB][sid].dec.Decode(&m); err == nil {
			go asyncHandleOBReply(&m, sid)
		} else if err == io.EOF {
			log.Errorf("%s | server %v closed connection | err: %v", cmdPhase[OPB], sid, err)
			break
		} else {
			log.Errorf("%s | gob decode Err: %v | conn with ser: %v | remoteAddr: %v",
				cmdPhase[OPB], err, sid, (*sConn).RemoteAddr())
			continue
		}
	}
}

func handleCPAConns(sConn *net.TCPConn, sid ServerId) {

	receiveCounter := int64(0)

	for {
		var m ValidatorCPAReply

		err := concierge.n[CPA][sid].dec.Decode(&m)

		counter := atomic.AddInt64(&receiveCounter, 1)

		if err == io.EOF {
			log.Errorf("%v | server %v closed connection | err: %v", time.Now(), sid, err)
			break
		}

		if err != nil {
			log.Errorf("Gob Decode Err: %v | conn with ser: %v | remoteAddr: %v | Now # %v", err, sid, (*sConn).RemoteAddr(), counter)
			continue
		}

		if &m != nil {
			go asyncHandleCPAReply(&m, sid)
		} else {
			log.Errorf("received message is nil")
		}
	}
}

func handleCPBConns(sConn *net.TCPConn, sid ServerId) {
	//
	//
}
