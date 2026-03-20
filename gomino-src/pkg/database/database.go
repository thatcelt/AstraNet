// database/database.go
package database

import (
	"strings"
	"time"

	"github.com/AugustLigh/GoMino/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func InitDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := cfg.DSN

	// Добавляем PRAGMA для оптимизации производительности SQLite при высокой нагрузке
	// journal_mode=WAL - значительно улучшает параллелизм
	// busy_timeout - дает время на ожидание, если БД заблокирована
	// synchronous=NORMAL - ускоряет запись за счет менее строгих гарантий
	// cache_size=-64000 - 64MB кеша
	// temp_store=MEMORY - временные таблицы в памяти
	// mmap_size=30000000000 - использовать mmap для доступа к файлу
	pragmas := "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size=-64000&_pragma=temp_store=MEMORY&_pragma=mmap_size=30000000000"

	if strings.Contains(dsn, "?") {
		dsn += "&" + pragmas
	} else {
		dsn += "?" + pragmas
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Настройки пула соединений
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100) // Ограничиваем количество открытых соединений
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func CloseDatabase(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
