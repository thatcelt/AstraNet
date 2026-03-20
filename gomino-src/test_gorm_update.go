package main

import (
	"fmt"
	"log"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Открываем БД с логированием SQL запросов
	db, err := gorm.Open(sqlite.Open("data/amino.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Получаем пользователя с ndcId = 0 (глобальный)
	var u user.User
	err = db.First(&u, "uid = ? AND ndc_id = ?", "3ac4d056-a45e-46a6-8e1d-162c0d6cf19d", 0).Error
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Loaded user: ID=%d, UID=%s, NdcID=%d, Content=%s\n", u.ID, u.UID, u.NdcID, *u.Content)

	// Обновляем как в текущем коде
	updates := map[string]interface{}{
		"content": "TEST UPDATE",
	}

	fmt.Println("\n=== CURRENT APPROACH (Model(u).Updates) ===")
	// Это НЕ обновляет, только показываем что будет
	result := db.Model(&u).Updates(updates)
	fmt.Printf("Rows affected: %d\n", result.RowsAffected)

	// Проверяем что изменилось в БД
	var global, community user.User
	db.First(&global, "uid = ? AND ndc_id = ?", "3ac4d056-a45e-46a6-8e1d-162c0d6cf19d", 0)
	db.First(&community, "uid = ? AND ndc_id = ?", "3ac4d056-a45e-46a6-8e1d-162c0d6cf19d", 1650020869)

	fmt.Printf("Global (ndcId=0): Content=%s\n", *global.Content)
	fmt.Printf("Community (ndcId=1650020869): Content=%s\n", *community.Content)
}
