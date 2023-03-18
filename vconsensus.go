package main

import (
	"time"
)

func startConsensusPhaseA() {
	consInstID := 0
	waited := 0

	for {
		commitBoothID := getBoothID()

		switch BoothMode {
		case BoothModeOCSB:

		case BoothModeOCDBWOP:
			commitBoothID = BoothIDOfModeOCDBWOP
		case BoothModeOCDBNOP:
			commitBoothID = BoothIDOfModeOCDBNOP
		}

		commitBooth := booMgr.b[commitBoothID]
		time.Sleep(time.Duration(ConsInterval) * time.Millisecond)

		blockIDRange := vgrec.GetIDRange()

		if blockIDRange == nil {
			log.Warnf("Waiting for new ordered blocks [consInstID:%v | LastOrdIdx: %v]; consensus has waited for %v second(s)",
				consInstID, vgrec.GetLastIdx(), waited)

			time.Sleep(1 * time.Second)

			waited++
			if waited < ConsWaitingSec {
				continue
			}

			log.Warnf("Terminating current VGuard instance")
			vgInst.Done()
			return
		}

		blockHashesInRange := make(map[int64][]byte)

		vgTxData.Lock()
		vgTxData.tx[consInstID] = make(map[string][][]Entry)
		vgTxData.Unlock()

		var newMembers []int
		var resentOPBEntries []ProposerOPBEntry

		for _, blockID := range blockIDRange {
			var blockEntries []Entry

			cmtSnapshot.Lock()
			blockCmtFrag, ok := cmtSnapshot.m[blockID]
			cmtSnapshot.Unlock()

			if !ok {
				log.Errorf("cmtSnapshot.h[%d] not exists; ; continue", blockID)
				continue
			}

			blockHashesInRange[blockID] = blockCmtFrag.hash

			for _, entry := range blockCmtFrag.entries {
				blockEntries = append(blockEntries, entry)
			}

			ordBoo := blockCmtFrag.booth

			newMemberFlag := false

			for _, cmtMember := range commitBooth.Indices {
				if !BooIndices(ordBoo.Indices).Contain(cmtMember) {
					newMemberFlag = true
					if !BooIndices(newMembers).Contain(cmtMember) {
						newMembers = append(newMembers, cmtMember)
					}
					log.Debugf("%s | %v is the new member in CMT-BOO: %v | ORD-BOO: %v | BlockID: %v",
						cmdPhase[CPA], cmtMember, commitBooth.Indices, ordBoo.Indices, blockID)
				}
			}

			if newMemberFlag {
				resentOPBEntries = append(resentOPBEntries, ProposerOPBEntry{
					Booth:   ordBoo,
					BlockId: blockID,
					CombSig: blockCmtFrag.tSig,
					Entries: blockCmtFrag.entries,
					Hash:    blockCmtFrag.hash,
				})
				if blockCmtFrag.tSig == nil {
					log.Errorf("CombSig is nil ! len of Entries: %v", len(blockCmtFrag.entries))
				}
			}

			boo, err := ordBoo.String()
			if err != nil {
				log.Error(err)
				return
			}

			vgTxData.Lock()
			if _, ok := vgTxData.tx[consInstID][boo]; ok {
				vgTxData.tx[consInstID][boo] = append(vgTxData.tx[consInstID][boo], blockEntries)
			} else {
				vgTxData.tx[consInstID][boo] = [][]Entry{blockEntries}
			}

			vgTxData.boo[consInstID] = commitBooth
			vgTxData.Unlock()
		}

		serialized, err := serialization(blockHashesInRange)

		if err != nil {
			log.Error(err)
			break
		}

		totalHash := getDigest(serialized)

		entryCA := ProposerCPAEntry{
			PrevOPBEntries: nil,
			Booth:          commitBooth,
			BIDs:           blockIDRange,
			ConsInstID:     consInstID,
			RangeHash:      blockHashesInRange,
			TotalHash:      totalHash,
		}

		if newMembers == nil {
			broadcastToBooth(entryCA, CPA, commitBoothID)
		} else {
			newEntryCA := entryCA
			newEntryCA.SetPrevOPBEntries(resentOPBEntries)
			log.Debugf("%s | sending newEntry CA to %v | len(resentOPBEntries): %v", cmdPhase[CPA], newMembers, len(resentOPBEntries))
			broadcastToNewBooth(entryCA, CPA, commitBoothID, newMembers, newEntryCA)
		}

		vgTxMeta.Lock()
		vgTxMeta.hash[consInstID] = totalHash
		vgTxMeta.blockIDs[consInstID] = blockIDRange
		vgTxMeta.Unlock()

		consInstID++
	}
}

func asyncHandleCPAReply(m *ValidatorCPAReply, sid ServerId) {

	vgTxMeta.RLock()
	fetchedTotalHash, ok := vgTxMeta.hash[m.ConsInstID]
	partialSig := vgTxMeta.sigs[m.ConsInstID]
	vgTxMeta.RUnlock()

	if !ok {
		log.Debugf("%s | rangId %d is not in vgTxMeta", cmdPhase[CPA], m.ConsInstID)
		return
	}

	vgTxData.RLock()
	residingBooth := vgTxData.boo[m.ConsInstID]
	vgTxData.RUnlock()

	if len(partialSig) == Threshold {
		log.Debugf("%s | Batch already committed| commitIndicator: %v | Threshold: %v | RangeId: %v | sid: %v",
			cmdPhase[CPA], len(partialSig), Threshold, m.ConsInstID, sid)
		return
	}

	partialSig = append(partialSig, m.ParSig)

	vgTxMeta.Lock()
	vgTxMeta.sigs[m.ConsInstID] = partialSig
	vgTxMeta.Unlock()

	if len(partialSig) < Threshold {
		log.Debugf("%s | insufficient votes | blockId: %d | indicator: %d | sid: %v", cmdPhase[CPA], m.ConsInstID, len(partialSig), sid)
		return
	} else if len(partialSig) > Threshold {
		log.Debugf("%s | block %d already broadcastToBooth | indicator: %d | sid: %v", cmdPhase[CPA], m.ConsInstID, len(partialSig), sid)
		return
	}

	log.Debugf(" ** votes sufficient | rangeId: %v | votes: %d | sid: %v", m.ConsInstID, len(partialSig), sid)

	recoveredSig, err := PenRecovery(partialSig, &fetchedTotalHash, PublicPoly)
	if err != nil {
		log.Errorf("%s | PenRecovery failed | len(sigShares): %d | error: %v", cmdPhase[CPA], len(partialSig), err)
		return
	}

	entryCB := ProposerCPBEntry{
		Booth:      residingBooth,
		ConsInstID: m.ConsInstID,
		ComSig:     recoveredSig,
		Hash:       fetchedTotalHash,
	}

	broadcastToBooth(entryCB, CPB, residingBooth.ID)

	if !PerfMetres {
		storeVgTx(m.ConsInstID)
	}

	// Future work: garbage collection

	if PerfMetres {
		metre.printConsensusLatency(vgrec.lastIdx)
	}
}
