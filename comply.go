package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"net"
	"time"
)

func runAsWorkerStartDialingLeader() {
	defer coordinatorIdOfPhases.RUnlock()
	coordinatorIdOfPhases.RLock()

	registerDialConn(coordinatorIdOfPhases.lookup[OrderingPhaseA], OrderingPhaseA, ListenerPortOfOrderingA)
	registerDialConn(coordinatorIdOfPhases.lookup[OrderingPhaseB], OrderingPhaseB, ListenerPortOfOrderingB)
	registerDialConn(coordinatorIdOfPhases.lookup[CommitPhaseA], CommitPhaseA, ListenerPortOfConsensusA)
	registerDialConn(coordinatorIdOfPhases.lookup[CommitPhaseB], CommitPhaseB, ListenerPortOfConsensusB)

	log.Debugf("... registerDialConn completed ...")

	go receivingOADialMessages(coordinatorIdOfPhases.lookup[OrderingPhaseA])
	go receivingOBDialMessages(coordinatorIdOfPhases.lookup[OrderingPhaseB])
	go receivingCADialMessages(coordinatorIdOfPhases.lookup[CommitPhaseA])
	go receivingCBDialMessages(coordinatorIdOfPhases.lookup[CommitPhaseB])
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

	DialogConnRegistry.Lock()
	DialogConnRegistry.conns[phaseNumber][coordinatorId] = dialConn{
		coordId: coordinatorId,
		conn:    conn,
		enc:     gob.NewEncoder(conn),
		dec:     gob.NewDecoder(conn),
	}
	DialogConnRegistry.Unlock()

	log.Infof("dial conn of Phase %d has registered | DialogConnRegistry.conns[phaseNumber: %d][coordinatorId: %d]: localconn: %s, remoteconn: %s",
		phaseNumber, phaseNumber, coordinatorId, DialogConnRegistry.conns[phaseNumber][coordinatorId].conn.LocalAddr().String(),
		DialogConnRegistry.conns[phaseNumber][coordinatorId].conn.RemoteAddr().String())
}

func establishDialConn(coordListenerAddr string, phase int) (*net.TCPConn, error) {
	var e error

	coordTCPListenerAddr, err := net.ResolveTCPAddr("tcp4", coordListenerAddr)
	if err != nil {
		panic(err)
	}

	ServerList[ThisServerID].RLock()

	var myDialAddr string
	myDialAdrIp := ServerList[ThisServerID].Ip

	switch phase {
	case OrderingPhaseA:
		myDialAddr = myDialAdrIp + ":" + ServerList[ThisServerID].Ports[DialPortOfOrderingPhaseA]
	case OrderingPhaseB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ThisServerID].Ports[DialPortOfOrderingPhaseB]
	case CommitPhaseA:
		myDialAddr = myDialAdrIp + ":" + ServerList[ThisServerID].Ports[DialPortOfConsensusPhaseA]
	case CommitPhaseB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ThisServerID].Ports[DialPortOfConsensusPhaseB]
	default:
		panic(errors.New("wrong phase name"))
	}

	ServerList[ThisServerID].RUnlock()

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
	DialogConnRegistry.RLock()
	postPhaseDialogInfo := DialogConnRegistry.conns[OrderingPhaseA][coordinatorId]
	orderPhaseDialogInfo := DialogConnRegistry.conns[OrderingPhaseB][coordinatorId]
	DialogConnRegistry.RUnlock()

	for {
		var m LeaderOrderingAEntry

		err := postPhaseDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v | coordinator closed connection | err: %v", time.Now(), err)
			break
		}

		if err != nil {
			log.Errorf("Gob Decode Err: %v", err)
			continue
		}

		go asyncWorkersHandleOAEntry(&m, orderPhaseDialogInfo.enc)
	}
}

