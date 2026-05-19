package handlers

import (
	"net/http"
	"time"

	"github.com/0x547d/lic/models"
	"github.com/0x547d/lic/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 帐密登录，返回 JWT Token
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var user models.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !utils.CheckPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// 查找用户第一个有效授权码，用于初始 token
	var license models.License
	h.DB.Where("user_id = ? AND status = ? AND valid_from <= ? AND valid_to >= ?",
		user.ID, models.LicenseStatusActive, time.Now(), time.Now()).
		First(&license)

	licenseKey := ""
	licenseID := uint(0)
	if license.ID > 0 {
		licenseKey = license.LicenseKey
		licenseID = license.ID
	}

	token, err := utils.GenerateJWT(user.ID, user.Username, licenseKey, licenseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user_id":     user.ID,
		"username":    user.Username,
		"license_key": licenseKey,
		"expires_in":  int(utils.JWTExpire.Seconds()),
	})
}

// VerifyToken 验证 Token 是否有效（客户端启动时调用）
type VerifyRequest struct {
	LicenseKey string `json:"license_key"`
}

func (h *AuthHandler) VerifyToken(c *gin.Context) {
	// JWT 中间件已验证 token，这里检查授权码状态
	licenseKey := c.GetString("license_key")
	if licenseKey == "" {
		var req VerifyRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			licenseKey = req.LicenseKey
		}
	}

	if licenseKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing license_key"})
		return
	}

	var license models.License
	if err := h.DB.Where("license_key = ?", licenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	// 查询关联的用户信息（获取公司/个人名称）
	var user models.User
	h.DB.First(&user, license.UserID)

	valid := license.IsValid()
	remaining := license.MaxActivations - license.ActivatedCount
	if remaining < 0 {
		remaining = 0
	}

	resp := gin.H{
		"valid":           valid,
		"status":          string(license.Status),
		"valid_from":      license.ValidFrom,
		"valid_to":        license.ValidTo,
		"license_key":     license.LicenseKey,
		"company":         user.Username,          // 公司/个人名称（即username字段）
		"product_keys":    license.ProductKeys,    // 产品类型列表
		"activated_count": license.ActivatedCount, // 已激活设备数量
		"max_activations": license.MaxActivations, // 最大激活数
		"remaining_quota": remaining,              // 剩余额度
	}
	if !valid {
		resp["reason"] = h.getInvalidReason(&license)
	}
	c.JSON(http.StatusOK, resp)
}

// ActivateOnline 在线激活（绑定设备指纹）
type ActivateRequest struct {
	LicenseKey        string `json:"license_key" binding:"required"`
	DeviceFingerprint string `json:"device_fingerprint" binding:"required"`
	ClientVersion     string `json:"client_version"`
}

