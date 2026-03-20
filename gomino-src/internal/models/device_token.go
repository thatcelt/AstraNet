package models

import (
	"time"
)

// Platform types for device tokens
const (
	PlatformAndroid = "android"
	PlatformIOS     = "ios"
	PlatformWeb     = "web"
)

// DeviceToken represents a Firebase Cloud Messaging token for push notifications
type DeviceToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"type:varchar(255);not null;index" json:"userId"`
	Token     string    `gorm:"type:text;not null;uniqueIndex" json:"token"`
	Platform  string    `gorm:"type:varchar(50);not null" json:"platform"` // android, ios, web
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name for DeviceToken
func (DeviceToken) TableName() string {
	return "device_tokens"
}
