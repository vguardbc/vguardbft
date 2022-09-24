package main

import (
	"sync"
	"sync/atomic"
)

const (
	PROPOSER = iota
	VALIDATOR
)

var state = struct {
	sync.RWMutex
	logIndex 		int64
	orderIndex		int64
	commitIndex 	int64

	serverId		ServerId
	role			int
}{}

func incrementLogIndex() int64 {
	return atomic.AddInt64(&state.logIndex, 1)
}

func getLogIndex() int64 {
	return atomic.LoadInt64(&state.logIndex)
}

func incrementOrderIndex() int64 {
	return atomic.AddInt64(&state.orderIndex, 1)
}

func getOrderIndex() int64 {
	return atomic.LoadInt64(&state.orderIndex)
}

func storeCommitIndex(cmtIndex int64) {
	atomic.StoreInt64(&state.commitIndex, cmtIndex)
}

func getCommitIndex() int64 {
	return atomic.LoadInt64(&state.commitIndex)
}