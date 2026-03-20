# Примеры использования Chat API

## 1. Инициализация в main.go

```go
package main

import (
    "log"
    
    "github.com/AugustLigh/GoMino/internal/api/chat"
    "github.com/AugustLigh/GoMino/internal/models/chat"
    "github.com/AugustLigh/GoMino/internal/models/user"
    "github.com/AugustLigh/GoMino/internal/service"
    "github.com/AugustLigh/GoMino/pkg/database"
    "github.com/gofiber/fiber/v3"
)

func main() {
    // Подключение к БД
    db, err := database.Connect()
    if err != nil {
        log.Fatal(err)
    }
    
    // AutoMigrate для разработки
    db.AutoMigrate(
        &user.User{},
        &chat.Thread{},
        &chat.Message{},
    )
    
    // Создание сервисов
    chatService := service.NewChatService(db)
    
    // Создание handlers
    chatHandler := chat.NewHandler(chatService)
    
    // Создание Fiber app
    app := fiber.New()
    
    // Регистрация маршрутов
    api := app.Group("/api/v1/g/s")
    
    // Chat routes
    chatRoutes := api.Group("/chat")
    {
        chatRoutes.Post("/thread", chatHandler.CreateThread)
        chatRoutes.Get("/thread", chatHandler.ListMyThreads)
        chatRoutes.Get("/thread/:threadId", chatHandler.GetThreadInfo)
        chatRoutes.Get("/thread/:threadId/message", chatHandler.GetMessages)
        chatRoutes.Post("/thread/:threadId/message", chatHandler.SendMessage)
        chatRoutes.Delete("/thread/:threadId/message/:messageId", chatHandler.DeleteMessage)
        chatRoutes.Get("/thread/:threadId/member", chatHandler.GetMembers)
        chatRoutes.Post("/thread/:threadId/member/:userId", chatHandler.JoinThread)
    }
    
    app.Listen(":8080")
}
```

## 2. Создание чата

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/g/s/chat/thread \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Мой первый чат",
    "content": "Добро пожаловать!",
    "type": 0
  }'
```

**Response:**
```json
{
  "api:statuscode": 0,
  "api:message": "OK",
  "api:timestamp": "2025-12-18T10:00:00Z",
  "thread": {
    "threadId": "550e8400-e29b-41d4-a716-446655440000",
    "title": "Мой первый чат",
    "content": "Добро пожаловать!",
    "type": 0,
    "membersCount": 1,
    "createdTime": "2025-12-18T10:00:00Z",
    "author": {
      "uid": "user123",
      "nickname": "TestUser"
    }
  }
}
```

## 3. Отправка сообщения

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/g/s/chat/thread/550e8400-e29b-41d4-a716-446655440000/message \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Привет всем!",
    "type": 0
  }'
```

**Response:**
```json
{
  "api:statuscode": 0,
  "message": {
    "messageId": "msg-123",
    "threadId": "550e8400-e29b-41d4-a716-446655440000",
    "content": "Привет всем!",
    "type": 0,
    "createdTime": "2025-12-18T10:05:00Z",
    "author": {
      "uid": "user123",
      "nickname": "TestUser"
    }
  }
}
```

## 4. Получение сообщений

**Request:**
```bash
curl -X GET "http://localhost:8080/api/v1/g/s/chat/thread/550e8400-e29b-41d4-a716-446655440000/message?size=25&pageToken=0"
```

**Response:**
```json
{
  "api:statuscode": 0,
  "messageList": [
    {
      "messageId": "msg-123",
      "content": "Привет всем!",
      "createdTime": "2025-12-18T10:05:00Z",
      "author": {
        "nickname": "TestUser"
      }
    }
  ],
  "paging": {
    "nextPageToken": "1",
    "total": 1
  }
}
```

## 5. Список моих чатов

**Request:**
```bash
curl -X GET "http://localhost:8080/api/v1/g/s/chat/thread?size=25&pageToken=0"
```

## 6. Программное использование в коде

### Создание чата из другого handler'а:

```go
package myhandler

import (
    "github.com/AugustLigh/GoMino/internal/service"
)

func SomeHandler(chatService *service.ChatService) {
    // Создать чат
    thread, err := chatService.CreateThread(
        "user123",           // UID автора
        "Название чата",     // Заголовок
        "Описание",          // Содержимое
        0,                   // Тип чата
    )
    
    if err != nil {
        // Обработка ошибки
    }
    
    // Отправить сообщение
    message, err := chatService.SendMessage(
        thread.ThreadID,     // ID чата
        "user123",           // UID автора
        "Привет!",           // Текст
        0,                   // Тип сообщения
    )
}
```

