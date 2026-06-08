package service

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/setting"
)

func TestAutoGroupAccessUsesAutoGroupTargets(t *testing.T) {
	t.Cleanup(func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(`{"default":"default group","vip":"vip group"}`)
		_ = setting.UpdateAutoGroupsByJsonString(`["default"]`)
	})

	if err := setting.UpdateUserUsableGroupsByJSONString(`{"default":"default group"}`); err != nil {
		t.Fatalf("update user usable groups: %v", err)
	}
	if err := setting.UpdateAutoGroupsByJsonString(`["default","vip"]`); err != nil {
		t.Fatalf("update auto groups: %v", err)
	}

	if !GroupInUserUsableGroups("default", "auto") {
		t.Fatalf("auto group should be usable when an auto target is usable")
	}

	got := GetUserAutoGroup("default")
	want := []string{"default"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetUserAutoGroup() = %v, want %v", got, want)
	}
}

func TestAutoGroupAccessRequiresUsableAutoTarget(t *testing.T) {
	t.Cleanup(func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(`{"default":"default group","vip":"vip group"}`)
		_ = setting.UpdateAutoGroupsByJsonString(`["default"]`)
	})

	if err := setting.UpdateUserUsableGroupsByJSONString(`{"vip":"vip group"}`); err != nil {
		t.Fatalf("update user usable groups: %v", err)
	}
	if err := setting.UpdateAutoGroupsByJsonString(`["default"]`); err != nil {
		t.Fatalf("update auto groups: %v", err)
	}

	if GroupInUserUsableGroups("guest", "auto") {
		t.Fatalf("auto group should not be usable without a usable auto target")
	}
}
