package snowflake

import (
	"sync"
	"time"
)

// Simple snowflake: 41-bit timestamp + 10-bit machine + 12-bit sequence
const (
	epoch     = int64(1700000000000) // custom epoch ms
	machineBits = 10
	seqBits     = 12
	maxSeq      = -1 ^ (-1 << seqBits)
)

var (
	mu        sync.Mutex
	lastMS    int64
	sequence  int64
	machineID int64 = 1
)

func Next() int64 {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now().UnixMilli() - epoch
	if now == lastMS {
		sequence = (sequence + 1) & maxSeq
		if sequence == 0 {
			for now <= lastMS {
				now = time.Now().UnixMilli() - epoch
			}
		}
	} else {
		sequence = 0
	}
	lastMS = now
	return (now << (machineBits + seqBits)) | (machineID << seqBits) | sequence
}
