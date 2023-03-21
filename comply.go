package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"sync"
	"time"
)

var valiConsJobStack = struct {
	sync.RWMutex
	s map[int]chan int // <ConsInstID, chan 1>
}{s: make(map[int]chan int)}

var valiOrdJobStack = struct {
	sync.RWMutex
	s map[int64]chan int // <OrdInstID, chan 1>
}{s: make(map[int64]chan int)}

func validatingOAEntry(m *ProposerOPAEntry, encoder *gob.Encoder) {
	log.Debugf("%s | ProposerOPBEntry received (BlockID: %d) @ %v", rpyPhase[OPA], m.BlockId, time.Now().UTC().String())

	ordSnapshot.Lock()
	if _, ok := ordSnapshot.m[m.BlockId]; ok {
		ordSnapshot.Unlock()
		log.Warnf("%s | blockID %v already used", rpyPhase[OPA], m.BlockId)
		return
	}

	snapshot := blockSnapshot{
		hash:    m.Hash,
		entries: m.Entries,
		sigs:    nil,
		booth:   m.Booth,
	}

	ordSnapshot.m[m.BlockId] = &snapshot
	ordSnapshot.Unlock()

	cmtSnapshot.Lock()
	cmtSnapshot.m[m.BlockId] = &blockSnapshot{
		hash:    m.Hash,
		entries: m.Entries,
		tSig:    nil,
		booth:   m.Booth,
	}

	valiOrdJobStack.Lock()
	if s, ok := valiOrdJobStack.s[m.BlockId]; !ok {
		s := make(chan int, 1)
		valiOrdJobStack.s[m.BlockId] = s
		valiOrdJobStack.Unlock()
		s <- 1
	} else {
		valiOrdJobStack.Unlock()
		s <- 1
	}

	cmtSnapshot.Unlock()

	sig, err := PenSign(m.Hash)
	if err != nil {
		log.Errorf("%s | PenSign failed, err: %v", rpyPhase[OPA], err)
		return
	}

	postReply := ValidatorOPAReply{
		BlockId: m.BlockId,
		ParSig:  sig,
	}

	log.Debugf("%s | msg: %v; ps: %v", rpyPhase[OPA], m.BlockId, hex.EncodeToString(sig))

	dialSendBack(postReply, encoder, OPA)
}

func validatingOBEntry(m *ProposerOPBEntry, encoder *gob.Encoder) {
	if encoder != nil {
		log.Debugf("%s | ProposerOPBEntry received (BlockID: %d) @ %v", rpyPhase[OPB], m.BlockId, time.Now().UTC().String())
	} else {
		log.Debugf("%s | Sync up -> ProposerOPBEntry received (BlockID: %d) @ %v", rpyPhase[CPA], m.BlockId, time.Now().UTC().String())
	}

	err := PenVerify(m.Hash, m.CombSig, PublicPoly)
	if err != nil {
		log.Errorf("%v: PenVerify failed | err: %v | BlockID: %v | m.Hash: %v| CombSig: %v",
			rpyPhase[OPB], err, m.BlockId, hex.EncodeToString(m.Hash), hex.EncodeToString(m.CombSig))
		return
	}

	ordSnapshot.RLock()
	_, ok := ordSnapshot.m[m.BlockId]
	ordSnapshot.RUnlock()

	if !ok {
		// It is common that some validators have not seen this message.
		// Consensus requires only 2f+1 servers, in which f of them may
		// not receive the message in the previous phase.
		log.Debugf("%v : block %v not stored in ordSnapshot (size of %v)", rpyPhase[OPB], m.BlockId, len(ordSnapshot.m))

		if encoder == nil {
			cmtSnapshot.Lock()
			cmtSnapshot.m[m.BlockId] = &blockSnapshot{
				hash:    m.Hash,
				entries: m.Entries,
				tSig:    m.CombSig,
				booth:   m.Booth,
			}
			cmtSnapshot.Unlock()
		}
		return
	} else {
		log.Debugf("%s | ordSnapshot fetched for BlockId: %d", rpyPhase[OPB], m.BlockId)
	}

	cmtSnapshot.Lock()
	if _, ok := cmtSnapshot.m[m.BlockId]; !ok {

		valiOrdJobStack.Lock()
		if s, ok := valiOrdJobStack.s[m.BlockId]; !ok {
			s := make(chan int, 1)
			valiOrdJobStack.s[m.BlockId] = s
			valiOrdJobStack.Unlock()
			<-s
		} else {
			valiOrdJobStack.Unlock()
			<-s
		}
	}
	cmtSnapshot.m[m.BlockId].tSig = m.CombSig
	cmtSnapshot.Unlock()

	log.Debugf("block %d ordered", m.BlockId)
}

