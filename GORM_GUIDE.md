# GORM Guide для GoMino

## Когда что использовать

### 1. AutoMigrate
```go
// В main.go или init функции
db.AutoMigrate(&user.User{}, &chat.Thread{}, &chat.Message{})
```

**Используй в разработке:**
- Быстрое прототипирование
- Локальная разработка
- Тесты

**НЕ используй в продакшене:**
- Не контролируешь изменения
- Можешь потерять данные
- Нет отката (rollback)

### 2. Ручные миграции (для продакшена)
```go
// migrations/001_create_users.sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    uid VARCHAR(255) UNIQUE NOT NULL,
    nickname VARCHAR(255) NOT NULL,
    ...
);
```

Используй библиотеки: `golang-migrate`, `goose`

---

## GORM теги - шпаргалка

### Основные теги:
```go
type User struct {
    ID        uint   `gorm:"primaryKey"`              // Первичный ключ
    Email     string `gorm:"uniqueIndex;not null"`    // Уникальный + NOT NULL
    Name      string `gorm:"size:255;default:'Guest'"` // Размер + дефолт
    Age       int    `gorm:"check:age >= 0"`          // Ограничение CHECK
    CreatedAt time.Time `gorm:"autoCreateTime"`       // Авто-время создания
}
```

### Связи:
```go
// Один-к-одному
type User struct {
    ProfileID uint
    Profile   Profile `gorm:"foreignKey:ProfileID"`
}

// Один-ко-многим
type User struct {
    Posts []Post `gorm:"foreignKey:UserID"`
}

// Многие-ко-многим
type User struct {
    Roles []Role `gorm:"many2many:user_roles"`
}
```

---

## Hooks (хуки)

### Когда вызываются:
```go
BeforeCreate  // Перед INSERT
AfterCreate   // После INSERT
BeforeUpdate  // Перед UPDATE
AfterUpdate   // После UPDATE
BeforeDelete  // Перед DELETE
AfterDelete   // После DELETE
```

### Пример использования:
```go
func (u *User) BeforeCreate(tx *gorm.DB) error {
    u.CreatedTime = time.Now()
    u.UID = generateUID()  // Генерация уникального ID
    return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
    u.ModifiedTime = time.Now()
    return nil
}
```

---

## Работа с Chat - практический пример

### 1. Модель с GORM тегами
```go
type Thread struct {
    ID         uint   `gorm:"primaryKey" json:"-"`
    ThreadID   string `gorm:"uniqueIndex;not null" json:"threadId"`
    Title      string `gorm:"size:255" json:"title"`
    Type       int    `gorm:"default:0" json:"type"`
    UID        string `gorm:"not null;index" json:"uid"`
    
    // Связи
    Author   User      `gorm:"foreignKey:UID" json:"author"`
    Messages []Message `gorm:"foreignKey:ThreadID;references:ThreadID" json:"-"`
}
```

### 2. Создание чата
```go
// В handler
func CreateChat(c *gin.Context) {
    thread := &chat.Thread{
        ThreadID: generateThreadID(),
        Title:    req.Title,
        UID:      currentUser.UID,
    }
    
    db.Create(thread)  // Автоматом вызовется BeforeCreate
}
```

### 3. Получение чата с сообщениями
```go
func GetChat(c *gin.Context) {
    var thread chat.Thread
    
    db.Preload("Author").           // Загрузить автора
       Preload("Messages").         // Загрузить сообщения
       First(&thread, "thread_id = ?", threadID)
       
    c.JSON(200, thread)
}
```

### 4. Добавление сообщения
```go
func SendMessage(c *gin.Context) {
    msg := &chat.Message{
        MessageID: generateMessageID(),
        ThreadID:  req.ThreadID,
        Content:   req.Content,
        UID:       currentUser.UID,
    }
    
    db.Create(msg)  // Авто-время через BeforeCreate
}
```

---

## JSON vs GORM теги

### Правило:
```go
type Thread struct {
    // `json` - для API ответов
    // `gorm` - для работы с БД
    
    ID       uint   `gorm:"primaryKey" json:"-"`  // Скрыть от JSON
    ThreadID string `gorm:"uniqueIndex" json:"threadId"` // Оба
    Title    string `json:"title,omitempty"`      // Только JSON (не в БД)
}
```

### Когда нужны оба:
- Поле хранится в БД И отдается в API

### Когда только `json`:
- Computed fields (вычисляемые поля)
- Вложенные структуры (не отдельная таблица)

### Когда только `gorm`:
- Служебные поля (ID, foreign keys)
- Поля, которые не нужны в API

---

## Типичные ошибки

### ❌ Неправильно:
```go
type Thread struct {
    ThreadID string `json:"threadId"`  // Нет GORM тегов!
    Messages []Message                 // Как связь работает?
}
```

### ✅ Правильно:
```go
type Thread struct {
    ID       uint      `gorm:"primaryKey" json:"-"`
    ThreadID string    `gorm:"uniqueIndex;not null" json:"threadId"`
    Messages []Message `gorm:"foreignKey:ThreadID" json:"messages,omitempty"`
}
```

---

## Миграция vs AutoMigrate - итог

| Критерий | AutoMigrate | Ручные миграции |
|----------|-------------|-----------------|
| Скорость | ✅ Быстро | ❌ Медленно |
| Контроль | ❌ Нет | ✅ Полный |
| Откат | ❌ Нет | ✅ Есть |
| Продакшн | ❌ Опасно | ✅ Безопасно |
| Разработка | ✅ Идеально | ❌ Избыточно |

**Вывод:** AutoMigrate для разработки, миграции для продакшена.
