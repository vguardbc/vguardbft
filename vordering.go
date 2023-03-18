package main

import (
	"encoding/hex"
	"sync"
	"time"
)

var vgrec IDRecorder

type IDRecorder struct {
	sync.RWMutex
	blockIDs []int64
	lastIdx  int
}

func (r *IDRecorder) Add(id int64) {
	r.Lock()
	r.blockIDs = append(r.blockIDs, id)
	r.Unlock()
}

func (r *IDRecorder) GetIDRange() []int64 {
	r.Lock()
	defer r.Unlock()
	curIdx := len(r.blockIDs)
	var blockIDs []int64

	for i := r.lastIdx; i < curIdx; i++ {
		blockIDs = append(blockIDs, r.blockIDs[i])
	}

	r.lastIdx = curIdx
	return blockIDs
}

func (r *IDRecorder) GetLastIdx() int {
	return r.lastIdx
}

//func (r *IDRecorder) GetLastConsPos() int {
//	return r.lastIdx
//}

//func (r *IDRecorder) RecordConsPos()  {
//	defer r.RUnlock()
//	r.RLock()
//	r.lastIdx = len(r.blockIDs)
//}

func startOrderingPhaseA(i int) {

	shuffle := struct {
		sync.RWMutex
		counter int
		entries map[int]Entry
	}{
		counter: 0,
		entries: make(map[int]Entry)}

	cycle := 0

	for {
		cycle++
		m, ok := <-requestQueue[i]

		if !ok {
			log.Infof("requestQueue closed, quiting leader service (server %d)", ServerID)
			return
		}

		entry := Entry{
			TimeStamp: m.Timestamp,
			Tx:        m.Transaction,
		}

		shuffle.entries[shuffle.counter] = entry

		shuffle.counter++
		if shuffle.counter < BatchSize {
			continue
		}

		serializedEntries, err := serialization(shuffle.entries)
		if err != nil {
			log.Errorf("serialization failed, err: %v", err)
			return
		}

		newBlockId := getLogIndex()

		postEntry := ProposerOPAEntry{
			Booth:   booMgr.b[getBoothID()],
			BlockId: newBlockId,
			Entries: shuffle.entries,
			Hash:    getDigest(serializedEntries),
		}

		if PerfMetres {
			if newBlockId%LatMetreInterval == 0 {
				metre.recordStartTime(newBlockId)
			}
		}

		incrementLogIndex()
		orderingBoothID := getBoothID()

		blockOrderFrag := blockSnapshot{
			hash:    postEntry.Hash,
			entries: shuffle.entries,
			sigs:    [][]byte{},
			booth:   booMgr.b[orderingBoothID],
		}

		blockCommitFrag := blockSnapshot{
			hash:    postEntry.Hash,
			entries: shuffle.entries,
			sigs:    [][]byte{},
			booth:   booMgr.b[orderingBoothID],
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

		broadcastToBooth(postEntry, OPA, orderingBoothID)

		if YieldCycle != 0 {
			if cycle%YieldCycle == 0 {
				time.Sleep(time.Duration(EasingDuration) * time.Millisecond)
			}
		}

		log.Debugf("new PostEntryBlock broadcastToBooth -> blk_id: %d | blk_hash: %s",
			postEntry.BlockId, hex.EncodeToString(postEntry.Hash))
	}
}

func asyncHandleOBReply(m *ValidatorOPAReply, sid ServerId) {

	ordSnapshot.RLock()
	blockOrderFrag, ok := ordSnapshot.m[m.BlockId]
	ordSnapshot.RUnlock()

	if !ok {
		log.Debugf("%s | no info of [block:%v] in cache; consensus may already reached | sid: %v", cmdPhase[OPB], m.BlockId, sid)
		return
	}

	currBooth := blockOrderFrag.booth

	blockOrderFrag.Lock()
	indicator := len(blockOrderFrag.sigs)

	if indicator == Threshold {
		blockOrderFrag.Unlock()

		log.Debugf("%s | Block %d already ordered | indicator: %v | Threshold: %v | sid: %v",
			cmdPhase[OPB], m.BlockId, indicator, Threshold, sid)
		return
	}

	indicator++
	aggregatedSigs := append(blockOrderFrag.sigs, m.ParSig)
	blockOrderFrag.sigs = aggregatedSigs

	blockOrderFrag.Unlock()

	if indicator < Threshold {
		log.Debugf("%s | insufficient votes for ordering | blockId: %v | indicator: %v | sid: %v",
			cmdPhase[OPB], m.BlockId, indicator, sid)
		return
	}

	if indicator > Threshold {
		log.Debugf("%s | block %v already broadcastToBooth for ordering | sid: %v", cmdPhase[OPB], m.BlockId, sid)
		return
	}

	thresholdSig, err := PenRecovery(aggregatedSigs, &blockOrderFrag.hash, PublicPoly)
	if err != nil {
		log.Errorf("%s | blockId: %v | PenRecovery failed | len(sigShares): %v | booth: %v| error: %v",
			cmdPhase[OPB], m.BlockId, len(aggregatedSigs), currBooth, err)
		return
	}

	orderEntry := ProposerOPBEntry{
		Booth:   currBooth,
		BlockId: m.BlockId,
		CombSig: thresholdSig,
		//Entries: blockOrderFrag.entries,
		Hash: blockOrderFrag.hash,
	}

	vgrec.Add(m.BlockId)

	if PerfMetres {
		timeNow := time.Now().UTC().String()
		if m.BlockId%LatMetreInterval == 0 {
			metre.recordOrderTime(m.BlockId)
		}

		if m.BlockId%100 == 0 {
			if m.BlockId == 0 {
				log.Infof("<METRE> ordering block %d ordered (tx: %d) at %s", m.BlockId, BatchSize, timeNow)
			} else {
				log.Infof("<METRE> ordering block %d ordered (tx: %d) at %s", m.BlockId, m.BlockId*int64(BatchSize), timeNow)
			}
		}
	}

	cmtSnapshot.Lock()
	cmtSnapshot.m[m.BlockId].tSig = thresholdSig
	cmtSnapshot.Unlock()

	broadcastToBooth(orderEntry, OPB, currBooth.ID)
}
