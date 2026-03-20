package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Подключение к базе данных
	db, err := sql.Open("sqlite3", "data/amino.db")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// SQL для конвертации старого формата BM в новый
	// Старый: [{"type": 100, "url": "...", ...}]
	// Новый: [100, "...", null]
	query := `
		UPDATE threads
		SET extensions = json_set(
			extensions,
			'$.bm',
			CASE
				WHEN json_type(extensions, '$.bm') = 'array'
					AND json_type(extensions, '$.bm[0]') = 'object'
				THEN json_array(
					json_extract(extensions, '$.bm[0].type'),
					json_extract(extensions, '$.bm[0].url'),
					NULL
				)
				ELSE json_extract(extensions, '$.bm')
			END
		)
		WHERE json_type(extensions, '$.bm[0]') = 'object'
	`

	result, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to update threads: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Migration completed! Updated %d threads", rowsAffected)

	// Проверка результата
	var count int
	db.QueryRow("SELECT COUNT(*) FROM threads WHERE json_type(extensions, '$.bm[0]') = 'object'").Scan(&count)
	if count > 0 {
		log.Printf("WARNING: %d threads still have old BM format!", count)
	} else {
		log.Println("All threads successfully migrated to new BM format!")
	}
}
