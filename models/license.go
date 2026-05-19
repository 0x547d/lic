package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LicenseStatus 授权码状态
type LicenseStatus string

const (
	LicenseStatusActive   LicenseStatus = "active"   // 有效
	LicenseStatusExpired  LicenseStatus = "expired"  // 已过期
	LicenseStatusDisabled LicenseStatus = "disabled" // 已禁用
	LicenseStatusRevoked  LicenseStatus = "revoked"  // 已撤销
)

// ActivationMethod 激活方式
type ActivationMethod string

const (
	ActivationMethodOnline  ActivationMethod = "online"  // 在线激活
	ActivationMethodOffline ActivationMethod = "offline" // 离线激活
)

// User 用户表（用于帐密登录）
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Password  string    `gorm:"size:128;not null" json:"-"` // bcrypt 哈希
	Email     string    `gorm:"size:128" json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Licenses  []License `gorm:"foreignKey:UserID" json:"-"`
}

// License 授权码表
type License struct {
	ID              uint             `gorm:"primaryKey" json:"id"`
	UserID          uint             `gorm:"index;not null" json:"user_id"`
	ProductKeys     models.JSONSlice `gorm:"type:text;index" json:"product_keys"`             // 关联产品标识（多选）
	LicenseKey      string           `gorm:"uniqueIndex;size:64;not null" json:"license_key"` // 授权码字符串
	Status          LicenseStatus    `gorm:"size:20;not null;default:active" json:"status"`
	MaxActivations  int              `gorm:"default:1" json:"max_activations"`      // 最大激活数
	ActivatedCount  int              `gorm:"default:0" json:"activated_count"`      // 已激活数
	ValidFrom       time.Time        `json:"valid_from"`                            // 有效期开始
	ValidTo         time.Time        `json:"valid_to"`                              // 有效期结束
	Notified7Days   bool             `gorm:"default:false" json:"notified_7days"`   // 已发送7天到期提醒
	Notified1Day    bool             `gorm:"default:false" json:"notified_1day"`    // 已发送1天到期提醒
	NotifiedExpired bool             `gorm:"default:false" json:"notified_expired"` // 已发送已过期通知
	IsPermanent     bool             `gorm:"default:false" json:"is_permanent"`     // 永久有效
	HardwareIDs     JSONSlice        `gorm:"type:text" json:"hardware_ids"`         // 绑定的硬件ID列表
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Activations     []Activation     `gorm:"foreignKey:LicenseID" json:"-"`
}

// Activation 激活记录表
type Activation struct {
	ID                uint             `gorm:"primaryKey" json:"id"`
	LicenseID         uint             `gorm:"index;not null" json:"license_id"`
	DeviceFingerprint string           `gorm:"size:128;not null" json:"device_fingerprint"` // 设备指纹
	Method            ActivationMethod `gorm:"size:20;not null" json:"method"`
	ActivatedAt       time.Time        `json:"activated_at"`
	LastVerifiedAt    time.Time        `json:"last_verified_at"`
	ClientVersion     string           `gorm:"size:64" json:"client_version"`
	ClientIP          string           `gorm:"size:45" json:"client_ip"`
	IsActive          bool             `gorm:"default:true" json:"is_active"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// OfflineRequest 离线激活请求记录
type OfflineRequest struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	LicenseID         uint      `gorm:"index" json:"license_id"`
	RequestToken      string    `gorm:"uniqueIndex;size:128;not null" json:"request_token"` // 请求令牌
	DeviceFingerprint string    `gorm:"size:128;not null" json:"device_fingerprint"`
	ResponseFile      string    `gorm:"type:text" json:"response_file"`        // 离线响应文件内容（加密）
	Status            string    `gorm:"size:20;default:pending" json:"status"` // pending/completed/expired
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// JSONSlice 自定义 GORM 类型，用于存储字符串数组
type JSONSlice []string