func (h *AuthHandler) ActivateOnline(c *gin.Context) {
	var req ActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var license models.License
	if err := h.DB.Where("license_key = ?", req.LicenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	if !license.IsValid() {
		c.JSON(http.StatusForbidden, gin.H{"error": "license is not valid", "reason": h.getInvalidReason(&license)})
		return
	}

	// 检查是否已激活该设备
	var existing models.Activation
	err := h.DB.Where("license_id = ? AND device_fingerprint = ? AND is_active = ?",
		license.ID, req.DeviceFingerprint, true).First(&existing).Error

	if err == nil {
		// 已激活，更新验证时间
		h.DB.Model(&existing).Update("last_verified_at", time.Now())
		c.JSON(http.StatusOK, gin.H{"message": "already activated", "activation_id": existing.ID})
		return
	}

	// 检查激活数是否超限
	if !license.CanActivate() {
		c.JSON(http.StatusForbidden, gin.H{"error": "max activations reached", "max": license.MaxActivations, "activated": license.ActivatedCount})
		return
	}

	// 创建激活记录
	activation := models.Activation{
		LicenseID:         license.ID,
		DeviceFingerprint: req.DeviceFingerprint,
		Method:            models.ActivationMethodOnline,
		ActivatedAt:       time.Now(),
		LastVerifiedAt:    time.Now(),
		ClientVersion:     req.ClientVersion,
		ClientIP:          c.ClientIP(),
		IsActive:          true,
	}
	h.DB.Create(&activation)
	h.DB.Model(&license).Update("activated_count", license.ActivatedCount+1)

	// 更新硬件ID列表
	var hwIDs models.JSONSlice
	h.DB.Model(&license).Select("hardware_ids").First(&license)
	hwIDs = license.HardwareIDs
	if hwIDs == nil {
		hwIDs = models.JSONSlice{}
	}
	hwIDs = append(hwIDs, req.DeviceFingerprint)
	h.DB.Model(&license).Update("hardware_ids", hwIDs)

	c.JSON(http.StatusOK, gin.H{
		"message":       "activation successful",
		"activation_id": activation.ID,
		"valid_to":      license.ValidTo,
	})
}

// OfflineRequestGen 生成离线激活请求（客户端调用，然后手动传输到服务端）
type OfflineRequestGenRequest struct {
	LicenseKey        string `json:"license_key" binding:"required"`
	DeviceFingerprint string `json:"device_fingerprint" binding:"required"`
	ClientVersion     string `json:"client_version"`
}

func (h *AuthHandler) OfflineRequestGen(c *gin.Context) {
	var req OfflineRequestGenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 验证授权码
	var license models.License
	if err := h.DB.Where("license_key = ?", req.LicenseKey).First(&license).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	if !license.IsValid() {
		c.JSON(http.StatusForbidden, gin.H{"error": "license is not valid"})
		return
	}

	// 生成离线请求记录
	offlineReq := models.OfflineRequest{
		LicenseID:         license.ID,
		RequestToken:      utils.GenerateRequestToken(),
		DeviceFingerprint: req.DeviceFingerprint,
		Status:            "pending",
		ExpiresAt:         time.Now().Add(7 * 24 * time.Hour), // 7天有效
	}
	h.DB.Create(&offlineReq)

	// 生成请求文件内容（客户端下载此文件，手动传到有网络的机器）
	reqFile, _ := utils.BuildOfflineRequest(req.LicenseKey, req.DeviceFingerprint, req.ClientVersion, "")

	reqJSON, _ := utils.EncodeRequestFile(reqFile)
	c.JSON(http.StatusOK, gin.H{
		"message":       "offline request created",
		"request_token": offlineReq.RequestToken,
		"request_file":  string(reqJSON),
		"download_url":  "/api/v1/offline/request/" + offlineReq.RequestToken + "/download",
		"expires_at":    offlineReq.ExpiresAt,
		"instructions":  "将此请求文件内容保存到 request.json，在联网机器上执行离线激活",
	})
}

// OfflineRequestDownload 下载离线激活请求文件
func (h *AuthHandler) OfflineRequestDownload(c *gin.Context) {
	token := c.Param("token")
	var offlineReq models.OfflineRequest
	if err := h.DB.Where("request_token = ?", token).First(&offlineReq).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}

	var license models.License
	h.DB.First(&license, offlineReq.LicenseID)

	reqFile, _ := utils.BuildOfflineRequest(license.LicenseKey, offlineReq.DeviceFingerprint, "", "")

	reqJSON, _ := utils.EncodeRequestFile(reqFile)
	c.Header("Content-Disposition", "attachment; filename=request-"+token[:8]+".json")
	c.Data(http.StatusOK, "application/json", reqJSON)
}

