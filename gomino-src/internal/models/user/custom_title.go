package user

import (
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

type CustomTitle struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	UserID      string           `json:"userId" gorm:"index;not null"`
	Title       string           `json:"title"`
	Color       string           `json:"color"`
	CreatedTime utils.CustomTime `json:"createdTime"`
	CreatedAt   utils.CustomTime `json:"-"`
	UpdatedAt   utils.CustomTime `json:"-"`
	DeletedAt   gorm.DeletedAt   `json:"-" gorm:"index"`
}

func (CustomTitle) TableName() string {
	return "custom_titles"
}
