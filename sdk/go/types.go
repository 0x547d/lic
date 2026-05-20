package lic_sdk

import "time"

// OfflineAuthFile 离线授权文件结构
type OfflineAuthFile struct {
	Version        string    `json:"version"`         // 协议版本
	LicenseKey     string    `json:"license_key"`     // 授权码
	Company        string    `json:"company"`         // 公司/个人名称
	ProductKeys    []string  `json:"product_keys"`    // 授权产品列表
	ValidFrom      time.Time `json:"valid_from"`      // 授权有效期开始
	ValidTo        time.Time `json:"valid_to"`        // 授权有效期结束
	ActivatedCount int       `json:"activated_count"` // 已激活设备数
	MaxActivations int       `json:"max_activations"` // 最大激活数
	IssuedAt       time.Time `json:"issued_at"`       // 文件签发时间
	Signature      string    `json:"signature"`       // 服务端 RSA 签名
	Certificate    string    `json:"certificate"`     // 服务端公钥证书（PEM格式）
}

// VerifyResult 验证结果
type VerifyResult struct {
	Valid   bool             `json:"valid"`   // 是否有效
	Reason  string           `json:"reason"`  // 无效原因
	License *OfflineAuthFile `json:"license"` // 授权信息
}
