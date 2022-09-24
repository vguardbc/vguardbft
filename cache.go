package main

import (
	"sync"
)

var ordSnapshot = struct {
	//m map[blockId]
	m map[int64]*blockSnapshot
	sync.RWMutex
}{m: make(map[int64]*blockSnapshot)}

var cmtSnapshot = struct {
	m map[int64]*blockSnapshot
	sync.RWMutex
}{m: make(map[int64]*blockSnapshot)}

type blockSnapshot struct {
	sync.RWMutex
	hashOfEntriesInBlock []byte
	//Collect postEntry replies and send orderEntry
	//cryptSigOfLeaderEntry	*[]byte //Use cryptSigOfLeaderPostEntry as a base in reply

	entriesInBlock map[int]Entry

	//The coordinator of the order phase accumulates threshold
	//signatures from others, append it to orderedThreshSig,
	//which is going to be converted to one threshold signature
	//by function PenRecovery.
	concatThreshSig [][]byte

	//counter increments when receiving a postReply.
	//When 2f+1 identical postReplies received, 2f+1 servers have
	//agreed on the posted order.
	counter int
}

var consensusMetaData = struct {
	sync.RWMutex
	meta          map[int64][][]byte // <rangeId, concatThreshSig[]>
	totalHash     map[int64][]byte
	rangeRecorder map[int64]int64 // <rangeId, startBID>
}{
	meta:          make(map[int64][][]byte),
	totalHash:     make(map[int64][]byte),
	rangeRecorder: make(map[int64]int64),
}

var consensusData = struct {
	sync.RWMutex
	buf map[int64][]Entry // <rangeId, >
}{
	buf: make(map[int64][]Entry),
}
