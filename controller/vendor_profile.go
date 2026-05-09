package controller

import (
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func validateVendorProfile(profile *model.VendorProfile) string {
	if profile == nil {
		return "参数错误"
	}
	if strings.TrimSpace(profile.VendorCode) == "" {
		return "供应商代码不能为空"
	}
	if strings.TrimSpace(profile.VendorName) == "" {
		return "供应商名称不能为空"
	}
	if strings.TrimSpace(profile.PlatformType) == "" {
		return "平台不能为空"
	}
	if profile.DiscountRate != nil && *profile.DiscountRate < 0 {
		return "折扣不能小于 0"
	}
	return ""
}

func fillVendorProfileCode(profile *model.VendorProfile) {
	if profile == nil || strings.TrimSpace(profile.Code) != "" {
		return
	}
	parts := []string{
		strings.TrimSpace(profile.VendorCode),
		strings.TrimSpace(profile.PlatformType),
		strings.TrimSpace(profile.DiscountCode),
	}
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}
	profile.Code = strings.Join(cleanParts, "-")
}

func GetAllVendorProfiles(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	profiles, err := model.GetAllVendorProfiles(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var total int64
	model.DB.Model(&model.VendorProfile{}).Count(&total)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(profiles)
	common.ApiSuccess(c, pageInfo)
}

func SearchVendorProfiles(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	profiles, total, err := model.SearchVendorProfiles(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(profiles)
	common.ApiSuccess(c, pageInfo)
}

func GetVendorProfile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	profile, err := model.GetVendorProfileByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func bindVendorProfile(c *gin.Context, profile *model.VendorProfile) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if err := common.Unmarshal(body, profile); err != nil {
		common.ApiError(c, err)
		return false
	}
	return true
}

func CreateVendorProfile(c *gin.Context) {
	var profile model.VendorProfile
	if !bindVendorProfile(c, &profile) {
		return
	}
	if message := validateVendorProfile(&profile); message != "" {
		common.ApiErrorMsg(c, message)
		return
	}
	fillVendorProfileCode(&profile)
	if dup, err := model.IsVendorProfileCodeDuplicated(0, profile.Code); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "供应商配置代码已存在")
		return
	}
	if err := profile.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, &profile)
}

func UpdateVendorProfile(c *gin.Context) {
	var profile model.VendorProfile
	if !bindVendorProfile(c, &profile) {
		return
	}
	if profile.Id == 0 {
		common.ApiErrorMsg(c, "缺少供应商配置 ID")
		return
	}
	if message := validateVendorProfile(&profile); message != "" {
		common.ApiErrorMsg(c, message)
		return
	}
	fillVendorProfileCode(&profile)
	if dup, err := model.IsVendorProfileCodeDuplicated(profile.Id, profile.Code); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "供应商配置代码已存在")
		return
	}
	if err := profile.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, &profile)
}

func DeleteVendorProfile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var count int64
	if err := model.DB.Model(&model.Channel{}).Where("vendor_profile_id = ?", id).Count(&count).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if count > 0 {
		common.ApiErrorMsg(c, "该供应商配置仍被渠道使用，不能删除")
		return
	}
	profile := model.VendorProfile{Id: id}
	if err := profile.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
