package utils

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type MediaItem struct {
	Type   int         `json:"type"`
	URL    string      `json:"url"`
	Extra1 interface{} `json:"extra1"`
	Code   string      `json:"code"`
	Extra2 interface{} `json:"extra2,omitempty"`
	Meta   interface{} `json:"meta,omitempty"`
}

// MediaList - тип для слайса MediaItem с поддержкой GORM
type MediaList []MediaItem

// Value реализует интерфейс driver.Valuer для записи в БД
func (m MediaList) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan реализует интерфейс sql.Scanner для чтения из БД
func (m *MediaList) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for MediaList")
	}

	return json.Unmarshal(bytes, m)
}

func (m MediaItem) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{m.Type, m.URL, m.Extra1})
}

func (m *MediaItem) UnmarshalJSON(data []byte) error {
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		if len(arr) >= 1 {
			if t, ok := arr[0].(float64); ok {
				m.Type = int(t)
			}
		}
		if len(arr) >= 2 {
			if u, ok := arr[1].(string); ok {
				m.URL = u
			}
		}
		if len(arr) >= 3 {
			m.Extra1 = arr[2]
		}
		return nil
	}

	// Fallback to object if it's not an array
	type Alias MediaItem
	return json.Unmarshal(data, (*Alias)(m))
}
