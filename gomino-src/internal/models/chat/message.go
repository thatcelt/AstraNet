package chat

import (
	"time"

	"github.com/AugustLigh/GoMino/internal/models/user"
	"github.com/AugustLigh/GoMino/internal/models/utils"
	"gorm.io/gorm"
)

// Типы сообщений
const (
	MessageTypeText    = 0   // Текстовое сообщение
	MessageTypeImage   = 3   // Изображение
	MessageTypeSticker = 100 // Стикер

	// Системные сообщения
	MessageTypeUserJoined   = 101 // Пользователь вступил в чат
	MessageTypeUserLeft     = 102 // Пользователь покинул чат
	MessageTypeDeleted      = 103 // Сообщение было удалено
)

// Message - модель сообщения в чате
type Message struct {
	// Primary key (скрыт в JSON)
	ID uint `gorm:"primaryKey" json:"-"`

	// Основные поля
	MessageID string `gorm:"uniqueIndex;not null" json:"messageId"`
	ThreadID  string `gorm:"not null;index:idx_thread_time" json:"threadId"`
	Content   string `gorm:"type:text" json:"content"`
	Type      int    `gorm:"default:0" json:"type"`

	// Медиа
	MediaType  int     `gorm:"default:0" json:"mediaType"`
	MediaValue *string `gorm:"size:1000" json:"mediaValue,omitempty"`
	StickerID  *string `gorm:"size:100" json:"stickerId,omitempty"`

	// Метаданные
	ClientRefID       int     `gorm:"default:0" json:"clientRefId"`
	ReplyTo           *string `gorm:"size:100;index" json:"replyMessageId,omitempty"` // ID сообщения, на которое отвечают
	IsHidden          bool    `gorm:"default:false" json:"isHidden"`
	IncludedInSummary bool    `gorm:"default:false" json:"includedInSummary"`

	// Foreign keys
	UID string `gorm:"not null;index" json:"uid"`

	// Временные метки
	CreatedTime utils.CustomTime `gorm:"not null;index:idx_thread_time" json:"createdTime"`

	// JSON поля
	Extensions map[string]interface{} `gorm:"type:json;serializer:json" json:"extensions,omitempty"`

	// Связи
	Author       user.Author `gorm:"foreignKey:UID;references:UID" json:"author"`
	Thread       *Thread     `gorm:"foreignKey:ThreadID;references:ThreadID" json:"-"`
	ReplyMessage *Message    `gorm:"foreignKey:ReplyTo;references:MessageID" json:"replyMessage,omitempty"`

	// GORM служебные поля
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName - имя таблицы в БД
func (Message) TableName() string {
	return "messages"
}

// BeforeCreate - хук перед созданием записи
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.CreatedTime.IsZero() {
		m.CreatedTime = utils.CustomTime{Time: time.Now()}
	}
	return nil
}
