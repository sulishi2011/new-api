package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

type VendorProfile struct {
	Id            int            `json:"id"`
	Code          string         `json:"code" gorm:"size:64;not null;uniqueIndex:uk_vendor_profile_code_delete_at,priority:1"`
	VendorCode    string         `json:"vendor_code" gorm:"size:32;not null;index"`
	VendorName    string         `json:"vendor_name" gorm:"size:128;not null"`
	PlatformType  string         `json:"platform_type" gorm:"size:32;not null;index"`
	DiscountCode  string         `json:"discount_code" gorm:"size:32;default:''"`
	DiscountLabel string         `json:"discount_label" gorm:"size:64;default:''"`
	DiscountRate  *float64       `json:"discount_rate,omitempty" gorm:"type:decimal(18,8)"`
	Status        int            `json:"status" gorm:"default:1;index"`
	Notes         string         `json:"notes,omitempty" gorm:"type:text"`
	CreatedTime   int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime   int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_vendor_profile_code_delete_at,priority:2"`
}

func normalizeVendorProfile(profile *VendorProfile) {
	if profile == nil {
		return
	}
	profile.Code = strings.TrimSpace(profile.Code)
	profile.VendorCode = strings.TrimSpace(profile.VendorCode)
	profile.VendorName = strings.TrimSpace(profile.VendorName)
	profile.PlatformType = strings.TrimSpace(profile.PlatformType)
	profile.DiscountCode = strings.TrimSpace(profile.DiscountCode)
	profile.DiscountLabel = strings.TrimSpace(profile.DiscountLabel)
	profile.Notes = strings.TrimSpace(profile.Notes)
	if profile.Code == "" {
		parts := []string{profile.VendorCode, profile.PlatformType, profile.DiscountCode}
		cleanParts := make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				cleanParts = append(cleanParts, part)
			}
		}
		profile.Code = strings.Join(cleanParts, "-")
	}
}

func (profile *VendorProfile) Insert() error {
	normalizeVendorProfile(profile)
	now := common.GetTimestamp()
	profile.CreatedTime = now
	profile.UpdatedTime = now
	if profile.Status == 0 {
		profile.Status = 1
	}
	return DB.Create(profile).Error
}

func (profile *VendorProfile) Update() error {
	normalizeVendorProfile(profile)
	profile.UpdatedTime = common.GetTimestamp()
	if profile.Status == 0 {
		profile.Status = 1
	}
	return DB.Model(&VendorProfile{}).Where("id = ?", profile.Id).Updates(map[string]interface{}{
		"code":           profile.Code,
		"vendor_code":    profile.VendorCode,
		"vendor_name":    profile.VendorName,
		"platform_type":  profile.PlatformType,
		"discount_code":  profile.DiscountCode,
		"discount_label": profile.DiscountLabel,
		"discount_rate":  profile.DiscountRate,
		"status":         profile.Status,
		"notes":          profile.Notes,
		"updated_time":   profile.UpdatedTime,
	}).Error
}

func (profile *VendorProfile) Delete() error {
	return DB.Delete(profile).Error
}

func IsVendorProfileCodeDuplicated(id int, code string) (bool, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&VendorProfile{}).Where("code = ? AND id <> ?", code, id).Count(&cnt).Error
	return cnt > 0, err
}

func GetVendorProfileByID(id int) (*VendorProfile, error) {
	var profile VendorProfile
	err := DB.First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func GetAllVendorProfiles(offset int, limit int) ([]*VendorProfile, error) {
	var profiles []*VendorProfile
	err := DB.Order("id DESC").Offset(offset).Limit(limit).Find(&profiles).Error
	return profiles, err
}

func SearchVendorProfiles(keyword string, offset int, limit int) ([]*VendorProfile, int64, error) {
	db := DB.Model(&VendorProfile{})
	if keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		db = db.Where("code LIKE ? OR vendor_code LIKE ? OR vendor_name LIKE ? OR platform_type LIKE ? OR discount_code LIKE ? OR discount_label LIKE ?", like, like, like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var profiles []*VendorProfile
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&profiles).Error; err != nil {
		return nil, 0, err
	}
	return profiles, total, nil
}
