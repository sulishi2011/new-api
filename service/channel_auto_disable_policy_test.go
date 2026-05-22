package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestShouldDisableChannelUsesDedicatedPolicyKeywords(t *testing.T) {
	originalEnabled := common.AutomaticDisableChannelEnabled
	originalKeywords := operation_setting.AutomaticDisableKeywords
	originalGroups := operation_setting.AutomaticDisablePolicyGroups
	originalStatusCodes := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = originalEnabled
		operation_setting.AutomaticDisableKeywords = originalKeywords
		operation_setting.AutomaticDisablePolicyGroups = originalGroups
		operation_setting.AutomaticDisableStatusCodeRanges = originalStatusCodes
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableKeywords = []string{"global quota"}
	operation_setting.AutomaticDisableStatusCodeRanges = nil
	operation_setting.AutomaticDisablePolicyGroups = []operation_setting.AutomaticDisablePolicyGroup{
		{ID: "paid", Name: "Paid", Enabled: true, Keywords: []string{"paid quota"}},
		{ID: "disabled", Name: "Disabled", Enabled: false, Keywords: []string{"disabled quota"}},
	}

	err := types.NewOpenAIError(errors.New("paid quota exceeded"), types.ErrorCode("upstream_error"), http.StatusBadRequest)
	require.True(t, ShouldDisableChannel(err, "paid"))
	require.False(t, ShouldDisableChannel(err))

	err = types.NewOpenAIError(errors.New("global quota exceeded"), types.ErrorCode("upstream_error"), http.StatusBadRequest)
	require.True(t, ShouldDisableChannel(err))
	require.True(t, ShouldDisableChannel(err, "missing"))
	require.True(t, ShouldDisableChannel(err, "disabled"))
	require.False(t, ShouldDisableChannel(err, "paid"))
}