func validatingCAEntry(m *ProposerCPAEntry, encoder *gob.Encoder) {

	log.Debugf("%s | ProposerCPAEntry received (RangeId: %d) @ %v", rpyPhase[CPA], m.ConsInstID, time.Now().UTC().String())

	vgTxData.Lock()
	vgTxData.tx[m.ConsInstID] = make(map[string][][]Entry)
	vgTxData.Unlock()

	if m.PrevOPBEntries != nil {
		log.Debugf("%s | %v| len(PrevOPBEntries): %v", rpyPhase[CPA], m.ConsInstID, len(m.PrevOPBEntries))

		for _, OPBEntry := range m.PrevOPBEntries {
			validatingOBEntry(&OPBEntry, nil)
		}
	}

	if m.BIDs == nil {
		log.Errorf("%s | ConsInstID: %v | Empty BIDs shouldn't have been transmitted", rpyPhase[CPA], m.ConsInstID)
		return
	}

	for _, blockID := range m.BIDs {
		cmtSnapshot.RLock()
		snapshot, ok := cmtSnapshot.m[blockID]
		cmtSnapshot.RUnlock()

		if !ok {
			log.Infof("%v | cmtSnapshot.h[%v] not found in cache|", rpyPhase[CPA], blockID)
			continue
		}

		if !bytes.Equal(snapshot.hash, m.RangeHash[blockID]) {
			log.Errorf(" block hashes don't match; ConsInstID:%d mapsize:%d; received m.RangeHash[%v]: %v | local: %v",
				m.ConsInstID, len(m.RangeHash), blockID, m.RangeHash[blockID], snapshot.hash)
			return
		}

		var blockEntries []Entry

		for _, entry := range snapshot.entries {
			blockEntries = append(blockEntries, entry)
		}

		boo, err := snapshot.booth.String()
		if err != nil {
			log.Error(err)
			return
		}

		vgTxData.Lock()
		if _, ok := vgTxData.tx[m.ConsInstID][boo]; ok {
			vgTxData.tx[m.ConsInstID][boo] = append(vgTxData.tx[m.ConsInstID][boo], blockEntries)
		} else {
			vgTxData.tx[m.ConsInstID][boo] = [][]Entry{blockEntries}
		}

		vgTxData.boo[m.ConsInstID] = m.Booth
		vgTxData.Unlock()
	}

	partialSig, err := PenSign(m.TotalHash)
	if err != nil {
		log.Errorf("PenSign failed: %v", err)
		return
	}

	replyCA := ValidatorCPAReply{
		ConsInstID: m.ConsInstID,
		ParSig:     partialSig,
	}

	dialSendBack(replyCA, encoder, CPA)

	vgTxMeta.Lock()
	vgTxMeta.hash[m.ConsInstID] = m.TotalHash
	vgTxMeta.Unlock()

	valiConsJobStack.Lock()
	if s, ok := valiConsJobStack.s[m.ConsInstID]; !ok {
		s := make(chan int, 1)
		valiConsJobStack.s[m.ConsInstID] = s
		valiConsJobStack.Unlock()
		s <- 1
	} else {
		valiConsJobStack.Unlock()
		s <- 1
	}
}

func validatingCBEntry(m *ProposerCPBEntry, encoder *gob.Encoder) {

	vgTxMeta.RLock()
	_, ok := vgTxMeta.hash[m.ConsInstID]
	vgTxMeta.RUnlock()

	if !ok {
		log.Debugf("%v | vgTxMeta.hash[m.RangeId:%v] not stored in cache|", rpyPhase[CPB], m.ConsInstID)
	}

	// Wait for prior job to be finished first
	valiConsJobStack.Lock()
	if s, ok := valiConsJobStack.s[m.ConsInstID]; !ok {
		s := make(chan int, 1)
		valiConsJobStack.s[m.ConsInstID] = s
		valiConsJobStack.Unlock()
		<-s
	} else {
		valiConsJobStack.Unlock()
		<-s
	}

	err := PenVerify(m.Hash, m.ComSig, PublicPoly)
	if err != nil {
		log.Errorf("%v | PenVerify failed; err: %v", rpyPhase[CPB], err)
		return
	}

	storeVgTx(m.ConsInstID)

	//replyCB := ValidatorCPBReply{
	//	RangeId: m.RangeId,
	//	Done:    true,
	//}
	//
	//dialSendBack(replyCB, encoder, CPB)
}
