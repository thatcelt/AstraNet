# Настройка Push-уведомлений через Firebase

Это руководство объясняет как настроить push-уведомления для вашего проекта.

## Требования

1. Проект Firebase (console.firebase.google.com)
2. Service Account JSON для backend
3. Настроенные приложения (Android, iOS, Web) в Firebase

## Настройка Backend (GoMino)

### 1. Получение Service Account Key

1. Откройте [Firebase Console](https://console.firebase.google.com/)
2. Выберите свой проект
3. Перейдите в **Project Settings** → **Service Accounts**
4. Нажмите **Generate New Private Key**
5. Сохраните JSON файл (например, `firebase-service-account.json`)
6. Поместите файл в корень проекта GoMino

### 2. Настройка .env

Добавьте в `.env` файл:

```env
# Firebase Push Notifications
FIREBASE_PUSH_ENABLED=true
FIREBASE_SERVICE_ACCOUNT_PATH=./firebase-service-account.json
```

### 3. Запуск сервера

После настройки запустите сервер. В логах вы должны увидеть:

```
Push Notification Service initialized successfully
```

Если Firebase отключен, вы увидите:

```
Push notifications are disabled (check FIREBASE_PUSH_ENABLED and FIREBASE_SERVICE_ACCOUNT_PATH in .env)
```

## Настройка Client (Flutter/Nulla-Client)

### Android

1. Скачайте `google-services.json` из Firebase Console
2. Поместите в `android/app/google-services.json`
3. Убедитесь что в `android/build.gradle` есть:
```gradle
classpath 'com.google.gms:google-services:4.3.15'
```
4. В `android/app/build.gradle` добавьте:
```gradle
apply plugin: 'com.google.gms.google-services'
```

### iOS

1. Скачайте `GoogleService-Info.plist` из Firebase Console
2. Поместите в `ios/Runner/GoogleService-Info.plist`
3. Добавьте в Xcode проект через Runner → Add Files to "Runner"

### Web (PWA)

Уже настроено! Файлы созданы:
- `web/firebase-config.js` - конфигурация
- `web/firebase-messaging-sw.js` - service worker
- `web/index.html` - обновлён с Firebase SDK

## Как это работает

### 1. Регистрация device token

При логине клиент:
1. Запрашивает разрешение на уведомления
2. Получает FCM token от Firebase
3. Отправляет token на сервер: `POST /api/v1/g/s/device-token`

### 2. Отправка уведомлений

Когда происходит событие (новое сообщение, новый подписчик и т.д.):

1. **Создаётся уведомление** в базе данных
2. **Отправляется WebSocket** всем онлайн пользователям
3. **Отправляется Push** через Firebase для offline пользователей

### 3. Типы уведомлений

#### Статические уведомления (колокольчик)
- Новый пост от подписчика
- Новый подписчик
- Новый чат
- Начало live room

#### Push-уведомления для чатов
- Отправляются только **offline** пользователям
- Проверяется presence (находится ли пользователь в чате)
- Учитывается настройка DoNotDisturb

## Тестирование

### Проверка регистрации токена

```bash
# После логина в клиенте
curl -X GET http://localhost:8080/api/v1/admin/device-tokens \
  -H "NDCAUTH: sid=YOUR_SID"
```

### Ручная отправка push

Используйте Firebase Console:
1. **Cloud Messaging** → **Send test message**
2. Введите FCM token (можно взять из базы данных)
3. Отправьте тестовое сообщение

### Проверка в приложении

1. Залогиньтесь в приложении
2. Разрешите уведомления
3. Выйдите из чата (но оставайтесь в приложении)
4. Попросите кого-то отправить сообщение
5. Должно прийти push-уведомление

## Отладка

### Backend

Логи находятся в консоли. Ищите:
```
Push Notification Service initialized successfully
Successfully sent push notification: ...
Failed to send push notification: ...
```

### Client

В Flutter DevTools → Logging смотрите:
```
FCM Token: ...
Device token registered successfully: android/ios/web
Notification permission granted: true/false
```

### Web (PWA)

В Chrome DevTools:
1. **Application** → **Service Workers** - проверьте что `firebase-messaging-sw.js` зарегистрирован
2. **Console** - смотрите логи Firebase
3. **Application** → **Notifications** - история уведомлений

## Troubleshooting

### Push не приходят на Android

1. Проверьте `google-services.json`
2. Убедитесь что `applicationId` в `build.gradle` совпадает с пакетом в Firebase
3. Проверьте разрешения в AndroidManifest.xml

### Push не приходят на iOS

1. Проверьте `GoogleService-Info.plist`
2. Убедитесь что Bundle ID совпадает с Firebase
3. Настройте APNs key в Firebase Console
4. Проверьте capabilities в Xcode: Push Notifications

### Push не приходят на Web

1. Откройте DevTools → Application → Service Workers
2. Убедитесь что service worker активен
3. Проверьте что разрешение на уведомления дано
4. Проверьте `gcm_sender_id` в `manifest.json`

### Token регистрируется но push не приходят

1. Проверьте что Firebase service account key валиден
2. Проверьте логи сервера на ошибки
3. Убедитесь что `FIREBASE_PUSH_ENABLED=true`
4. Проверьте что токен не expired (Firebase автоматически обновляет)

## Database Schema

### device_tokens table

```sql
CREATE TABLE device_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id VARCHAR(255) NOT NULL,
    token TEXT NOT NULL UNIQUE,
    platform VARCHAR(50) NOT NULL, -- android, ios, web
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX idx_device_tokens_user_id ON device_tokens(user_id);
CREATE UNIQUE INDEX idx_device_tokens_token ON device_tokens(token);
```

## API Endpoints

### POST /api/v1/g/s/device-token
Регистрация/обновление device token

**Request:**
```json
{
  "token": "FCM_TOKEN_HERE",
  "platform": "android" // or "ios", "web"
}
```

**Response:**
```json
{
  "api:statuscode": 200,
  "api:message": "Device token registered successfully",
  "deviceToken": {
    "id": 1,
    "userId": "user-123",
    "token": "FCM_TOKEN_HERE",
    "platform": "android",
    "createdAt": "2025-02-01T12:00:00Z",
    "updatedAt": "2025-02-01T12:00:00Z"
  }
}
```

### DELETE /api/v1/g/s/device-token
Удаление всех токенов пользователя (при logout)

**Response:**
```json
{
  "api:statuscode": 200,
  "api:message": "Device tokens deleted successfully",
  "deletedCount": 2
}
```

## Безопасность

1. **Service Account Key** - храните в безопасном месте, НЕ коммитьте в git
2. **Device Tokens** - хранятся в БД, связаны с user_id
3. **Валидация** - сервер проверяет что пользователь авторизован перед регистрацией токена
4. **Auto-cleanup** - невалидные токены автоматически удаляются при ошибках отправки

## Monitoring

Рекомендуется отслеживать:
- Количество зарегистрированных токенов
- Успешность отправки push (success rate)
- Невалидные токены (auto-deleted)
- Ошибки Firebase API

## Дополнительные возможности

### Кастомизация уведомлений

В `internal/service/push/push_notification.go` можно настроить:
- Иконки для разных платформ
- Звуки
- Badge count
- Priority
- Click action

### Группировка уведомлений

Можно добавить в notification data:
```go
Android: &messaging.AndroidConfig{
    CollapseKey: threadID, // Группировать по чату
}
```

### Rich notifications

Поддерживаются:
- Изображения (ImageURL)
- Действия (Actions)
- Кастомные данные (Data)
