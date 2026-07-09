package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

var (
	logRetentionOnce    sync.Once
	logRetentionRunning atomic.Bool
)

type logRetentionPolicy struct {
	logType int
	name    string
	days    int
}

func StartLogRetentionTask() {
	logRetentionOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			interval := time.Duration(common.LogCleanupIntervalHours) * time.Hour
			if interval < time.Hour {
				interval = 24 * time.Hour
			}
			logger.LogInfo(context.Background(), fmt.Sprintf("log retention task started: interval=%s", interval))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			runLogRetentionOnce()
			for range ticker.C {
				runLogRetentionOnce()
			}
		})
	})
}

func runLogRetentionOnce() {
	if !common.TraceStorageEnabled {
		return
	}
	if !logRetentionRunning.CompareAndSwap(false, true) {
		return
	}
	defer logRetentionRunning.Store(false)

	ctx := context.Background()
	now := time.Now().UTC()
	cleanupExpiredTraceMetadata(ctx, now)
}

// logCleanupBatchTimeout caps a single cleanup query so a slow batch cannot
// hold the database for minutes; the remainder is picked up by the next run.
const logCleanupBatchTimeout = 30 * time.Second

func cleanupExpiredTraceMetadata(ctx context.Context, now time.Time) {
	batchSize := common.LogCleanupBatchSize
	deadline := time.Now().Add(time.Duration(common.LogCleanupRunMaxSeconds) * time.Second)
	var total int64
	for {
		batchCtx, cancel := context.WithTimeout(ctx, logCleanupBatchTimeout)
		count, err := model.CleanupExpiredLogTraces(batchCtx, now.Unix(), batchSize)
		cancel()
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("cleanup expired log traces failed: %v", err))
			return
		}
		total += count
		if count < int64(batchSize) {
			break
		}
		if time.Now().After(deadline) {
			logger.LogInfo(ctx, fmt.Sprintf("log trace cleanup run budget exhausted, deleted=%d so far, remainder deferred to next run", total))
			return
		}
		sleepLogCleanupBatch()
	}
	if total > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("deleted expired log traces: count=%d", total))
	}
}

func cleanupLogsByRetentionPolicy(ctx context.Context, now time.Time) {
	policies := []logRetentionPolicy{
		{logType: model.LogTypeConsume, name: "consume", days: common.LogRetentionConsumeDays},
		{logType: model.LogTypeError, name: "error", days: common.LogRetentionErrorDays},
		{logType: model.LogTypeSystem, name: "system", days: common.LogRetentionSystemDays},
		{logType: model.LogTypeManage, name: "manage", days: common.LogRetentionManageDays},
		{logType: model.LogTypeTopup, name: "topup", days: common.LogRetentionTopupDays},
		{logType: model.LogTypeRefund, name: "refund", days: common.LogRetentionRefundDays},
	}
	for _, policy := range policies {
		if policy.days <= 0 {
			continue
		}
		cutoff := now.Add(-time.Duration(policy.days) * 24 * time.Hour).Unix()
		total := cleanupLogType(ctx, policy, cutoff)
		if total > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("deleted old %s logs: count=%d, cutoff=%d", policy.name, total, cutoff))
		}
	}
}

func cleanupLogType(ctx context.Context, policy logRetentionPolicy, cutoff int64) int64 {
	batchSize := common.LogCleanupBatchSize
	var total int64
	for {
		count, err := model.DeleteLogsByTypeBefore(ctx, policy.logType, cutoff, batchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("cleanup %s logs failed: %v", policy.name, err))
			return total
		}
		total += count
		if count < int64(batchSize) {
			return total
		}
		sleepLogCleanupBatch()
	}
}

func sleepLogCleanupBatch() {
	if common.LogCleanupBatchSleepMS <= 0 {
		return
	}
	time.Sleep(time.Duration(common.LogCleanupBatchSleepMS) * time.Millisecond)
}
