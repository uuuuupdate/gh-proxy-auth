package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Username    string         `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Password    string         `gorm:"size:255;not null" json:"-"`
	IsAdmin     bool           `gorm:"default:false" json:"is_admin"`
	TOTPSecret  string         `gorm:"size:255" json:"-"`
	TOTPEnabled bool           `gorm:"default:false" json:"totp_enabled"`
	MFAPriority string         `gorm:"size:20;default:passkey" json:"mfa_priority"` // passkey or totp
	SpeedLimit  int64          `gorm:"default:0" json:"speed_limit"`                // bytes/sec, 0 = unlimited
}
