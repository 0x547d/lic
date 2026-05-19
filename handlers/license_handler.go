package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/0x547d/lic/models"
	"github.com/0x547d/lic/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LicenseHandler struct {
	DB *gorm.DB
}

func NewLicenseHandler(db *gorm.DB) *LicenseHandler {
	return &LicenseHandler{DB: db}
}

// CreateLicenseRequest 创建授权码请求
type CreateLicenseRequest struct {
	UserID         uint   `json:"user_id"`
	Username       string `json:"username"` // 可选，用用户名查找
	MaxActivations int    `json:"max_activations" binding:"required,min=1"`
	ValidMonths    int    `json:"valid_months"` // 按月（优先），0 或负数表示永久
	ValidDays      int    `json:"valid_days"`   // 按天（兼容旧版）
}

// CreateLicense 创建新授权码（管理员功能）
func (h *LicenseHandler) CreateLicense(c *gin.Context) {
	var req CreateLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找用户
	var user models.User
	if req.Username != "" {
		if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
	} else if req.UserID > 0 {
		if err := h.DB.First(&user, req.UserID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
	} else {
		// 使用当前 token 中的用户
		userID := c.GetUint("user_id")
		if userID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id or username required"})
			return
		}
		h.DB.First(&user, userID)
	}

	now := time.Now()
	license := models.License{
		UserID:         user.ID,
		LicenseKey:     uuid.New().String()[0:8] + "-" + uuid.New().String()[0:8] + "-" + uuid.New().String()[0:8],
		Status:         models.LicenseStatusActive,
		MaxActivations: req.MaxActivations,
		ActivatedCount: 0,
		ValidFrom:      now,
		HardwareIDs:    models.JSONSlice{},
	}

	if req.ValidMonths == 0 {
		// 永久有效
		license.IsPermanent = true
		license.ValidTo = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	} else if req.ValidMonths > 0 {
		license.ValidTo = now.AddDate(0, req.ValidMonths, 0)
	} else if req.ValidDays > 0 {
		license.ValidTo = now.Add(time.Duration(req.ValidDays) * 24 * time.Hour)
	} else {
		// 默认 365 天
		license.ValidTo = now.AddDate(0, 12, 0)
	}

	h.DB.Create(&license)

	adminID := c.GetUint("user_id")
	adminName := c.GetString("username")
	permanentStr := ""
	if license.IsPermanent {
		permanentStr = "（永久有效）"
	}
	utils.LogOperation(h.DB, adminID, adminName, "create", license.LicenseKey,
		fmt.Sprintf("创建授权码，有效期至 %s%s，最大激活数 %d",
			license.ValidTo.Format("2006-01-02"), permanentStr, license.MaxActivations),
		c.ClientIP())

	c.JSON(http.StatusCreated, gin.H{
		"message":         "license created",
		"license_key":     license.LicenseKey,
		"user_id":         user.ID,
		"username":        user.Username,
		"valid_from":      license.ValidFrom,
		"valid_to":        license.ValidTo,
		"is_permanent":    license.IsPermanent,
		"max_activations": license.MaxActivations,
	})
}