### Получение чата с сообщениями:

```go
// С сообщениями
thread, err := chatService.GetThread(threadID, true)

// Без сообщений (быстрее)
thread, err := chatService.GetThread(threadID, false)

// Доступ к данным
fmt.Println(thread.Title)
fmt.Println(thread.Author.Nickname)
if thread.LastMessageSummary != nil {
    fmt.Println(thread.LastMessageSummary.Content)
}
```

### Пагинация сообщений:

```go
// 1. Обычная пагинация (Offset-based)
	messages, err := chatService.GetMessages(threadID, service.GetMessagesOptions{
		Size:  25,
		Start: 0,
	})

	// 2. Курсорная пагинация (Time-based, infinite scroll вверх)
	// Получить сообщения старее чем 2025-12-18 10:00:00
	beforeTime := time.Date(2025, 12, 18, 10, 0, 0, 0, time.UTC)
	messages, err = chatService.GetMessages(threadID, service.GetMessagesOptions{
		Size:       25,
		BeforeTime: &beforeTime,
	})

	// 3. Прыжок к сообщению (Context-based)
	// Получить сообщение msg-123 и контекст вокруг него
	targetMsgID := "msg-123"
	messages, err = chatService.GetMessages(threadID, service.GetMessagesOptions{
		Size:            25,
		AroundMessageID: &targetMsgID,
	})
```

## 7. WebSocket для real-time чата (будущее расширение)

```go
// В файле internal/ws/chat.go

type ChatHub struct {
    clients    map[string]map[*Client]bool  // threadID -> clients
    broadcast  chan *BroadcastMessage
    register   chan *ClientRegistration
    unregister chan *ClientRegistration
}

func (h *ChatHub) Run() {
    for {
        select {
        case reg := <-h.register:
            // Подключить клиента к чату
        case unreg := <-h.unregister:
            // Отключить клиента
        case msg := <-h.broadcast:
            // Отправить всем в чате
        }
    }
}
```

## 8. Тесты

```go
package service_test

import (
    "testing"
    
    "github.com/AugustLigh/GoMino/internal/service"
    "github.com/stretchr/testify/assert"
)

func TestCreateThread(t *testing.T) {
    db := setupTestDB()
    chatService := service.NewChatService(db)
    
    thread, err := chatService.CreateThread(
        "test-user",
        "Test Chat",
        "Test Content",
        0,
    )
    
    assert.NoError(t, err)
    assert.NotEmpty(t, thread.ThreadID)
    assert.Equal(t, "Test Chat", *thread.Title)
}

func TestSendMessage(t *testing.T) {
    db := setupTestDB()
    chatService := service.NewChatService(db)
    
    // Создать чат
    thread, _ := chatService.CreateThread("user1", "Chat", "Content", 0)
    
    // Отправить сообщение
    msg, err := chatService.SendMessage(
        thread.ThreadID,
        "user1",
        "Hello",
        0,
    )
    
    assert.NoError(t, err)
    assert.Equal(t, "Hello", msg.Content)
}
```

## 9. Полезные GORM операции

### Обновить количество участников:
```go
db.Model(&chat.Thread{}).
    Where("thread_id = ?", threadID).
    Update("members_count", gorm.Expr("members_count + ?", 1))
```

### Получить чаты с количеством сообщений:
```go
type ThreadWithCount struct {
    chat.Thread
    MessageCount int
}

var results []ThreadWithCount
db.Model(&chat.Thread{}).
    Select("threads.*, COUNT(messages.id) as message_count").
    Joins("LEFT JOIN messages ON messages.thread_id = threads.thread_id").
    Group("threads.id").
    Find(&results)
```

### Поиск чатов по названию:
```go
var threads []chat.Thread
db.Where("title LIKE ?", "%"+search+"%").
    Preload("Author").
    Order("latest_activity_time DESC").
    Find(&threads)
```

## Итог

Теперь у тебя:
- ✅ Модели с правильными GORM тегами
- ✅ Сервис с бизнес-логикой
- ✅ Handlers для API
- ✅ Примеры использования

**Следующие шаги:**
1. Добавить middleware для авторизации (установка `userUID` в `c.Locals`)
2. Добавить WebSocket для real-time
3. Добавить участников чата (таблица `thread_members`)
4. Добавить типы сообщений (текст, изображение, стикер)
