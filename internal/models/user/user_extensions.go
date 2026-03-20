package user

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type BackgroundMedia []interface{}

type Style struct {
	BackgroundMediaList []BackgroundMedia `json:"backgroundMediaList"`
}

type Extensions struct {
	Style Style `json:"style"`
}

func (e Extensions) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Scan реализует интерфейс sql.Scanner для чтения Extensions из БД
func (e *Extensions) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, e)
}
