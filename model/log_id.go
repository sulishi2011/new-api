package model

import (
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	logIDTimeFactor  = int64(1000000)
	logIDNodeFactor  = int64(10000)
	logIDNodeSlots   = int64(100)
	logIDSequenceMax = int64(10000)
)

var (
	logIDMu         sync.Mutex
	logIDLastSecond int64
	logIDSequence   int64
	logIDNodeSlot   = resolveLogIDNodeSlot()
)

func resolveLogIDNodeSlot() int64 {
	if raw := os.Getenv("LOG_ID_NODE_ID"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && value >= 0 && value < logIDNodeSlots {
			return value
		}
		common.SysLog(fmt.Sprintf("invalid LOG_ID_NODE_ID %q, fallback to hashed node slot", raw))
	}

	seed := common.NodeName
	if seed == "" {
		seed = os.Getenv("NODE_NAME")
	}
	if seed == "" {
		if hostname, err := os.Hostname(); err == nil {
			seed = hostname
		}
	}
	if seed == "" {
		seed = common.GetIp()
	}
	if seed == "" {
		seed = common.GetUUID()
	}

	hash := fnv.New32a()
	_, _ = hash.Write([]byte(seed))
	return int64(hash.Sum32()) % logIDNodeSlots
}

func assignLogID(log *Log) {
	if log == nil || log.Id != 0 {
		return
	}
	if log.CreatedAt <= 0 {
		log.CreatedAt = common.GetTimestamp()
	}
	log.Id = nextLogID(&log.CreatedAt)
}

func nextLogID(createdAt *int64) int64 {
	logIDMu.Lock()
	defer logIDMu.Unlock()

	second := *createdAt
	if second <= 0 {
		second = common.GetTimestamp()
	}
	if second < logIDLastSecond {
		second = logIDLastSecond
	}
	if second == logIDLastSecond {
		logIDSequence++
		if logIDSequence >= logIDSequenceMax {
			for second <= logIDLastSecond {
				time.Sleep(time.Millisecond)
				second = common.GetTimestamp()
			}
			logIDSequence = 0
		}
	} else {
		logIDLastSecond = second
		logIDSequence = 0
	}
	if second != *createdAt {
		*createdAt = second
	}
	logIDLastSecond = second
	return second*logIDTimeFactor + logIDNodeSlot*logIDNodeFactor + logIDSequence
}

func timestampFromGeneratedLogID(logID int64) (int64, bool) {
	second := logID / logIDTimeFactor
	now := common.GetTimestamp()
	if second < 1577836800 || second > now+366*86400 {
		return 0, false
	}
	return second, true
}
