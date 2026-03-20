# Управление Push-уведомлениями

## Функционал

Пользователи могут **включать и выключать push-уведомления** через настройки приложения.

### Как это работает:

1. **Глобальный флаг**: Каждый пользователь имеет поле `push_enabled` в таблице `users` (по умолчанию `true`)
2. **Проверка перед отправкой**: Сервер проверяет флаг перед отправкой push
3. **Удаление токенов**: При выключении все device tokens удаляются автоматически
4. **Повторная регистрация**: При включении токен регистрируется заново

---

## Client API

### Получить текущие настройки

**GET** `/api/v1/g/s/account/push-settings`

**Headers:**
```
NDCAUTH: sid=YOUR_SID
AUID: YOUR_UID
NDCDEVICEID: YOUR_DEVICE_ID
```

**Response:**
```json
{
  "api:statuscode": 200,
  "enabled": true
}
```

---

### Обновить настройки

**POST** `/api/v1/g/s/account/push-settings`

**Headers:**
```
NDCAUTH: sid=YOUR_SID
AUID: YOUR_UID
NDCDEVICEID: YOUR_DEVICE_ID
Content-Type: application/json
```

**Body:**
```json
{
  "enabled": false
}
```

**Response:**
```json
{
  "api:statuscode": 200,
  "api:message": "Push settings updated successfully",
  "enabled": false
}
```

---

## Flutter Integration

### Provider

Используется `PushSettingsProvider`:

```dart
// Get current state
final pushSettings = context.read<PushSettingsProvider>();
bool isEnabled = pushSettings.pushEnabled;

// Update settings
await pushSettings.setPushEnabled(false); // Disable
await pushSettings.setPushEnabled(true);  // Enable
```

### UI (Settings Screen)

Переключатель находится в **Settings → Notifications**:

```dart
Consumer<PushSettingsProvider>(
  builder: (context, pushSettings, _) {
    return Switch.adaptive(
      value: pushSettings.pushEnabled,
      onChanged: (value) async {
        final success = await pushSettings.setPushEnabled(value);
        // Show feedback to user
      },
    );
  },
)
```

---

## Backend Logic

### Проверка перед отправкой

В `SendPushToUser()` есть автоматическая проверка:

```go
func (s *PushNotificationService) SendPushToUser(userID string, data NotificationData) error {
    // Check if user has push notifications enabled
    var u user.User
    if err := s.db.Select("push_enabled").Where("uid = ? AND ndc_id = 0", userID).First(&u).Error; err != nil {
        return nil
    }

    // Skip if push notifications are disabled
    if !u.PushEnabled {
        log.Printf("Push notifications disabled for user %s, skipping", userID)
        return nil
    }

    // ... send push
}
```

### Автоудаление токенов

При `POST /account/push-settings` с `enabled: false`:

```go
// If push notifications are disabled, delete all device tokens
if !req.Enabled {
    deleteResult := db.Where("user_id = ?", uid).Delete(&models.DeviceToken{})
    log.Printf("Deleted %d device tokens for user %s", deleteResult.RowsAffected, uid)
}
```

---

## Workflow

### Выключение уведомлений

1. Пользователь переключает switch в Settings
2. Клиент отправляет `POST /account/push-settings` с `enabled: false`
3. Сервер:
   - Устанавливает `push_enabled = false` в таблице `users`
   - Удаляет все `device_tokens` пользователя
4. Клиент удаляет локальный FCM token
5. Push не отправляются (проверка в `SendPushToUser`)

### Включение уведомлений

1. Пользователь переключает switch в Settings
2. Клиент отправляет `POST /account/push-settings` с `enabled: true`
3. Сервер устанавливает `push_enabled = true`
4. Клиент:
   - Запрашивает разрешение на уведомления
   - Получает новый FCM token
   - Регистрирует token через `POST /device-token`
5. Push снова отправляются

---

## Тестирование

### 1. Проверка выключения

```bash
# Выключить уведомления
curl -X POST http://localhost:8080/api/v1/g/s/account/push-settings \
  -H "NDCAUTH: sid=YOUR_SID" \
  -H "AUID: YOUR_UID" \
  -H "NDCDEVICEID: YOUR_DEVICE_ID" \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# Проверить device tokens (должны быть удалены)
sqlite3 data/gomino.db "SELECT * FROM device_tokens WHERE user_id='YOUR_UID';"
# Результат: пусто

# Попытаться отправить уведомление (не должно прийти)
# ... create notification event ...
# В логах: "Push notifications disabled for user YOUR_UID, skipping"
```

### 2. Проверка включения

```bash
# Включить уведомления
curl -X POST http://localhost:8080/api/v1/g/s/account/push-settings \
  -H "NDCAUTH: sid=YOUR_SID" \
  -H "AUID: YOUR_UID" \
  -H "NDCDEVICEID: YOUR_DEVICE_ID" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'

# В клиенте будет запрос разрешения на уведомления
# После разрешения - автоматическая регистрация токена

# Проверить device tokens (должны появиться)
sqlite3 data/gomino.db "SELECT * FROM device_tokens WHERE user_id='YOUR_UID';"
# Результат: 1+ записей с токенами
```

---

## Troubleshooting

### Push не приходят после включения

1. Проверьте `push_enabled` в БД:
```sql
SELECT uid, push_enabled FROM users WHERE uid='YOUR_UID' AND ndc_id=0;
```

2. Проверьте device tokens:
```sql
SELECT * FROM device_tokens WHERE user_id='YOUR_UID';
```

3. Проверьте разрешения в приложении:
   - Android: Settings → Apps → Astranet → Notifications
   - iOS: Settings → Astranet → Notifications
   - Web: Chrome → Site Settings → Notifications

### Token не регистрируется после включения

1. Проверьте логи клиента:
```
FCM Token: ... (должен быть)
Device token registered successfully (должно быть)
```

2. Проверьте Firebase initialization:
```
Push Notification Service initialized
```

3. Перелогиньтесь в приложении

### Уведомления приходят после выключения

1. Проверьте `push_enabled` (должно быть `false`)
2. Проверьте что device tokens удалены
3. Проверьте логи сервера:
```
Push notifications disabled for user XXX, skipping
```

---

## Best Practices

1. **Уважайте выбор пользователя** - не спамьте просьбами включить push
2. **Объясняйте зачем нужны** - показывайте пользу от уведомлений
3. **Granular control** - в будущем можно добавить настройки по типам уведомлений
4. **Тестируйте регулярно** - проверяйте что выключение/включение работает
5. **Логируйте** - записывайте когда пользователь меняет настройки

---

## Future Enhancements

Возможные улучшения:

1. **Granular Settings**:
   - Отдельные настройки для чатов, подписчиков, постов и т.д.
   - Настройка DoNotDisturb по времени

2. **Per-Community Settings**:
   - Включить/выключить для конкретных сообществ

3. **Smart Notifications**:
   - Автоматическое отключение в часы сна
   - Priority notifications (только важные)

4. **Notification History**:
   - История всех push (отправлено/доставлено/открыто)

5. **Analytics**:
   - Tracking открытий уведомлений
   - A/B тестирование текстов уведомлений
