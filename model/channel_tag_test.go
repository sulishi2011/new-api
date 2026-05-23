package model

import (
	"database/sql"
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

	if err := db.AutoMigrate(&Channel{}, &Ability{}, &VendorProfile{}); err != nil {
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

func TestChannelVendorProfileZeroIsStoredAsNull(t *testing.T) {
	setupChannelTagTestDB(t)

	zero := 0
	channels := []Channel{
		{
			Name:            "no vendor profile",
			Key:             "key-no-profile",
			Models:          "gpt-test",
			Group:           "default",
			VendorProfileId: &zero,
		},
	}
	if err := BatchInsertChannels(channels); err != nil {
		t.Fatalf("failed to insert channel without vendor profile: %v", err)
	}

	var stored sql.NullInt64
	if err := DB.Raw("SELECT vendor_profile_id FROM channels WHERE name = ?", "no vendor profile").Scan(&stored).Error; err != nil {
		t.Fatalf("failed to inspect stored vendor profile id: %v", err)
	}
	if stored.Valid {
		t.Fatalf("expected vendor_profile_id to be NULL, got %d", stored.Int64)
	}

	var loaded Channel
	if err := DB.First(&loaded, "name = ?", "no vendor profile").Error; err != nil {
		t.Fatalf("failed to reload channel: %v", err)
	}
	if loaded.VendorProfileId != nil {
		t.Fatalf("expected loaded vendor profile id to be nil, got %d", *loaded.VendorProfileId)
	}
}

func TestChannelVendorProfileCanBeClearedToNull(t *testing.T) {
	setupChannelTagTestDB(t)

	profile := VendorProfile{
		Code:         "openai-default",
		VendorCode:   "openai",
		VendorName:   "OpenAI",
		PlatformType: "api",
	}
	if err := profile.Insert(); err != nil {
		t.Fatalf("failed to insert vendor profile: %v", err)
	}

	channel := Channel{
		Name:            "with vendor profile",
		Key:             "key-with-profile",
		Models:          "gpt-test",
		Group:           "default",
		VendorProfileId: &profile.Id,
	}
	if err := channel.Insert(); err != nil {
		t.Fatalf("failed to insert channel with vendor profile: %v", err)
	}

	var before sql.NullInt64
	if err := DB.Raw("SELECT vendor_profile_id FROM channels WHERE id = ?", channel.Id).Scan(&before).Error; err != nil {
		t.Fatalf("failed to inspect associated vendor profile id: %v", err)
	}
	if !before.Valid || int(before.Int64) != profile.Id {
		t.Fatalf("expected vendor_profile_id %d before clear, got valid=%v value=%d", profile.Id, before.Valid, before.Int64)
	}

	zero := 0
	if err := DB.Model(&Channel{}).
		Where("id = ?", channel.Id).
		Update("vendor_profile_id", ChannelVendorProfileIdDBValue(&zero)).Error; err != nil {
		t.Fatalf("failed to clear vendor profile id: %v", err)
	}

	var after sql.NullInt64
	if err := DB.Raw("SELECT vendor_profile_id FROM channels WHERE id = ?", channel.Id).Scan(&after).Error; err != nil {
		t.Fatalf("failed to inspect cleared vendor profile id: %v", err)
	}
	if after.Valid {
		t.Fatalf("expected cleared vendor_profile_id to be NULL, got %d", after.Int64)
	}
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

func TestGetChannelSkipsExcludedChannelWhenAlternativeExists(t *testing.T) {
	setupChannelTagTestDB(t)

	channels := []Channel{
		{
			Name:   "failed",
			Key:    "key-failed",
			Models: "gpt-test",
			Group:  "default",
			Status: common.ChannelStatusEnabled,
		},
		{
			Name:   "alternative",
			Key:    "key-alternative",
			Models: "gpt-test",
			Group:  "default",
			Status: common.ChannelStatusEnabled,
		},
	}
	if err := BatchInsertChannels(channels); err != nil {
		t.Fatalf("failed to insert channels: %v", err)
	}

	var failed Channel
	if err := DB.First(&failed, "name = ?", "failed").Error; err != nil {
		t.Fatalf("failed to reload failed channel: %v", err)
	}
	var alternative Channel
	if err := DB.First(&alternative, "name = ?", "alternative").Error; err != nil {
		t.Fatalf("failed to reload alternative channel: %v", err)
	}

	selected, err := GetChannel("default", "gpt-test", 0, failed.Id)
	if err != nil {
		t.Fatalf("failed to select channel: %v", err)
	}
	if selected == nil || selected.Id != alternative.Id {
		t.Fatalf("expected alternative channel %d, got %#v", alternative.Id, selected)
	}
}

func TestGetChannelFallsBackWhenOnlyExcludedChannelExists(t *testing.T) {
	setupChannelTagTestDB(t)

	channel := Channel{
		Name:   "only",
		Key:    "key-only",
		Models: "gpt-test",
		Group:  "default",
		Status: common.ChannelStatusEnabled,
	}
	if err := channel.Insert(); err != nil {
		t.Fatalf("failed to insert channel: %v", err)
	}

	selected, err := GetChannel("default", "gpt-test", 0, channel.Id)
	if err != nil {
		t.Fatalf("failed to select channel: %v", err)
	}
	if selected == nil || selected.Id != channel.Id {
		t.Fatalf("expected excluded channel fallback %d, got %#v", channel.Id, selected)
	}
}
