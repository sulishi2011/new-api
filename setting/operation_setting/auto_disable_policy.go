package operation_setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type AutomaticDisablePolicyGroup struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	Keywords []string `json:"keywords"`
}

var AutomaticDisablePolicyGroups = []AutomaticDisablePolicyGroup{}

func NormalizeAutomaticDisableKeywords(keywords []string) []string {
	normalized := make([]string, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(strings.ToLower(keyword))
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		normalized = append(normalized, keyword)
	}
	return normalized
}

func AutomaticDisablePolicyGroupsToString() string {
	if len(AutomaticDisablePolicyGroups) == 0 {
		return "[]"
	}
	jsonBytes, err := common.Marshal(AutomaticDisablePolicyGroups)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func AutomaticDisablePolicyGroupsFromString(s string) error {
	groups, err := ParseAutomaticDisablePolicyGroups(s)
	if err != nil {
		return err
	}
	AutomaticDisablePolicyGroups = groups
	return nil
}

func ParseAutomaticDisablePolicyGroups(s string) ([]AutomaticDisablePolicyGroup, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []AutomaticDisablePolicyGroup{}, nil
	}

	groups := make([]AutomaticDisablePolicyGroup, 0)
	if err := common.Unmarshal([]byte(s), &groups); err != nil {
		return nil, err
	}

	normalized := make([]AutomaticDisablePolicyGroup, 0, len(groups))
	ids := make(map[string]struct{}, len(groups))
	names := make(map[string]struct{}, len(groups))
	for index, group := range groups {
		group.ID = strings.TrimSpace(group.ID)
		group.Name = strings.TrimSpace(group.Name)
		if group.ID == "" {
			return nil, fmt.Errorf("auto-disable policy group #%d id is required", index+1)
		}
		if group.Name == "" {
			return nil, fmt.Errorf("auto-disable policy group #%d name is required", index+1)
		}
		if _, ok := ids[group.ID]; ok {
			return nil, fmt.Errorf("duplicate auto-disable policy group id: %s", group.ID)
		}
		nameKey := strings.ToLower(group.Name)
		if _, ok := names[nameKey]; ok {
			return nil, fmt.Errorf("duplicate auto-disable policy group name: %s", group.Name)
		}
		ids[group.ID] = struct{}{}
		names[nameKey] = struct{}{}
		group.Keywords = NormalizeAutomaticDisableKeywords(group.Keywords)
		normalized = append(normalized, group)
	}
	return normalized, nil
}

func GetAutomaticDisablePolicyGroups() []AutomaticDisablePolicyGroup {
	groups := make([]AutomaticDisablePolicyGroup, len(AutomaticDisablePolicyGroups))
	copy(groups, AutomaticDisablePolicyGroups)
	return groups
}

func GetAutomaticDisablePolicyKeywords(policyGroupID string) []string {
	policyGroupID = strings.TrimSpace(policyGroupID)
	if policyGroupID == "" {
		return AutomaticDisableKeywords
	}
	for _, group := range AutomaticDisablePolicyGroups {
		if group.ID != policyGroupID {
			continue
		}
		if !group.Enabled {
			return AutomaticDisableKeywords
		}
		return group.Keywords
	}
	return AutomaticDisableKeywords
}
