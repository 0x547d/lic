package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/0x547d/lic/config"
	"github.com/0x547d/lic/models"
	"github.com/0x547d/lic/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WebHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewWebHandler(db *gorm.DB, cfg *config.Config) *WebHandler {
	return &WebHandler{DB: db, Cfg: cfg}
}

// ApplyPage 客户自助申请页面
func (h *WebHandler) ApplyPage(c *gin.Context) {
	c.HTML(http.StatusOK, "apply.html", nil)
}

// AdminLoginPage 管理后台登录页
func (h *WebHandler) AdminLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "admin_login.html", nil)
}

// AdminDashboardPage 管理后台主页
func (h *WebHandler) AdminDashboardPage(c *gin.Context) {
	c.HTML(http.StatusOK, "admin_dashboard.html", nil)
}

// WebLogin 管理后台登录接口
type WebLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type WebLoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

func (h *WebHandler) WebLogin(c *gin.Context) {
	var req WebLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}
	var user models.User
	err := h.DB.Where("username = ?", req.Username).First(&user).Error
	if err != nil || !utils.CheckPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	var license models.License
	h.DB.Where("user_id = ? AND status = ?", user.ID, models.LicenseStatusActive).First(&license)
	licenseKey := ""
	licenseID := uint(0)
	if license.ID > 0 {
		licenseKey = license.LicenseKey
		licenseID = license.ID
	}
	token, err := utils.GenerateJWT(user.ID, user.Username, licenseKey, licenseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}
	c.JSON(http.StatusOK, WebLoginResponse{Token: token, Username: user.Username})
}

// HandleApply 客户提交授权申请
type ApplyRequest struct {
	ApplicantName  string   `json:"applicant_name" binding:"required"`
	Email          string   `json:"email" binding:"required,email"`
	Phone          string   `json:"phone"`
	ProductKeys    []string `json:"product_keys" binding:"required,min=1"`
	DurationMonths int      `json:"duration_months" binding:"required,min=1"`
	MaxActivations int      `json:"max_activations" binding:"required,min=1"`
	Description    string   `json:"description"`
}

func (h *WebHandler) HandleApply(c *gin.Context) {
	var req ApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	record := models.ApplyRecord{
		ApplicantName:  req.ApplicantName,
		Email:          req.Email,
		Phone:          req.Phone,
		ProductKeys:    models.JSONSlice(req.ProductKeys),
		DurationMonths: req.DurationMonths,
		MaxActivations: req.MaxActivations,
		Description:    req.Description,
		Status:         "pending",
	}
	h.DB.Create(&record)

	// 发邮件通知管理员
	subject, body := utils.BuildApplyNotifyAdmin(&record)
	to := h.Cfg.AdminEmail
	if to == "" {
		to = h.Cfg.SMTPFrom // 兜底：用发件人地址
	}
	utils.SendEmail(h.Cfg, to, subject, body)

	c.JSON(http.StatusOK, gin.H{
		"apply_id": uuid.New().String()[:8],
		"message":  "申请已提交，我们将尽快审核并通过邮件通知您",
	})
}

// ListApplications 管理员查看申请列表
func (h *WebHandler) ListApplications(c *gin.Context) {
	var records []models.ApplyRecord
	h.DB.Order("id DESC").Find(&records)
	c.JSON(http.StatusOK, gin.H{"applications": records})
}

// ApproveApplication 管理员通过申请
type ReviewRequest struct {
	AdminNote string `json:"admin_note"`
}

func (h *WebHandler) ApproveApplication(c *gin.Context) {
	id := c.Param("id")
	var record models.ApplyRecord
	if err := h.DB.First(&record, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "申请记录不存在"})
		return
	}
	if record.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该申请已处理"})
		return
	}

	var req ReviewRequest
	_ = c.ShouldBindJSON(&req)

	// 创建用户
	hash, _ := utils.HashPassword(uuid.New().String()[:12])
	user := models.User{
		Username: "user_" + uuid.New().String()[:8],
		Password: hash,
		Email:    record.Email,
	}
	h.DB.Create(&user)

	// 创建授权码
	validTo := time.Now().Add(time.Duration(record.DurationMonths) * 30 * 24 * time.Hour)
	license := models.License{
		UserID:         user.ID,
		ProductKeys:    record.ProductKeys,
		Status:         models.LicenseStatusActive,
		MaxActivations: record.MaxActivations,
		ActivatedCount: 0,
		ValidFrom:      time.Now(),
		ValidTo:        validTo,
		HardwareIDs:    models.JSONSlice{},
	}
	h.DB.Create(&license)

	// 更新申请记录
	h.DB.Model(&record).Updates(map[string]interface{}{
		"status":      "approved",
		"license_key": license.LicenseKey,
		"admin_note":  req.AdminNote,
	})

	// 发邮件通知申请人
	subject, body := utils.BuildApplyApproved(&record, license.LicenseKey)
	utils.SendEmail(h.Cfg, record.Email, subject, body)

	// 记录操作日志
	adminID := c.GetUint("user_id")
	adminName := c.GetString("username")
	utils.LogOperation(h.DB, adminID, adminName, "approve", license.LicenseKey,
		"通过申请 #"+id+"，授权码："+license.LicenseKey, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"message":     "申请已通过，邮件已发送至申请人",
		"license_key": license.LicenseKey,
	})
}

// RejectApplication 管理员拒绝申请
func (h *WebHandler) RejectApplication(c *gin.Context) {
	id := c.Param("id")
	var record models.ApplyRecord
	if err := h.DB.First(&record, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "申请记录不存在"})
		return
	}
	if record.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该申请已处理"})
		return
	}

	var req ReviewRequest
	_ = c.ShouldBindJSON(&req)
	adminNote := req.AdminNote
	h.DB.Model(&record).Updates(map[string]interface{}{
		"status":     "rejected",
		"admin_note": adminNote,
	})

	// 发邮件通知申请人
	subject, body := utils.BuildApplyRejected(&record, adminNote)
	utils.SendEmail(h.Cfg, record.Email, subject, body)

	// 记录操作日志
	adminID := c.GetUint("user_id")
	adminName := c.GetString("username")
	utils.LogOperation(h.DB, adminID, adminName, "reject", "",
		"拒绝申请 #"+id+" 原因："+adminNote, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "申请已拒绝，邮件已发送至申请人"})
}

// ListOperationLogs 查看操作日志（支持分页和筛选）
func (h *WebHandler) ListOperationLogs(c *gin.Context) {
	page := 1
	pageSize := 20
	if v := c.DefaultQuery("page", "1"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.DefaultQuery("page_size", "20"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			pageSize = n
		}
	}
	action := c.Query("action")

	query := h.DB.Model(&models.OperationLog{})
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if v := c.Query("start_time"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if v := c.Query("end_time"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	query.Count(&total)

	var logs []models.OperationLog
	offset := (page - 1) * pageSize
	query.Order("id DESC").Limit(pageSize).Offset(offset).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
