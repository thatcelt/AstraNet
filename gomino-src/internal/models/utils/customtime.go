package utils

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type CustomTime struct {
	time.Time
}

func (ct CustomTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(ct.Time.UTC().Format("2006-01-02T15:04:05Z"))
}

func (ct *CustomTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", s)
	if err != nil {
		return err
	}
	ct.Time = t
	return nil
}

func (ct CustomTime) Value() (driver.Value, error) {
	return ct.Time, nil
}

func (ct *CustomTime) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	if t, ok := value.(time.Time); ok {
		*ct = CustomTime{Time: t}
		return nil
	}
	return errors.New("cannot scan into CustomTime")
}
