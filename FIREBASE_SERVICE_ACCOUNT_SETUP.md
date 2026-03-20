# Настройка Firebase Service Account для Push-уведомлений

## Шаг 1: Получение Service Account Key из Firebase Console

1. Откройте [Firebase Console](https://console.firebase.google.com/)
2. Выберите ваш проект **astranet-afac1**
3. Нажмите на иконку шестеренки (⚙️) рядом с "Project Overview" → **Project settings**
4. Перейдите на вкладку **Service accounts**
5. Нажмите кнопку **Generate new private key**
6. Подтвердите действие нажав **Generate key**
7. Файл `astranet-afac1-firebase-adminsdk-xxxxx-xxxxxxxxxx.json` скачается автоматически

## Шаг 2: Переименование и загрузка на сервер

### Локально:

```bash
# Переименуйте скачанный файл
mv ~/Downloads/astranet-afac1-firebase-adminsdk-*.json firebase-service-account.json

# Загрузите файл на сервер
sshpass -p 'astranet' scp firebase-service-account.json root@144.31.166.46:/root/astranet/
```

## Шаг 3: Перезапуск сервисов

```bash
# Подключитесь к серверу
sshpass -p 'astranet' ssh root@144.31.166.46

# Перейдите в директорию проекта
cd /root/astranet

# Перезапустите контейнеры
docker-compose down
docker-compose up -d

# Проверьте логи (должно быть сообщение "Push Notification Service initialized successfully")
docker-compose logs -f api | grep -i "push\|firebase"
```

## Проверка работы

После перезапуска в логах должно появиться:

```
Push Notification Service initialized successfully
```

Если видите это сообщение - push-уведомления работают! ✅

## Если что-то пошло не так

### Ошибка: "Failed to initialize Firebase Admin SDK"

- Проверьте что файл `firebase-service-account.json` существует в `/root/astranet/`
- Проверьте права доступа: `chmod 644 /root/astranet/firebase-service-account.json`
- Проверьте что файл корректный JSON: `cat /root/astranet/firebase-service-account.json | jq`

### Ошибка: "permission denied"

```bash
chmod 644 /root/astranet/firebase-service-account.json
```

### Push не приходят

1. Проверьте что `FIREBASE_PUSH_ENABLED=true` в `.env`
2. Проверьте логи: `docker-compose logs -f api`
3. Убедитесь что device token зарегистрирован:
   ```bash
   sqlite3 /root/astranet/data/amino.db "SELECT * FROM device_tokens;"
   ```
4. Проверьте что пользователь включил push в настройках:
   ```bash
   sqlite3 /root/astranet/data/amino.db "SELECT uid, push_enabled FROM users WHERE ndc_id=0;"
   ```

## Структура файлов на сервере

После настройки должно быть:

```
/root/astranet/
├── firebase-service-account.json  ← Этот файл нужно добавить
├── docker-compose.yml             ✓ Уже обновлен
├── .env                           ✓ Уже обновлен
├── api/                           ✓ Уже обновлен
├── web/                           ✓ Уже обновлен (WASM build)
└── ...
```

## Важно! 🔒

**Файл `firebase-service-account.json` содержит приватный ключ!**

- ❌ НЕ коммитьте его в Git
- ❌ НЕ публикуйте его публично
- ✅ Храните только на сервере
- ✅ Права доступа: `644` (rw-r--r--)

## Тестирование push-уведомлений

После настройки:

1. Войдите в приложение на https://astranetapp.com
2. Разрешите push-уведомления в браузере
3. Откройте DevTools → Console, проверьте что:
   - `FCM Token: ...` появился
   - `Device token registered successfully` появилось
4. Попросите кого-то отправить вам сообщение
5. Push-уведомление должно прийти! 🎉

## Готово! ✅

После выполнения всех шагов push-уведомления будут работать для:
- 💬 Новых сообщений в чатах
- 👤 Новых подписчиков
- ❤️ Лайков на постах
- 💭 Новых комментариев
- 📢 И других событий в приложении
