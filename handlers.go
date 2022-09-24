package main

import (
	"encoding/gob"
	"errors"
	"io"
	"net"
	"sync"
)

var requestQueue = []chan *ProposeEntry{}

//var requestQueue = make(chan *ProposeEntry, MaxQueueLength)

type latencyMetre struct {
	startMu sync.RWMutex
	orderMu sync.RWMutex

	startTime map[int64]int64
	orderTime map[int64]int64
}

func (m *latencyMetre) init() {
	m.startTime = make(map[int64]int64)
	m.orderTime = make(map[int64]int64)
}

func (m *latencyMetre) recordStartTime(blockId, time int64) {
	defer m.startMu.Unlock()
	m.startMu.Lock()
	m.startTime[blockId] = time
}

func (m *latencyMetre) recordOrderTime(blockId, time int64) {
	defer m.orderMu.Unlock()
	m.orderMu.Lock()
	m.orderTime[blockId] = time
}

func (m *latencyMetre) printConsensusLatency(commitTime, rangeId int64) {
	m.orderMu.Lock()
	n := len(m.orderTime)
	orderingLatency := int64(0)
	consensusLatency := int64(0)

	for i, t := range m.orderTime {

		olat := t - m.startTime[i]
		clat := commitTime - m.startTime[i]

		orderingLatency = orderingLatency + olat
		consensusLatency = consensusLatency + clat

		delete(m.orderTime, i)
		delete(m.startTime, i)
	}
	m.orderMu.Unlock()

	log.Infof("-> Ordering latency (average): %f | Consensus latency (average): %f | rangeId: %d",
		float32(orderingLatency)/float32(n), float32(consensusLatency)/float32(n), rangeId*int64(BatchSize))
}

var serverbooth = struct {
	n  [NOP][]*serverConnDock // Three phases
	mu sync.RWMutex
}{}

func initServerConnNavigatorAndDialogConnRegistryConns(numOfServers int) {
	for i := 0; i < len(serverbooth.n); i++ {
		serverbooth.n[i] = make([]*serverConnDock, numOfServers)
	}
	for i := 0; i < len(DialogConnRegistry.conns); i++ {
		DialogConnRegistry.conns[i] = make(map[ServerId]dialConn)
	}
}

type serverConnDock struct {
	sync.RWMutex
	serverId ServerId
	conn     *net.Conn
	enc      *gob.Encoder
	dec      *gob.Decoder
}

var DialogConnRegistry = struct {
	sync.RWMutex
	conns []map[ServerId]dialConn
}{conns: make([]map[ServerId]dialConn, NOP)}

type dialConn struct {
	sync.RWMutex
	coordId ServerId
	conn    *net.TCPConn
	enc     *gob.Encoder
	dec     *gob.Decoder
}

func registerIncomingWorkerServers(sConn *net.Conn, phase int) (ServerId, error) {

	serverbooth.mu.Lock()

	defer serverbooth.mu.Unlock()

	defer serverConnRegistry.RUnlock()
	//addr[0] is the ip
	serverConnRegistry.RLock()

	if sid, ok := serverConnRegistry.m[(*sConn).RemoteAddr().String()]; ok {
		serverbooth.n[phase][sid] = &serverConnDock{
			serverId: sid,
			conn:     sConn,
			enc:      gob.NewEncoder(*sConn),
			dec:      gob.NewDecoder(*sConn),
		}

		log.Infof("%s | new server registered | Id: %v -> Addr: %v\n",
			cmdPhase[phase], sid, (*sConn).RemoteAddr())
		return sid, nil
	} else {
		return -1, errors.New("incoming connection conf was not loaded :(")
	}
}

var broadcastError = false

func broadcast(e interface{}, phase int) {
	if broadcastError {
		return
	}
	for i := 0; i < len(serverbooth.n[phase]); i++ {
		if ThisServerID == i {
			continue
		}

		if serverbooth.n[phase][i] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, e)
			continue
		}

		//go func(i int) {
		err := serverbooth.n[phase][i].enc.Encode(e)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", serverbooth.n[phase][i].serverId, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", serverbooth.n[phase][i].serverId, err)
			}
		}
		//}(i)
	}
}

func dialSendBack(m interface{}, encoder *gob.Encoder, phaseNumber int) {
	if encoder == nil {
		log.Errorf("%s | encoder is nil", replyPhase[phaseNumber])
	}
	if err := encoder.Encode(m); err != nil {
		log.Errorf("%s | send back failed | err: %v", replyPhase[phaseNumber], err)
	}
}

func start() {
	operatingAsInitialRoles(ServerId(0))
}

func operatingAsInitialRoles(initLeaderId ServerId) {
	if initLeaderId == ServerId(ThisServerID) {
		go runAsLeaderAndAcceptServerConnections(initLeaderId)
	} else {

		coordinatorIdOfPhases.Lock()
		for i := 0; i < NOP; i++ {
			coordinatorIdOfPhases.lookup[Phase(i)] = initLeaderId
		}
		coordinatorIdOfPhases.Unlock()

		go runAsWorkerStartDialingLeader()
	}
}
