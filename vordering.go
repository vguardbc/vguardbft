package main

import (
	"encoding/hex"
	"io"
	"net"
	"sync"
	"time"
)

func startOrderingPhaseA(i int) {

	shuffle := struct {
		sync.RWMutex
		counter int
		entries map[int]Entry
	}{
		counter: 0,
		entries: make(map[int]Entry)}
	cycle := 0
	slowModeCycle := 0

	for {
		cycle++
		m, ok := <-requestQueue[i]

		if !ok {
			log.Infof("requestQueue closed, quiting leader service (server %d)", ThisServerID)
			return
		}
		//pack proposes in a block
		entry := Entry{
			TimeStamp: m.Timestamp,
			Tx:        m.Transaction,
		}

		// No need to lock the map, it works chronologically in each thread.
		shuffle.entries[shuffle.counter] = entry

		shuffle.counter++
		if shuffle.counter < BatchSize {
			continue
		}

		bytesOfBatchedEntries, err := serialization(shuffle.entries)
		if err != nil {
			log.Errorf("serialization failed: %v", err)
			return
		}

		hashOfBatchedEntries := getDigest(bytesOfBatchedEntries)

		if err != nil {
			log.Errorf("%s| threshold signing failed | err: %v\n", cmdPhase[OrderingPhaseA], err)
			return
		}

		newBlockId := getLogIndex()

		postEntry := LeaderOrderingAEntry{
			BlockId: newBlockId,
			//PartialSignature: leaderPartialSignature,
			HashOfBatchedEntries: hashOfBatchedEntries,
			//Entries:   shuffle.entries,
		}

		if !slowModeFlag {
			if newBlockId%LatencySampleInterval == 0 {
				metre.recordStartTime(newBlockId, time.Now().UnixMilli())
			}
		}

		incrementLogIndex()

		blockOrderFrag := blockSnapshot{
			hashOfEntriesInBlock: postEntry.HashOfBatchedEntries,
			entriesInBlock:       shuffle.entries,
			concatThreshSig:      [][]byte{},
			counter:              0,
		}

		blockCommitFrag := blockSnapshot{
			hashOfEntriesInBlock: postEntry.HashOfBatchedEntries,
			entriesInBlock:       shuffle.entries,
			concatThreshSig:      nil,
			counter:              0,
		}

		ordSnapshot.Lock()
		ordSnapshot.m[postEntry.BlockId] = &blockOrderFrag
		ordSnapshot.Unlock()

		cmtSnapshot.Lock()
		cmtSnapshot.m[postEntry.BlockId] = &blockCommitFrag
		cmtSnapshot.Unlock()

		//clear shuffle variables
		shuffle.counter = 0
		shuffle.entries = make(map[int]Entry)

		broadcast(postEntry, OrderingPhaseA)

		slowModeCycle++
		if slowModeCycle > SlowModeCycleNum {
			slowModeFlag = false
		}

		if CycleEaseSending != 0 {

			if slowModeFlag {
				time.Sleep(time.Duration(SleepTimeInSlowMode) * time.Second)
				log.Warnf("<! In Slow Mode !> current cycle: %d| ends at: %d", slowModeCycle, SlowModeCycleNum)
			}

			if cycle%CycleEaseSending == 0 {
				time.Sleep(time.Duration(EasingDuration) * time.Millisecond)
			}
		}
		//t_after_broadcast := time.Now()
		log.Debugf("%s | after postEntry broadcast | time : %v", cmdPhase[OrderingPhaseA], time.Now().UTC().String())
		//latencyMeters(cmdPhase[OrderingPhaseA], "phase total time", t_phase_start)
		log.Debugf("new PostEntryBlock broadcast -> b_id: %d | b_hash: %s | sig: %s",
			postEntry.BlockId, hex.EncodeToString(postEntry.HashOfBatchedEntries), hex.EncodeToString(postEntry.PartialSignature))
	}
}

func handleOrderingPhaseAServerConn(sConn *net.Conn) {
	sid, err := registerIncomingWorkerServers(sConn, OrderingPhaseA)
	if err != nil {
		log.Errorf("%s | err: %v | incoming conn Addr: %v",
			cmdPhase[OrderingPhaseA], err, (*sConn).RemoteAddr())
		return
	}
	log.Debugf("sid -> %v", sid)
}

