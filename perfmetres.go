package main

import (
	"sync"
	"time"
)

// latencyMetre records and measures the latency of V-Guard consensus processes.
// Note that the measurement of performance may become harmful to the overall
// performance; it may decrease throughput and increase latency. In this case,
// latency and throughput evaluations are conduced by taking samples of the
// ordering and consensus instances.
//
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

func (m *latencyMetre) recordStartTime(blockId int64) {
	defer m.startMu.Unlock()
	m.startMu.Lock()
	m.startTime[blockId] = time.Now().UnixMilli()
}

func (m *latencyMetre) recordOrderTime(blockId int64) {
	defer m.orderMu.Unlock()
	m.orderMu.Lock()
	m.orderTime[blockId] = time.Now().UnixMilli()
}

func (m *latencyMetre) printConsensusLatency(lastIdx int) {
	commitTime := time.Now().UnixMilli()
	m.orderMu.Lock()
	n := len(m.orderTime)
	orderingLatency := int64(0)
	consensusLatency := int64(0)

	counter := 0
	skipped := 0
	for i, t := range m.orderTime {
		if i > int64(lastIdx) {
			skipped++
			continue
		}
		counter++
		olat := t - m.startTime[i]
		clat := commitTime - m.startTime[i]

		orderingLatency = orderingLatency + olat
		consensusLatency = consensusLatency + clat

		delete(m.orderTime, i)
		delete(m.startTime, i)
	}
	m.orderMu.Unlock()

	log.Infof("<METRE> Ordering latency (average): %f "+
		"| Consensus latency (average): %f "+
		"| lastIdx: %d | Smaple points: %v | Skipped points: %v",
		float32(orderingLatency)/float32(n),
		float32(consensusLatency)/float32(n),
		lastIdx*BatchSize, counter, skipped)
}
