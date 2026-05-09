package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupChannelTagTestDB(t *testing.T) {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldCommonGroupCol := commonGroupCol
	oldCommonKeyCol := commonKeyCol
	oldCommonTrueVal := commonTrueVal
	oldCommonFalseVal := commonFalseVal

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.MemoryCacheEnabled = false
	commonGroupCol = "`group`"
	commonKeyCol = "`key`"
	commonTrueVal = "1"
	commonFalseVal = "0"

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&Channel{}, &Ability{}); err != nil {
		t.Fatalf("failed to migrate channel tag test tables: %v", err)
	}

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		commonGroupCol = oldCommonGroupCol
		commonKeyCol = oldCommonKeyCol
		commonTrueVal = oldCommonTrueVal
		commonFalseVal = oldCommonFalseVal
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestTagAggregationIncludesUntaggedChannels(t *testing.T) {
	setupChannelTagTestDB(t)

	emptyTag := ""
	channels := []Channel{
		{Name: "untagged nil", Key: "key-1", Models: "gpt-test", Group: "default"},
		{Name: "untagged empty", Key: "key-2", Models: "gpt-test", Group: "default", Tag: &emptyTag},
		{Name: "tagged alpha", Key: "key-3", Models: "gpt-test", Group: "default", Tag: common.GetPointer("alpha")},
	}
	if err := DB.Create(&channels).Error; err != nil {
		t.Fatalf("failed to create channels: %v", err)
	}

	total, err := CountAllTags()
	if err != nil {
		t.Fatalf("failed to count tags: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected untagged bucket plus alpha tag, got %d", total)
	}

	tags, err := GetPaginatedTags(0, 10)
	if err != nil {
		t.Fatalf("failed to get paginated tags: %v", err)
	}
	tagSet := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if tag == nil {
			t.Fatal("expected tag buckets to be non-nil")
		}
		tagSet[*tag] = true
	}
	if !tagSet[""] || !tagSet["alpha"] {
		t.Fatalf("expected tags to contain untagged bucket and alpha, got %#v", tagSet)
	}

	untagged, err := GetChannelsByTag("", false, false)
	if err != nil {
		t.Fatalf("failed to get untagged channels: %v", err)
	}
	if len(untagged) != 2 {
		t.Fatalf("expected two untagged channels, got %d", len(untagged))
	}

	if err := DisableChannelByTag(""); err != nil {
		t.Fatalf("failed to disable untagged channels: %v", err)
	}
	var disabled int64
	if err := withTagCondition(DB.Model(&Channel{}), "").
		Where("status = ?", common.ChannelStatusManuallyDisabled).
		Count(&disabled).Error; err != nil {
		t.Fatalf("failed to count disabled untagged channels: %v", err)
	}
	if disabled != 2 {
		t.Fatalf("expected two disabled untagged channels, got %d", disabled)
	}
}

func TestArchivedChannelIsHiddenFromRuntimeSelection(t *testing.T) {
	setupChannelTagTestDB(t)

	active := Channel{
		Name:   "active",
		Key:    "key-active",
		Models: "gpt-test",
		Group:  "default",
		Status: common.ChannelStatusEnabled,
	}
	if err := active.Insert(); err != nil {
		t.Fatalf("failed to insert active channel: %v", err)
	}

	archived := Channel{
		Name:     "archived",
		Key:      "key-archived",
		Models:   "gpt-test",
		Group:    "default",
		Status:   common.ChannelStatusEnabled,
		Archived: true,
	}
	if err := archived.Insert(); err != nil {
		t.Fatalf("failed to insert archived channel: %v", err)
	}

	var archivedAbility Ability
	if err := DB.First(&archivedAbility, "channel_id = ?", archived.Id).Error; err != nil {
		t.Fatalf("failed to load archived ability: %v", err)
	}
	if archivedAbility.Enabled {
		t.Fatal("expected archived channel ability to be disabled")
	}

	selected, err := GetChannel("default", "gpt-test", 0)
	if err != nil {
		t.Fatalf("failed to select channel: %v", err)
	}
	if selected == nil || selected.Id != active.Id {
		t.Fatalf("expected active channel to be selected, got %#v", selected)
	}

	archivedActive, err := SetChannelArchived(active.Id, true)
	if err != nil {
		t.Fatalf("failed to archive active channel: %v", err)
	}
	if !archivedActive.Archived || archivedActive.Status != common.ChannelStatusManuallyDisabled {
		t.Fatalf("expected archive to mark channel archived and disabled, got archived=%v status=%d", archivedActive.Archived, archivedActive.Status)
	}

	selected, err = GetChannel("default", "gpt-test", 0)
	if err != nil {
		t.Fatalf("failed to select after archiving: %v", err)
	}
	if selected != nil {
		t.Fatalf("expected no selectable channel after archiving all channels, got %#v", selected)
	}

	if !UpdateChannelStatus(active.Id, "", common.ChannelStatusEnabled, "") {
		t.Fatal("expected enabling archived channel to update status")
	}
	var reenabled Channel
	if err := DB.First(&reenabled, "id = ?", active.Id).Error; err != nil {
		t.Fatalf("failed to reload reenabled channel: %v", err)
	}
	if reenabled.Archived || reenabled.Status != common.ChannelStatusEnabled {
		t.Fatalf("expected enable to unarchive channel, got archived=%v status=%d", reenabled.Archived, reenabled.Status)
	}
}
