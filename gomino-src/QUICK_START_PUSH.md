# Быстрый старт: Push-уведомления

## 1. Firebase Console (5 минут)

1. Перейдите на https://console.firebase.google.com/
2. Используйте существующий проект **astranet-afac1** или создайте новый
3. Скачайте Service Account Key:
   - **Project Settings** → **Service Accounts**
   - **Generate New Private Key**
   - Сохраните как `firebase-service-account.json` в корень GoMino

## 2. Backend Setup (2 минуты)

```bash
cd /home/avgust/Документы/new_projects/projects/GoMino

# Добавьте в .env:
echo "FIREBASE_PUSH_ENABLED=true" >> .env
echo "FIREBASE_SERVICE_ACCOUNT_PATH=./firebase-service-account.json" >> .env

# Установите зависимости (если еще не установлены)
go mod tidy

# Запустите сервер
go run cmd/api/main.go
```

Вы должны увидеть:
```
Push Notification Service initialized successfully
```

## 3. Client Setup (уже готово!)

Flutter клиент уже настроен! Осталось только:

### Android (опционально)
```bash
cd /home/avgust/Документы/new_projects/projects/github/Nulla-Client

# Скачайте google-services.json из Firebase Console
# Поместите в android/app/google-services.json
```

### iOS (опционально)
```bash
# Скачайте GoogleService-Info.plist из Firebase Console
# Поместите в ios/Runner/GoogleService-Info.plist
```

### Web (уже настроено!)
Конфигурация уже в `web/firebase-config.js`

## 4. Тестирование (3 минуты)

### Способ 1: Через приложение

1. Запустите Flutter приложение:
```bash
cd /home/avgust/Документы/new_projects/projects/github/Nulla-Client
flutter run -d chrome  # для Web
# или
flutter run            # для Android/iOS
```

2. Залогиньтесь в приложении
3. Разрешите уведомления когда попросит
4. Откройте DevTools → Console
5. Найдите строку: `FCM Token: ...`
6. Токен зарегистрирован!

### Способ 2: Проверка в базе данных

```bash
cd /home/avgust/Документы/new_projects/projects/GoMino
sqlite3 data/gomino.db

SELECT * FROM device_tokens;
```

Вы должны увидеть зарегистрированные токены.

### Способ 3: Отправка тестового push

Используйте Firebase Console:
1. **Cloud Messaging** → **Send test message**
2. Вставьте FCM token из базы данных
3. Отправьте!

## 5. Проверка работы уведомлений

### Статические уведомления (колокольчик)

1. Создайте второй аккаунт
2. Подпишитесь на первого пользователя
3. У первого пользователя должно появиться уведомление в колокольчике
4. Если приложение открыто - увидите локальное уведомление
5. Если приложение закрыто - придёт push

### Chat уведомления

1. Создайте чат между двумя пользователями
2. Один пользователь выходит из чата (но остается в приложении)
3. Второй отправляет сообщение
4. Первый получает push-уведомление!

## Troubleshooting

### "Push notifications are disabled"

Проверьте `.env`:
```bash
FIREBASE_PUSH_ENABLED=true
FIREBASE_SERVICE_ACCOUNT_PATH=./firebase-service-account.json
```

### "Failed to initialize Firebase app"

Проверьте путь к `firebase-service-account.json`:
```bash
ls -la firebase-service-account.json
```

### "No device tokens found for user"

Пользователь не дал разрешение на уведомления или не залогинился.

### Push не приходят на Web

1. Проверьте что Service Worker зарегистрирован:
   - Chrome DevTools → Application → Service Workers
2. Проверьте разрешение:
   - Chrome → Настройки → Конфиденциальность → Уведомления
3. Проверьте консоль на ошибки

## Что дальше?

Полная документация: [PUSH_NOTIFICATIONS_SETUP.md](./PUSH_NOTIFICATIONS_SETUP.md)

Там вы найдете:
- Детальная настройка для всех платформ
- Кастомизация уведомлений
- Monitoring и отладка
- API документация
- Безопасность
- Дополнительные возможности
