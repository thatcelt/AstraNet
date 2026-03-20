package service

import (
	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func generateUID() string {
	return uuid.New().String()
}

func isTeamAstranet(db *gorm.DB, uid string) bool {
	var count int64
	db.Model(&user.User{}).Where("uid = ? AND ndc_id = 0 AND is_astranet = ?", uid, true).Count(&count)
	return count > 0
}
