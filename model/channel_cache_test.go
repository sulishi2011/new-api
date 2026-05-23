package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func setupChannelCacheSelectionTest(t *testing.T, channels []int, channelMap map[int]*Channel) {
	t.Helper()

	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroup2model2channels := group2model2channels
	oldChannelsIDM := channelsIDM

	common.MemoryCacheEnabled = true
	group2model2channels = map[string]map[string][]int{
		"default": {
			"gpt-test": channels,
		},
	}
	channelsIDM = channelMap

	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		group2model2channels = oldGroup2model2channels
		channelsIDM = oldChannelsIDM
	})
}

func testChannelForSelection(id int, priority int64) *Channel {
	weight := uint(0)
	return &Channel{
		Id:       id,
		Status:   common.ChannelStatusEnabled,
		Priority: &priority,
		Weight:   &weight,
	}
}

func TestGetRandomSatisfiedChannelSkipsExcludedChannelWhenAlternativeExists(t *testing.T) {
	setupChannelCacheSelectionTest(t, []int{1, 2}, map[int]*Channel{
		1: testChannelForSelection(1, 0),
		2: testChannelForSelection(2, 0),
	})

	selected, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, 1)
	if err != nil {
		t.Fatalf("failed to select channel: %v", err)
	}
	if selected == nil || selected.Id != 2 {
		t.Fatalf("expected channel 2, got %#v", selected)
	}
}

func TestGetRandomSatisfiedChannelFallsBackWhenOnlyExcludedChannelExists(t *testing.T) {
	setupChannelCacheSelectionTest(t, []int{1}, map[int]*Channel{
		1: testChannelForSelection(1, 0),
	})

	selected, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, 1)
	if err != nil {
		t.Fatalf("failed to select channel: %v", err)
	}
	if selected == nil || selected.Id != 1 {
		t.Fatalf("expected channel 1 fallback, got %#v", selected)
	}
}

func TestGetRandomSatisfiedChannelLooksForLowerPriorityBeforeExcludedFallback(t *testing.T) {
	setupChannelCacheSelectionTest(t, []int{1, 2}, map[int]*Channel{
		1: testChannelForSelection(1, 10),
		2: testChannelForSelection(2, 0),
	})

	selected, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, 1)
	if err != nil {
		t.Fatalf("failed to select channel: %v", err)
	}
	if selected == nil || selected.Id != 2 {
		t.Fatalf("expected lower priority channel 2, got %#v", selected)
	}
}
