package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAutomaticDisablePolicyGroupsNormalizesKeywords(t *testing.T) {
	groups, err := ParseAutomaticDisablePolicyGroups(`[
		{"id":" paid ","name":" Paid account ","enabled":true,"keywords":[" Quota ","quota","","Permission denied"]},
		{"id":"auth","name":"Auth errors","enabled":false,"keywords":["invalid token"]}
	]`)

	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, "paid", groups[0].ID)
	require.Equal(t, "Paid account", groups[0].Name)
	require.True(t, groups[0].Enabled)
	require.Equal(t, []string{"quota", "permission denied"}, groups[0].Keywords)
	require.False(t, groups[1].Enabled)
}

func TestParseAutomaticDisablePolicyGroupsRejectsDuplicates(t *testing.T) {
	_, err := ParseAutomaticDisablePolicyGroups(`[
		{"id":"paid","name":"Paid","enabled":true,"keywords":[]},
		{"id":"paid","name":"Other","enabled":true,"keywords":[]}
	]`)

	require.Error(t, err)

	_, err = ParseAutomaticDisablePolicyGroups(`[
		{"id":"paid","name":"Paid","enabled":true,"keywords":[]},
		{"id":"auth","name":"paid","enabled":true,"keywords":[]}
	]`)

	require.Error(t, err)
}

func TestParseAutomaticDisablePolicyGroupsEmptyInput(t *testing.T) {
	originalGroups := AutomaticDisablePolicyGroups
	t.Cleanup(func() {
		AutomaticDisablePolicyGroups = originalGroups
	})
	AutomaticDisablePolicyGroups = nil

	groups, err := ParseAutomaticDisablePolicyGroups("  ")

	require.NoError(t, err)
	require.Empty(t, groups)
	require.Equal(t, "[]", AutomaticDisablePolicyGroupsToString())
}

func TestGetAutomaticDisablePolicyKeywordsFallbacks(t *testing.T) {
	originalKeywords := AutomaticDisableKeywords
	originalGroups := AutomaticDisablePolicyGroups
	t.Cleanup(func() {
		AutomaticDisableKeywords = originalKeywords
		AutomaticDisablePolicyGroups = originalGroups
	})

	AutomaticDisableKeywords = []string{"global"}
	AutomaticDisablePolicyGroups = []AutomaticDisablePolicyGroup{
		{ID: "dedicated", Name: "Dedicated", Enabled: true, Keywords: []string{"dedicated"}},
		{ID: "empty", Name: "Empty", Enabled: true, Keywords: []string{}},
		{ID: "disabled", Name: "Disabled", Enabled: false, Keywords: []string{"disabled"}},
	}

	require.Equal(t, []string{"global"}, GetAutomaticDisablePolicyKeywords(""))
	require.Equal(t, []string{"dedicated"}, GetAutomaticDisablePolicyKeywords("dedicated"))
	require.Equal(t, []string{}, GetAutomaticDisablePolicyKeywords("empty"))
	require.Equal(t, []string{"global"}, GetAutomaticDisablePolicyKeywords("disabled"))
	require.Equal(t, []string{"global"}, GetAutomaticDisablePolicyKeywords("missing"))
}
