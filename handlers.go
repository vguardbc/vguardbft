package main

import (
	"encoding/gob"
	"errors"
	"net"
	"sync"
)

var requestQueue []chan *Proposal

var concierge = struct {
	n [NOP][]*ConnDock // Three phases
	//b  map[int][]int //map[booth#] int{server blockIDs in this booth}
	mu sync.RWMutex
}{}

//var requestQueue = make(chan *Proposal, MaxQueue)

func initConns(numOfServers int) {
	// initialize concierge
	for i := 0; i < len(concierge.n); i++ {
		concierge.n[i] = make([]*ConnDock, numOfServers)
	}

	// initialize dialog conns
	for i := 0; i < len(dialogMgr.conns); i++ {
		dialogMgr.conns[i] = make(map[ServerId]ConnDock)
	}
}

var dialogMgr = struct {
	sync.RWMutex
	conns []map[ServerId]ConnDock
}{conns: make([]map[ServerId]ConnDock, NOP)}

type ConnDock struct {
	SID  ServerId
	conn *net.TCPConn
	enc  *gob.Encoder
	dec  *gob.Decoder
}

func connRegistration(sconn net.TCPConn, phase int) (ServerId, error) {

	concierge.mu.Lock()

	defer concierge.mu.Unlock()
	defer serverIdLookup.RUnlock()

	serverIdLookup.RLock()

	if sid, ok := serverIdLookup.m[sconn.RemoteAddr().String()]; ok {
		concierge.n[phase][sid] = &ConnDock{
			SID:  sid,
			conn: &sconn,
			enc:  gob.NewEncoder(&sconn),
			dec:  gob.NewDecoder(&sconn),
		}

		log.Infof("%s | new server registered | Id: %v -> Addr: %v\n", cmdPhase[phase], sid, sconn.RemoteAddr())
		return sid, nil
	} else {
		return -1, errors.New("incoming connection not recognized")
	}
}

func dialSendBack(m interface{}, encoder *gob.Encoder, phaseNumber int) {
	if encoder == nil {
		log.Errorf("%s | encoder is nil", rpyPhase[phaseNumber])
	}
	if err := encoder.Encode(m); err != nil {
		log.Errorf("%s | send back failed | err: %v", rpyPhase[phaseNumber], err)
	}
}

func takingInitRoles(proposer ServerId) {
	if proposer == ServerId(ServerID) {
		go runAsProposer(proposer)
	} else {
		proposerLookup.Lock()
		for i := 0; i < NOP; i++ {
			proposerLookup.m[Phase(i)] = proposer
		}
		proposerLookup.Unlock()

		go runAsValidator()
	}
}

func start() {
	takingInitRoles(ServerId(0))
}