// OfflineActivate 处理离线激活请求，生成响应文件（在有网络的机器上执行）
func (h *AuthHandler) OfflineActivate(c *gin.Context) {
	token := c.Param("token")

	var offlineReq models.OfflineRequest
	if err := h.DB.Where("request_token = ?", token).First(&offlineReq).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}

	if offlineReq.Status == "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request already completed"})
		return
	}

	if time.Now().After(offlineReq.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request expired"})
		return
	}

	var license models.License
	if err := h.DB.First(&license, offlineReq.LicenseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "license not found"})
		return
	}

	if !license.IsValid() {
		c.JSON(http.StatusForbidden, gin.H{"error": "license is not valid"})
		return
	}

	// 生成离线激活响应文件
	respFile, err := utils.BuildOfflineResponse(&license, offlineReq.DeviceFingerprint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build response"})
		return
	}

	respJSON, err := utils.EncodeResponseFile(respFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode response"})
		return
	}

	// 更新请求状态
	h.DB.Model(&offlineReq).Updates(map[string]interface{}{
		"status":        "completed",
		"response_file": string(respJSON),
	})

	// 创建激活记录
	var existing models.Activation
	err = h.DB.Where("license_id = ? AND device_fingerprint = ?", license.ID, offlineReq.DeviceFingerprint).First(&existing).Error
	if err != nil {
		activation := models.Activation{
			LicenseID:         license.ID,
			DeviceFingerprint: offlineReq.DeviceFingerprint,
			Method:            models.ActivationMethodOffline,
			ActivatedAt:       time.Now(),
			LastVerifiedAt:    time.Now(),
			IsActive:          true,
		}
		h.DB.Create(&activation)
		if license.ActivatedCount < license.MaxActivations {
			h.DB.Model(&license).Update("activated_count", license.ActivatedCount+1)
		}
	}

	c.Header("Content-Disposition", "attachment; filename=response-"+token[:8]+".json")
	c.Data(http.StatusOK, "application/json", respJSON)
}

// OfflineVerify 客户端验证离线激活响应文件
type OfflineVerifyRequest struct {
	ResponseFile string `json:"response_file" binding:"required"` // JSON 字符串
}

func (h *AuthHandler) OfflineVerify(c *gin.Context) {
	var req OfflineVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	resp, err := utils.DecodeResponseFile([]byte(req.ResponseFile))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid response file"})
		return
	}

	// 用服务端公钥验证签名
	valid, err := utils.VerifyOfflineResponse(resp, resp.Certificate)
	if err != nil || !valid {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature", "detail": err.Error()})
		return
	}

	// 检查时间有效性
	now := time.Now()
	if now.Before(resp.ValidFrom) || now.After(resp.ValidTo) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "license expired",
			"valid_from": resp.ValidFrom,
			"valid_to":   resp.ValidTo,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":              true,
		"license_key":        resp.LicenseKey,
		"device_fingerprint": resp.DeviceFingerprint,
		"valid_from":         resp.ValidFrom,
		"valid_to":           resp.ValidTo,
		"issued_at":          resp.IssuedAt,
	})
}

func (h *AuthHandler) getInvalidReason(license *models.License) string {
	now := time.Now()
	if license.Status != models.LicenseStatusActive {
		return "license " + string(license.Status)
	}
	if now.Before(license.ValidFrom) {
		return "license not yet valid"
	}
	if now.After(license.ValidTo) {
		return "license expired"
	}
	return ""
}

// Register 注册新用户（管理员功能）
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查用户名是否已存在
	var count int64
	h.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	// 检查邮箱是否已存在（如果提供了邮箱）
	if req.Email != "" {
		h.DB.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
			return
		}
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := models.User{
		Username: req.Username,
		Password: hash,
		Email:    req.Email,
	}
	h.DB.Create(&user)

	// 自动生成默认授权码（1年有效，1台设备）
	license := models.License{
		UserID:         user.ID,
		Status:         models.LicenseStatusActive,
		MaxActivations: 1,
		ValidFrom:      time.Now(),
		ValidTo:        time.Now().Add(365 * 24 * time.Hour),
		HardwareIDs:    models.JSONSlice{},
	}
	h.DB.Create(&license)

	c.JSON(http.StatusCreated, gin.H{
		"message":     "registration successful",
		"user_id":     user.ID,
		"license_key": license.LicenseKey,
	})
}
