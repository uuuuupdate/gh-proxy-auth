package models

import (
	"time"

	"gorm.io/gorm"
)

type Passkey struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	UserID          uint           `gorm:"index;not null" json:"user_id"`
	Name            string         `gorm:"size:128" json:"name"`
	CredentialID    string         `gorm:"size:512;not null" json:"credential_id"`
	PublicKey       []byte         `gorm:"type:blob" json:"-"`
	AttestationType string         `gorm:"size:64" json:"-"`
	AAGUID          string         `gorm:"size:128" json:"-"`
	SignCount       uint32         `json:"-"`
	Transport       string         `gorm:"size:256" json:"-"`
	BackupEligible  bool           `gorm:"default:false" json:"-"`
	BackupState     bool           `gorm:"default:false" json:"-"`
}
