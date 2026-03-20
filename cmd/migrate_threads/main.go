package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/AugustLigh/GoMino/internal/models/chat"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// Подключение к базе данных
	db, err := gorm.Open(sqlite.Open("data/amino.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Получить все чаты
	var threads []chat.Thread
	if err := db.Find(&threads).Error; err != nil {
		log.Fatalf("Failed to fetch threads: %v", err)
	}

	log.Printf("Found %d threads to migrate", len(threads))

	// Обновить каждый чат
	defaultLanguage := "ru"
	defaultMembersCanInvite := true
	updatedCount := 0

	for i := range threads {
		thread := &threads[i]
		needsUpdate := false

		// Если extensions == nil, создать новый
		if thread.Extensions == nil {
			thread.Extensions = &chat.ThreadExtensions{
				Language:         &defaultLanguage,
				MembersCanInvite: &defaultMembersCanInvite,
				CoHost:           []string{},
			}
			needsUpdate = true
		} else {
			// Проверить и установить отсутствующие поля
			if thread.Extensions.Language == nil {
				thread.Extensions.Language = &defaultLanguage
				needsUpdate = true
			}
			if thread.Extensions.MembersCanInvite == nil {
				thread.Extensions.MembersCanInvite = &defaultMembersCanInvite
				needsUpdate = true
			}
			if thread.Extensions.CoHost == nil {
				thread.Extensions.CoHost = []string{}
				needsUpdate = true
			}
		}

		if needsUpdate {
			// Сериализовать extensions в JSON для отладки
			extJSON, _ := json.MarshalIndent(thread.Extensions, "", "  ")
			log.Printf("Updating thread %s with extensions:\n%s", thread.ThreadID, extJSON)

			// Обновить в базе данных используя Save для корректной сериализации JSON
			if err := db.Save(thread).Error; err != nil {
				log.Printf("Failed to update thread %s: %v", thread.ThreadID, err)
			} else {
				updatedCount++
			}
		}
	}

	log.Printf("Migration completed! Updated %d threads", updatedCount)
	fmt.Println("\nДля запуска миграции выполните:")
	fmt.Println("  go run cmd/migrate_threads/main.go")
}