func handleOrderingPhaseBServerConn(sConn *net.Conn) {
	// Handle PostReply from servers
	sid, err := registerIncomingWorkerServers(sConn, OrderingPhaseB)
	if err != nil {
		log.Errorf("%s | err: %v | incoming conn Addr: %v",
			cmdPhase[OrderingPhaseB], err, (*sConn).RemoteAddr())
		return
	}

	for {
		var m WorkerOrderingAReply

		if err := serverbooth.n[OrderingPhaseB][sid].dec.Decode(&m); err == nil {
			go asyncHandleOBReply(&m, sid)
		} else if err == io.EOF {
			log.Errorf("%s | server %v closed connection | err: %v",
				cmdPhase[OrderingPhaseB], sid, err)
			break
		} else {
			log.Errorf("%s | gob decode Err: %v | conn with ser: %v | remoteAddr: %v",
				cmdPhase[OrderingPhaseB], err, sid, (*sConn).RemoteAddr())
			continue
		}
	}
}

func asyncHandleOBReply(m *WorkerOrderingAReply, sid ServerId) {

	ordSnapshot.RLock()
	blockOrderFrag, ok := ordSnapshot.m[m.BlockId]
	ordSnapshot.RUnlock()

	if !ok {
		log.Debugf("%s | no info in cache; consensus may already reached", cmdPhase[OrderingPhaseB])
		return
	}

	if !PeakPerf {
		serverId, err := PenVerifyPartially(blockOrderFrag.hashOfEntriesInBlock, m.PartialSignature)
		if err != nil {
			log.Errorf("%s | PenVerifyPartially server id: %d | err: %v",
				cmdPhase[OrderingPhaseB], serverId, err)
		}
	}

	blockOrderFrag.Lock()
	orderedIndicator := blockOrderFrag.counter

	if orderedIndicator == Threshold {
		blockOrderFrag.Unlock()

		log.Debugf("%s | Block already ordered | orderedIndicator: %v | Threshold: %v | BlockId: %v",
			cmdPhase[OrderingPhaseB], orderedIndicator, Threshold, m.BlockId)
		return
	}

	orderedIndicator++
	blockOrderFrag.counter = orderedIndicator //update orderedCounter
	aggregatedSigShares := append(blockOrderFrag.concatThreshSig, m.PartialSignature)
	blockOrderFrag.concatThreshSig = aggregatedSigShares //update orderedThreshSig

	blockOrderFrag.Unlock()

	if orderedIndicator < Threshold {
		log.Debugf("%s | insufficient votes for ordering | blockId: %v | orderedIndicator: %v",
			cmdPhase[OrderingPhaseB], m.BlockId, orderedIndicator)
		return
	}

	if orderedIndicator > Threshold {
		log.Debugf("%s | block %v already broadcast for ordering", cmdPhase[OrderingPhaseB], m.BlockId)
		return
	}

	sigThreshed, err := PenRecovery(aggregatedSigShares, &blockOrderFrag.hashOfEntriesInBlock)
	if err != nil {
		log.Errorf("%s | PenRecovery failed | len(sigShares): %v | error: %v",
			cmdPhase[OrderingPhaseB], len(aggregatedSigShares), err)
		return
	}

	orderEntry := LeaderOrderingBEntry{
		BlockId:            m.BlockId,
		CombinedSignatures: sigThreshed,
		Entries:            blockOrderFrag.entriesInBlock,
	}

	incrementOrderIndex()
	timeNow := time.Now().UTC().String()

	if !slowModeFlag {
		if m.BlockId%LatencySampleInterval == 0 {
			metre.recordOrderTime(m.BlockId, time.Now().UnixMilli())
		}
	}

	broadcast(orderEntry, OrderingPhaseB)

	if m.BlockId%100 == 0 {
		if m.BlockId == 0 {
			log.Infof("++ ordering block %d ordered (tx: %d) at %s", m.BlockId, BatchSize, timeNow)
		} else {
			log.Infof("++ ordering block %d ordered (tx: %d) at %s", m.BlockId, m.BlockId*int64(BatchSize), timeNow)
		}
	}
}
