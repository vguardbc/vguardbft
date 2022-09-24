package main

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// startConsensusPhaseA follows a policy to conduct consensus.
//
func startConsensusPhaseA() {
	counter := 0
	stopFlag := 0
	for {
		priorRange := getOrderIndex()
		time.Sleep(time.Duration(ConsensusInterval) * time.Millisecond)

		blockIdStart := getCommitIndex()
		rangeId := getOrderIndex()

		if rangeId == 0 || priorRange == rangeId {
			log.Infof("### No new ordered transactions; wait for the next interval (priorRange %d; currentRange %d) | waited for %d times",
				priorRange, rangeId, stopFlag)

			stopFlag++

			if stopFlag < StopFlag {
				continue
			}

			log.Infof("### Waited for %d | Closing consensus phase ", stopFlag)
			return
		}

		timeNow := time.Now().UTC().String()
		log.Infof(">>> consensus instance %d started (starts:%d, ends:%d) at %s", counter, blockIdStart, rangeId, timeNow)
		counter++

		var blockHashesInRange = struct {
			sync.RWMutex
			m map[int64][]byte
		}{m: make(map[int64][]byte)}

		var blockEntries []Entry

		for i := blockIdStart; i < rangeId; i++ {
			cmtSnapshot.Lock()
			blockCmtFrag, ok := cmtSnapshot.m[i]
			cmtSnapshot.Unlock()
			if !ok {
				log.Errorf("cmtSnapshot.m[%d] not exists; current range <%v, %v>; continue", i, blockIdStart, rangeId)
				continue
			}
			blockHashesInRange.Lock()
			blockHashesInRange.m[i] = blockCmtFrag.hashOfEntriesInBlock
			blockHashesInRange.Unlock()

			for _, entry := range blockCmtFrag.entriesInBlock {
				blockEntries = append(blockEntries, entry)
			}

			consensusData.Lock()
			consensusData.buf[rangeId] = blockEntries
			consensusData.Unlock()
		}

		blockHashesInRange.RLock()
		serialized, err := serialization(blockHashesInRange.m)
		blockHashesInRange.RUnlock()

		if err != nil {
			log.Error(err)
			break
		}

		totalHash := getDigest(serialized)

		entryCA := LeaderEntryCA{
			BIDStart:  blockIdStart,
			RangeId:   rangeId,
			RangeHash: blockHashesInRange.m,
			TotalHash: totalHash,
		}

		broadcast(entryCA, CommitPhaseA)

		consensusMetaData.Lock()
		consensusMetaData.totalHash[rangeId] = totalHash
		consensusMetaData.rangeRecorder[rangeId] = blockIdStart
		consensusMetaData.Unlock()
	}
}

func handleCommitPhaseAServerConn(sConn *net.Conn) {

	sid, err := registerIncomingWorkerServers(sConn, CommitPhaseA)

	if err != nil {
		log.Errorf("%s | err: %v | incoming conn Addr: %v",
			cmdPhase[CommitPhaseA], err, (*sConn).RemoteAddr())
		return
	}

	receiveCounter := int64(0)

	for {
		var m WorkerReplyCA

		err := serverbooth.n[CommitPhaseA][sid].dec.Decode(&m)

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
			go asyncHandleServerConsensusPhaseAReply(&m, sid)
		} else {
			log.Errorf("received message is nil")
		}
	}
}

func asyncHandleServerConsensusPhaseAReply(m *WorkerReplyCA, sid ServerId) {

	if m == nil {
		log.Errorf("received WorkerOrderingBReply is empty")
		return
	}
	log.Debugf("%s | (BlockId:%d; ServerId:%d) WorkerOrderingBReply received | time: %s", cmdPhase[CommitPhaseA], m.RangeId, sid, time.Now().UTC().String())

	consensusMetaData.RLock()
	fetchedTotalHash, ok := consensusMetaData.totalHash[m.RangeId]
	partialSig := consensusMetaData.meta[m.RangeId]
	consensusMetaData.RUnlock()

	if !ok {
		log.Debugf("rangId %d is not in consensusMetaData", m.RangeId)
		return
	}

	if !PeakPerf {
		serverId, err := PenVerifyPartially(fetchedTotalHash, m.ParSig)
		if err != nil {
			log.Errorf("%s | PenVerifyPartially server id: %d | err: %v",
				cmdPhase[OrderingPhaseB], serverId, err)
		}
	}

	if len(partialSig) == Threshold {
		log.Debugf(" /!\\ Batch already committed| commitIndicator: %v | Threshold: %v | RangeId: %v",
			len(partialSig), Threshold, m.RangeId)
		return
	}

	partialSig = append(partialSig, m.ParSig)

	consensusMetaData.Lock()
	consensusMetaData.meta[m.RangeId] = partialSig
	consensusMetaData.Unlock()

	if len(partialSig) < Threshold {
		log.Debugf("%s | insufficient votes | blockId: %d | committedIndicator: %d",
			cmdPhase[CommitPhaseA], m.RangeId, len(partialSig))
		return
	}

	if len(partialSig) > Threshold {
		log.Debugf("%s | block %d already broadcast for committing | committedIndicator: %d",
			cmdPhase[CommitPhaseA], m.RangeId, len(partialSig))
		return
	}

	//now incremented logIndicator == quorum
	log.Debugf(" *** votes sufficed *** | rangeId: %v | votes: %d | time: %s", m.RangeId, len(partialSig), time.Now().UTC().String())

	recoveredSig, err := PenRecovery(partialSig, &fetchedTotalHash)
	if err != nil {
		log.Errorf("%s | PenRecovery failed | len(sigShares): %d | error: %v", cmdPhase[CommitPhaseA], len(partialSig), err)
		return
	}

	storeCommitIndex(m.RangeId)

	entryCB := LeaderEntryCB{
		RangeId: m.RangeId,
		ComSig:  recoveredSig,
	}

	broadcast(entryCB, CommitPhaseB)

	log.Infof(">> consensus reached| RangeId %d at %v", m.RangeId*int64(BatchSize), time.Now().UTC().String())

	// Future work:
	// Free cache space for order & commit & consensusMeta maps

	if !slowModeFlag {
		metre.printConsensusLatency(m.RangeId, time.Now().UnixMilli())
	}
}

func handleCommitPhaseBServerConn(sConn *net.Conn) {

	sid, err := registerIncomingWorkerServers(sConn, CommitPhaseB)

	if err != nil {
		log.Errorf("%s | sid: %v | err: %v | incoming conn Addr: %v", sid,
			cmdPhase[CommitPhaseB], err, (*sConn).RemoteAddr())
		return
	}

	// receiveCounter := int64(0)

	//for {
	//	var m WorkerOrderingBReply
	//
	//	err := serverbooth.n[CommitPhaseB][sid].dec.Decode(&m)
	//
	//	counter := atomic.AddInt64(&receiveCounter, 1)
	//
	//	if err == io.EOF{
	//		log.Errorf("%v | server %v closed connection | err: %v", time.Now(), sid, err)
	//		break
	//	}
	//
	//	if err != nil{
	//		log.Errorf("Gob Decode Err: %v | conn with ser: %v | remoteAddr: %v | Now # %v", err, sid, (*sConn).RemoteAddr(), counter)
	//		continue
	//	}
	//
	//	if &m != nil {
	//		go asyncHandleServerOrderReply(&m, sid)
	//	} else {
	//		log.Errorf("received message is nil")
	//	}
	//}
}