func asyncWorkersHandleOAEntry(m *LeaderOrderingAEntry, encoder *gob.Encoder) {

	//Verify leader's partial signature
	if !PeakPerf {
		id, err := PenVerifyPartially(m.HashOfBatchedEntries, m.PartialSignature)
		if err != nil {
			log.Errorf("%s | PenVerifyPartially failed server id: %d | err: %v",
				replyPhase[OrderingPhaseA], id, err)
			return
		}
		log.Debugf("%s | LeaderOrderingAEntry (BlockId: %d) after PenVerifyPartially %v", replyPhase[OrderingPhaseA], m.BlockId, time.Now().UTC().String())
	}

	//
	// Prepare this server's threshold signature
	replyThreshSig, err := PenSign(m.HashOfBatchedEntries)
	if err != nil {
		log.Errorf("%s | threshold signing failed | err: %v", replyPhase[OrderingPhaseA], err)
		return
	}

	postReply := WorkerOrderingAReply{
		BlockId:          m.BlockId,
		PartialSignature: replyThreshSig,
	}

	blockOrderFrag := blockSnapshot{
		hashOfEntriesInBlock: m.HashOfBatchedEntries,
		//entriesInBlock:       nil,
		//concatThreshSig:      nil,
		//counter:              0,
	}

	//register block order cache
	ordSnapshot.Lock()
	ordSnapshot.m[m.BlockId] = &blockOrderFrag
	log.Debugf("%s: ordSnapshot: %v", replyPhase[OrderingPhaseA], ordSnapshot.m)
	ordSnapshot.Unlock()

	// Workers send PostPhaseReply to Coordinator of the order phase.
	dialSendBack(postReply, encoder, OrderingPhaseA)
}

func receivingOBDialMessages(coordinatorId ServerId) {
	DialogConnRegistry.RLock()
	orderPhaseDialogInfo := DialogConnRegistry.conns[OrderingPhaseB][coordinatorId]
	commitPhaseDialogInfo := DialogConnRegistry.conns[CommitPhaseA][coordinatorId]
	DialogConnRegistry.RUnlock()

	for {
		var m LeaderOrderingBEntry

		err := orderPhaseDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%s | coordinator closed connection | err: %v", replyPhase[OrderingPhaseB], err)
			break
		}

		if err != nil {
			log.Errorf("%s | gob Decode Err: %v", replyPhase[OrderingPhaseB], err)
			continue
		}

		go asyncWorkersHandleOBEntry(&m, commitPhaseDialogInfo.enc)
	}
}

func asyncWorkersHandleOBEntry(m *LeaderOrderingBEntry, encoder *gob.Encoder) {

	ordSnapshot.RLock()
	blockFrag, ok := ordSnapshot.m[m.BlockId]
	ordSnapshot.RUnlock()

	if !ok {
		log.Errorf("%v : block %v not stored in ordSnapshot", replyPhase[OrderingPhaseB], m.BlockId)

		ordSnapshot.RLock()
		log.Debugf("%v | ordSnapshot size: %v", replyPhase[OrderingPhaseB], ordSnapshot.m)
		ordSnapshot.RUnlock()
		return
	}

	//
	// The below scenario is common if occurs.
	// The progress of consensus only needs 2f+1 servers, in which
	// f of them may not have stored the tx in the post phase.
	// In case of segmentation faults, it is crucial to always guard
	// a Map by checking ok.
	log.Debugf("%s | LeaderOrderingBEntry cache fetched (BlockId: %d) @ %v", replyPhase[OrderingPhaseB], m.BlockId, time.Now().UTC().String())

	err := PenVerify(blockFrag.hashOfEntriesInBlock, m.CombinedSignatures)
	if err != nil {
		log.Errorf("%v: PenVerify failed | err: %v", replyPhase[OrderingPhaseB], err)
		return
	}
	log.Debugf("%s | after PenVerify (BlockId: %d) @ %v", replyPhase[OrderingPhaseB], m.BlockId, time.Now().UTC().String())

	blockCommitFrag := blockSnapshot{
		hashOfEntriesInBlock: blockFrag.hashOfEntriesInBlock,
		entriesInBlock:       m.Entries,
		//concatThreshSig:      nil,
		//counter:              0,
	}

	//register block commit cache
	cmtSnapshot.Lock()
	cmtSnapshot.m[m.BlockId] = &blockCommitFrag
	cmtSnapshot.Unlock()

	//orderedIndex := incrementLogIndex()
	incrementLogIndex()

	log.Infof("block %d ordered", m.BlockId)
}