func (j JSONSlice) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONSlice) Scan(value interface{}) error {
	if value == nil {
		*j = JSONSlice{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// BeforeCreate 自动生成授权码
func (l *License) BeforeCreate(tx *gorm.DB) error {
	if l.LicenseKey == "" {
		l.LicenseKey = generateLicenseKey()
	}
	return nil
}

// IsValid 检查授权码是否有效（未过期、未禁用、未撤销）
func (l *License) IsValid() bool {
	if l.Status != LicenseStatusActive {
		return false
	}
	if l.IsPermanent {
		return true
	}
	now := time.Now()
	if now.Before(l.ValidFrom) || now.After(l.ValidTo) {
		return false
	}
	return true
}

// CanActivate 检查是否可以激活（未达最大激活数）
func (l *License) CanActivate() bool {
	return l.ActivatedCount < l.MaxActivations
}

// OfflineActivationRequest 客户端离线激活请求文件结构
type OfflineActivationRequest struct {
	Version           string `json:"version"`            // 协议版本
	LicenseKey        string `json:"license_key"`        // 授权码
	DeviceFingerprint string `json:"device_fingerprint"` // 设备指纹
	ClientVersion     string `json:"client_version"`     // 客户端版本
	Timestamp         int64  `json:"timestamp"`          // 请求时间戳
	Signature         string `json:"signature"`          // 请求签名（用客户端私钥或简单HMAC）
}

// OfflineActivationResponse 服务端离线激活响应文件结构
type OfflineActivationResponse struct {
	Version           string    `json:"version"`            // 协议版本
	LicenseKey        string    `json:"license_key"`        // 授权码
	DeviceFingerprint string    `json:"device_fingerprint"` // 设备指纹（与请求一致）
	ValidFrom         time.Time `json:"valid_from"`         // 有效期开始
	ValidTo           time.Time `json:"valid_to"`           // 有效期结束
	IssuedAt          time.Time `json:"issued_at"`          // 签发时间
	Signature         string    `json:"signature"`          // 服务端签名（RSA 私钥签名）
	Certificate       string    `json:"certificate"`        // 服务端公钥证书（PEM格式）
}

// 生成授权码（格式：XXXX-XXXX-XXXX-XXXX）
func generateLicenseKey() string {
	import_uuid := uuid.New().String()
	// 取 UUID 的前 16 位十六进制字符，分成 4 组
	clean := strings.ReplaceAll(strings.ToUpper(import_uuid), "-", "")
	if len(clean) < 16 {
		return "0000-0000-0000-0000"
	}
	return clean[0:4] + "-" + clean[4:8] + "-" + clean[8:12] + "-" + clean[12:16]
}

// ApplyRecord 客户自助授权申请记录
type ApplyRecord struct {
	ID             uint             `gorm:"primaryKey" json:"id"`
	ApplicantName  string           `gorm:"size:128;not null" json:"applicant_name"`
	Email          string           `gorm:"size:128;not null;index" json:"email"`
	Phone          string           `gorm:"size:32" json:"phone"`
	ProductKeys    models.JSONSlice `gorm:"type:text" json:"product_keys"` // 关联产品标识（多选）
	DurationMonths int              `json:"duration_months"`
	MaxActivations int              `json:"max_activations"`
	Description    string           `gorm:"type:text" json:"description"`
	Status         string           `gorm:"size:20;default:pending" json:"status"` // pending/approved/rejected
	AdminNote      string           `gorm:"type:text" json:"admin_note"`
	LicenseKey     string           `gorm:"size:64" json:"license_key"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// OperationLog 管理员操作日志
type OperationLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AdminID   uint      `gorm:"index" json:"admin_id"`
	AdminName string    `gorm:"size:64" json:"admin_name"`
	Action    string    `gorm:"size:32;index" json:"action"` // create/revoke/extend/activate/deactivate
	Target    string    `gorm:"size:64" json:"target"`       // license_key or apply_id
	Detail    string    `gorm:"type:text" json:"detail"`     // 操作详情
	IP        string    `gorm:"size:45" json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}
