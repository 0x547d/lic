package models

import "time"

// Product 产品表
// 记录产品标识（英文唯一 id）和中文名称
type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ProductKey  string    `gorm:"uniqueIndex;size:64;not null" json:"product_key"` // 英文唯一标识，如 standard / pro / enterprise / trial
	Name        string    `gorm:"size:128;not null" json:"name"`                   // 中文名称，如 标准版 / 专业版
	Description string    `gorm:"type:text" json:"description"`                    // 产品描述（可选）
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
