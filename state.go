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
	booMu       sync.RWMutex
	logIndex    int64
	orderIndex  int64
	commitIndex int64

	serverId ServerId
	role     int
	boothID  int
}{}

//var boothID = struct {
//	sync.RWMutex
//	i int
//}{}

func nextBoothID() int {
	defer state.booMu.Unlock()
	state.booMu.Lock()
	state.boothID++
	return state.boothID
}

func getBoothID() int {
	defer state.booMu.RUnlock()
	state.booMu.RLock()
	return state.boothID
}

func incrementLogIndex() int64 {
	return atomic.AddInt64(&state.logIndex, 1)
}

func getLogIndex() int64 {
	return atomic.LoadInt64(&state.logIndex)
}

//
//func incrementOrderIndex() int64 {
//	return atomic.AddInt64(&state.orderIndex, 1)
//}

//func getOrderIndex() int64 {
//	return atomic.LoadInt64(&state.orderIndex)
//}

//func setCommitIndex(cmtIndex int64) {
//	atomic.StoreInt64(&state.commitIndex, cmtIndex)
//}
//
//func getCommitIndex() int64 {
//	return atomic.LoadInt64(&state.commitIndex)
//}
