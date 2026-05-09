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

const usageAggregationTickInterval = time.Minute

var (
	usageAggregationOnce    sync.Once
	usageAggregationRunning atomic.Bool
	usageAggregationLastRun atomic.Int64
)

func StartUsageAggregationTask() {
	usageAggregationOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("usage aggregation task started: tick=%s", usageAggregationTickInterval))
			ticker := time.NewTicker(usageAggregationTickInterval)
			defer ticker.Stop()

			runUsageAggregationOnce()
			for range ticker.C {
				runUsageAggregationOnce()
			}
		})
	})
}

func runUsageAggregationOnce() {
	if !common.UsageAggregationEnabled {
		return
	}
	if !usageAggregationRunning.CompareAndSwap(false, true) {
		return
	}
	defer usageAggregationRunning.Store(false)

	now := time.Now().UTC()
	scheduleMinute := common.UsageAggregationScheduleMinute
	if scheduleMinute < 0 {
		scheduleMinute = 0
	}
	if scheduleMinute > 59 {
		scheduleMinute = 59
	}
	if now.Minute() < scheduleMinute {
		return
	}

	targetHour := now.Truncate(time.Hour).Add(-time.Hour).Unix()
	if usageAggregationLastRun.Load() == targetHour {
		return
	}

	recomputeHours := common.UsageAggregationRecomputeHours
	if recomputeHours <= 0 {
		recomputeHours = 1
	}
	rollupsSucceeded := true
	for i := 0; i < recomputeHours; i++ {
		bucket := targetHour - int64(i)*3600
		if !runUsageAggregationJob("hourly", bucket, func() error {
			return model.RollupUsageAggregateHourly(bucket)
		}) {
			rollupsSucceeded = false
		}
	}

	recomputeDays := common.UsageAggregationRecomputeDays
	if recomputeDays <= 0 {
		recomputeDays = 1
	}
	currentDay := model.UsageAggregateDayBucket(targetHour)
	for i := 0; i < recomputeDays; i++ {
		dayBucket := currentDay - int64(i)*86400
		if !runUsageAggregationJob("daily", dayBucket, func() error {
			return model.RollupUsageAggregateDaily(dayBucket)
		}) {
			rollupsSucceeded = false
		}
	}

	cleanupUsageAggregates(now)
	if rollupsSucceeded {
		usageAggregationLastRun.Store(targetHour)
	}
}

func runUsageAggregationJob(jobType string, bucketStart int64, fn func() error) bool {
	startedAt := common.GetTimestamp()
	err := fn()
	finishedAt := common.GetTimestamp()
	status := "success"
	if err != nil {
		status = "failed"
		logger.LogWarn(context.Background(), fmt.Sprintf("usage aggregation %s bucket=%d failed: %v", jobType, bucketStart, err))
	}
	model.RecordUsageAggregationJob(jobType, bucketStart, status, startedAt, finishedAt, err)
	return err == nil
}

func cleanupUsageAggregates(now time.Time) {
	hourlyRetentionDays := common.UsageAggregationHourlyRetentionDays
	if hourlyRetentionDays > 0 {
		cutoff := now.Add(-time.Duration(hourlyRetentionDays) * 24 * time.Hour).Truncate(time.Hour).Unix()
		batchHours := common.UsageAggregationDeleteBatchHours
		runUsageAggregationJob("cleanup_hourly", cutoff, func() error {
			_, err := model.CleanupUsageAggregateHourly(cutoff, batchHours)
			return err
		})
	}

	dailyRetentionDays := common.UsageAggregationDailyRetentionDays
	if dailyRetentionDays > 0 {
		cutoff := model.UsageAggregateDayBucket(now.Add(-time.Duration(dailyRetentionDays) * 24 * time.Hour).Unix())
		runUsageAggregationJob("cleanup_daily", cutoff, func() error {
			_, err := model.CleanupUsageAggregateDaily(cutoff, common.UsageAggregationDeleteBatchHours)
			return err
		})
	}
}
