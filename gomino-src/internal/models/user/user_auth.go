package user

import (
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

type UserAuth struct {
	ID            uint   `json:"-" gorm:"primaryKey"`
	UserID        string `json:"-" gorm:"uniqueIndex;not null"` // Связь с User через UID
	Email         string `json:"-" gorm:"uniqueIndex;not null"`
	PasswordHash  string `json:"-" gorm:"not null"`
	ContentRegion string `json:"-" gorm:"default:en"` // Private content region preference (ru, en, es, ar)

	// GORM служебные поля
	CreatedAt utils.CustomTime `json:"-"`
	UpdatedAt utils.CustomTime `json:"-"`
	DeletedAt gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (UserAuth) TableName() string {
	return "user_auth"
}