// ListLicenses 列出授权码（支持搜索、分页、统计）
func (h *LicenseHandler) ListLicenses(c *gin.Context) {
	userID := c.GetUint("user_id")
	keyword := c.Query("keyword")
	page := c.GetInt("page")
	if page < 1 {
		page = 1
	}
	pageSize := 10

	query := h.DB.Model(&models.License{})

	// 权限控制：admin 传 all=true 看全部，否则只看自己的
	if c.Query("all") != "true" {
		query = query.Where("user_id = ?", userID)
	}

	// 关键字搜索：匹配 license_key 或用户名
	if keyword != "" {
		query = query.Joins("LEFT JOIN users ON users.id = licenses.user_id").
			Where("licenses.license_key LIKE ? OR users.username LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 统计总数
	var total int64
	query.Count(&total)

	// 分页查询
	var licenses []models.License
	query.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&licenses)

	// 统计：有效/已过期/激活设备数
	var activeCount, expiredCount int64
	now := time.Now()
	h.DB.Model(&models.License{}).Where("status = ? AND valid_from <= ? AND valid_to >= ?", "active", now, now).Count(&activeCount)
	h.DB.Model(&models.License{}).Where("valid_to < ?", now).Count(&expiredCount)
	var activationCount int64
	h.DB.Model(&models.Activation{}).Where("is_active = ?", true).Count(&activationCount)

	// 如果是 admin 请求，附加用户名
	type LicenseWithUser struct {
		models.License
		Username string `json:"username"`
	}
	var result []LicenseWithUser
	for _, l := range licenses {
		var u models.User
		h.DB.First(&u, l.UserID)
		result = append(result, LicenseWithUser{License: l, Username: u.Username})
	}

	c.JSON(http.StatusOK, gin.H{
		"licenses":         result,
		"total":            total,
		"active_count":     activeCount,
		"expired_count":    expiredCount,
		"activation_count": activationCount,
		"page":             page,
		"page_size":        pageSize,
		"total_pages":      (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// GetLicense 获取单个授权码详情
func (h *LicenseHandler) GetLicense(c *gin.Context) {
	licenseKey := c.Param("licenseKey")

	var license models.License
	if err := h.DB.Where("license_key = ?", licenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	// 查询激活记录
	var activations []models.Activation
	h.DB.Where("license_id = ? AND is_active = ?", license.ID, true).Find(&activations)

	c.JSON(http.StatusOK, gin.H{
		"license":     license,
		"activations": activations,
		"is_valid":    license.IsValid(),
	})
}

// RevokeLicense 撤销授权码
func (h *LicenseHandler) RevokeLicense(c *gin.Context) {
	licenseKey := c.Param("licenseKey")

	var license models.License
	if err := h.DB.Where("license_key = ?", licenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	h.DB.Model(&license).Update("status", models.LicenseStatusRevoked)

	adminID := c.GetUint("user_id")
	adminName := c.GetString("username")
	utils.LogOperation(h.DB, adminID, adminName, "revoke", licenseKey,
		"撤销授权码", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "license revoked"})
}

// ExtendLicenseRequest 延长授权码有效期请求
type ExtendLicenseRequest struct {
	ExtraMonths int `json:"extra_months"` // 按月（优先）
	ExtraDays   int `json:"extra_days"`   // 按天（兼容旧版）
}

// ExtendLicense 延长授权码有效期
func (h *LicenseHandler) ExtendLicense(c *gin.Context) {
	licenseKey := c.Param("licenseKey")
	var req ExtendLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var license models.License
	if err := h.DB.Where("license_key = ?", licenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	var newValidTo time.Time
	if license.IsPermanent {
		// 已是永久授权，无需延长
		newValidTo = license.ValidTo
	} else if req.ExtraMonths > 0 {
		newValidTo = license.ValidTo.AddDate(0, req.ExtraMonths, 0)
	} else if req.ExtraDays > 0 {
		newValidTo = license.ValidTo.Add(time.Duration(req.ExtraDays) * 24 * time.Hour)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extra_months or extra_days required"})
		return
	}

	h.DB.Model(&license).Update("valid_to", newValidTo)

	adminID := c.GetUint("user_id")
	adminName := c.GetString("username")
	utils.LogOperation(h.DB, adminID, adminName, "extend", licenseKey,
		fmt.Sprintf("延长，新的有效期至 %s", newValidTo.Format("2006-01-02")),
		c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"message":      "license extended",
		"valid_to":     newValidTo,
		"is_permanent": license.IsPermanent,
	})
}

// ListActivations 列出激活记录
func (h *LicenseHandler) ListActivations(c *gin.Context) {
	licenseKey := c.Param("licenseKey")

	var license models.License
	if err := h.DB.Where("license_key = ?", licenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	var activations []models.Activation
	h.DB.Where("license_id = ?", license.ID).Find(&activations)

	c.JSON(http.StatusOK, gin.H{"activations": activations})
}

// DeactivateDevice 禁用某台设备的激活
func (h *LicenseHandler) DeactivateDevice(c *gin.Context) {
	licenseKey := c.Param("licenseKey")
	deviceFP := c.Param("deviceFP")

	var license models.License
	if err := h.DB.Where("license_key = ?", licenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	result := h.DB.Model(&models.Activation{}).
		Where("license_id = ? AND device_fingerprint = ?", license.ID, deviceFP).
		Update("is_active", false)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "activation not found"})
		return
	}

	adminID := c.GetUint("user_id")
	adminName := c.GetString("username")
	utils.LogOperation(h.DB, adminID, adminName, "deactivate", licenseKey+"/"+deviceFP,
		"禁用设备激活", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "device deactivated"})
}