func receivingCADialMessages(coordinatorId ServerId) {
	DialogConnRegistry.RLock()
	CADialogInfo := DialogConnRegistry.conns[CommitPhaseA][coordinatorId]
	DialogConnRegistry.RUnlock()

	for {
		var m LeaderEntryCA

		err := CADialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v: Coordinator closed connection | err: %v", replyPhase[CommitPhaseA], err)
			break
		}

		if err != nil {
			log.Errorf("%v: Gob Decode Err: %v", replyPhase[CommitPhaseA], err)
			continue
		}

		go asyncWorkersHandleCAEntry(&m, CADialogInfo.enc)
	}
}

func asyncWorkersHandleCAEntry(m *LeaderEntryCA, encoder *gob.Encoder) {

	log.Debugf("%s | LeaderEntryCA received (RangeId: %d) @ %v", replyPhase[CommitPhaseA], m.RangeId, time.Now().UTC().String())
	var blockEntries []Entry

	//log.Warnf("startId:%d, rangId:%d, len(map):%d", m.BIDStart, m.RangeId, len(m.RangeHash))

	for i := m.BIDStart; i < m.RangeId-1; i++ {
		cmtSnapshot.RLock()
		blockCmtFrag, ok := cmtSnapshot.m[i]
		cmtSnapshot.RUnlock()

		if !ok {
			log.Debugf("%v | cmtSnapshot.m[%v] not stored in cache|", replyPhase[CommitPhaseA], i)
			continue
		}

		if !bytes.Equal(blockCmtFrag.hashOfEntriesInBlock, m.RangeHash[i]) {
			log.Errorf(" block hashes don't match; received range (Bidstart:%d, rangeId:%d) mapsize:%d;"+
				"received m.RangeHash[%v]: %v | local: %v",
				m.BIDStart, m.RangeId, len(m.RangeHash),
				i, m.RangeHash[i], blockCmtFrag.hashOfEntriesInBlock)
			return
		}

		for _, entry := range blockCmtFrag.entriesInBlock {
			blockEntries = append(blockEntries, entry)
		}
	}

	partialSig, err := PenSign(m.TotalHash)
	if err != nil {
		log.Errorf("PenSign failed: %v", err)
		return
	}

	replyCA := WorkerReplyCA{
		RangeId: m.RangeId,
		ParSig:  partialSig,
	}

	dialSendBack(replyCA, encoder, CommitPhaseA)

	consensusMetaData.Lock()
	consensusMetaData.totalHash[m.RangeId] = m.TotalHash
	consensusMetaData.Unlock()

	consensusData.Lock()
	consensusData.buf[m.RangeId] = blockEntries
	consensusData.Unlock()
}

func receivingCBDialMessages(coordinatorId ServerId) {
	DialogConnRegistry.RLock()
	CBDialogInfo := DialogConnRegistry.conns[CommitPhaseB][coordinatorId]
	DialogConnRegistry.RUnlock()

	for {
		var m LeaderEntryCB

		err := CBDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v: Coordinator closed connection | err: %v", replyPhase[CommitPhaseB], err)
			break
		}

		if err != nil {
			log.Errorf("%v: Gob Decode Err: %v", replyPhase[CommitPhaseB], err)
			continue
		}

		go asyncWorkersHandleCBEntry(&m, CBDialogInfo.enc)
	}
}

func asyncWorkersHandleCBEntry(m *LeaderEntryCB, encoder *gob.Encoder) {

	consensusMetaData.RLock()
	theHash := consensusMetaData.totalHash[m.RangeId]
	consensusMetaData.RUnlock()

	err := PenVerify(theHash, m.ComSig)
	if err != nil {
		log.Errorf("%v | PenVerify failed; err: %v", replyPhase[CommitPhaseB], err)
		return
	}

	storeCommitIndex(m.RangeId)

	consensusData.RLock()
	entries := consensusData.buf[m.RangeId]
	consensusData.RUnlock()

	log.Infof("range %d committed", m.RangeId)
	for _, entry := range entries {
		log.Infof("ts: %d; tx: %v", entry.TimeStamp, string(entry.Tx))
	}

	replyCB := WorkerReplyCB{
		RangeId: m.RangeId,
		done:    true,
	}

	dialSendBack(replyCB, encoder, CommitPhaseB)
}
