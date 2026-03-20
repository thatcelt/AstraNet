# Инструкция по деплою Astranet на сервер

## Сервер
- IP: 144.31.166.46
- User: root
- Password: astranet
- URL: https://astranetapp.com

## Структура проекта

```
Локально:
- /home/avgust/Документы/new_projects/projects/GoMino/ - Backend (Go)
- /home/avgust/Документы/new_projects/projects/github/Nulla-Client/ - Frontend (Flutter)

На сервере (/root/astranet/):
- api - бинарник backend (собирается Docker'ом)
- web/ - статические файлы frontend
- data/ - база данных SQLite
- media_files/ - загруженные медиа файлы
- logs/ - логи приложения
- firebase-service-account.json - ключ Firebase
- docker-compose.yml - конфигурация Docker
- .env - переменные окружения
```

## 1. Деплой Backend (Go API)

### Подготовка

```bash
cd /home/avgust/Документы/new_projects/projects/GoMino
```

### Загрузка кода на сервер

```bash
# Загрузить go.mod и go.sum (если изменились зависимости)
sshpass -p 'astranet' scp go.mod go.sum root@144.31.166.46:/root/astranet/

# Загрузить Dockerfile.api (если изменился)
sshpass -p 'astranet' scp Dockerfile.api root@144.31.166.46:/root/astranet/

# Загрузить исходный код
sshpass -p 'astranet' scp -r cmd root@144.31.166.46:/root/astranet/
sshpass -p 'astranet' scp -r internal root@144.31.166.46:/root/astranet/
sshpass -p 'astranet' scp -r pkg root@144.31.166.46:/root/astranet/
```

### Пересборка и перезапуск

```bash
# Подключиться к серверу
sshpass -p 'astranet' ssh root@144.31.166.46

# Перейти в директорию проекта
cd /root/astranet

# Пересобрать Docker образ API
docker-compose build api

# Перезапустить все сервисы
docker-compose down && docker-compose up -d

# Проверить статус
docker ps

# Проверить логи
docker-compose logs -f api
```

## 2. Деплой Frontend (Flutter Web)

### Сборка WASM версии

```bash
cd /home/avgust/Документы/new_projects/projects/github/Nulla-Client

# Собрать Flutter Web с WASM для лучшей производительности
flutter build web --wasm --release
```

### Загрузка на сервер

```bash
# Создать backup текущей версии
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && mv web web_backup_\$(date +%Y%m%d_%H%M%S)"

# Загрузить новую версию
sshpass -p 'astranet' scp -r build/web root@144.31.166.46:/root/astranet/

# Готово! Новая версия сразу доступна на https://astranetapp.com
```

## 3. Полный деплой (Backend + Frontend)

```bash
# Backend
cd /home/avgust/Документы/new_projects/projects/GoMino
sshpass -p 'astranet' scp go.mod go.sum Dockerfile.api root@144.31.166.46:/root/astranet/
sshpass -p 'astranet' scp -r cmd internal pkg root@144.31.166.46:/root/astranet/

# Frontend
cd /home/avgust/Документы/new_projects/projects/github/Nulla-Client
flutter build web --wasm --release

# Deploy на сервер
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && mv web web_backup_\$(date +%Y%m%d_%H%M%S)"
sshpass -p 'astranet' scp -r build/web root@144.31.166.46:/root/astranet/

# Перезапуск backend
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose build api && docker-compose down && docker-compose up -d"
```

## 4. Проверка развертывания

```bash
# Проверить статус контейнеров
sshpass -p 'astranet' ssh root@144.31.166.46 "docker ps"

# Должны работать 3 контейнера:
# - astranet-api (UP)
# - astranet-media (UP)
# - astranet-caddy (UP)

# Проверить логи API
sshpass -p 'astranet' ssh root@144.31.166.46 "docker-compose -f /root/astranet/docker-compose.yml logs --tail=50 api"

# Должно быть сообщение:
# "Push Notification Service initialized successfully"

# Проверить веб-версию
curl -I https://astranetapp.com
# Должно быть: HTTP/2 200
```

## 5. Управление сервисами

### Просмотр логов

```bash
# Логи всех сервисов
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose logs -f"

# Логи только API
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose logs -f api"

# Логи только Caddy (веб-сервер)
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose logs -f caddy"
```

### Перезапуск сервисов

```bash
# Перезапустить все
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose restart"

# Перезапустить только API
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose restart api"
```

### Остановка/Запуск

```bash
# Остановить все
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose down"

# Запустить все
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose up -d"
```

## 6. База данных

### Backup базы данных

```bash
# Создать backup базы данных
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet/data && cp amino.db amino_backup_\$(date +%Y%m%d_%H%M%S).db"

# Скачать backup локально
sshpass -p 'astranet' scp root@144.31.166.46:/root/astranet/data/amino_backup_*.db ~/backups/
```

### Восстановление базы данных

```bash
# Загрузить backup на сервер
sshpass -p 'astranet' scp ~/backups/amino_backup_YYYYMMDD_HHMMSS.db root@144.31.166.46:/root/astranet/data/

# Восстановить
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose down && cp data/amino_backup_YYYYMMDD_HHMMSS.db data/amino.db && docker-compose up -d"
```

## 7. Обновление Firebase Service Account

```bash
# Если нужно обновить ключ Firebase
sshpass -p 'astranet' scp /path/to/new/firebase-service-account.json root@144.31.166.46:/root/astranet/

# Перезапустить API
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose restart api"
```

## 8. Мониторинг

### Использование ресурсов

```bash
# Проверить использование CPU/RAM
sshpass -p 'astranet' ssh root@144.31.166.46 "docker stats --no-stream"
```

### Размер логов

```bash
# Проверить размер логов
sshpass -p 'astranet' ssh root@144.31.166.46 "du -sh /root/astranet/logs/*"

# Очистить старые логи (если нужно)
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet/logs && rm error.log && docker-compose restart api"
```

## 9. Rollback (откат к предыдущей версии)

### Frontend

```bash
# Посмотреть список backups
sshpass -p 'astranet' ssh root@144.31.166.46 "ls -la /root/astranet/ | grep web_backup"

# Откатиться на предыдущую версию
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && rm -rf web && mv web_backup_YYYYMMDD_HHMMSS web"
```

### Backend

```bash
# Откатиться на предыдущий Docker образ
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose down && docker image ls astranet_api"

# Если нужно - пересобрать с предыдущей версией кода
```

## 10. Troubleshooting

### API не запускается

```bash
# Проверить логи
sshpass -p 'astranet' ssh root@144.31.166.46 "docker-compose -f /root/astranet/docker-compose.yml logs api"

# Проверить что Firebase файл существует
sshpass -p 'astranet' ssh root@144.31.166.46 "ls -la /root/astranet/firebase-service-account.json"

# Проверить .env файл
sshpass -p 'astranet' ssh root@144.31.166.46 "cat /root/astranet/.env | grep FIREBASE"
```

### Сайт не открывается

```bash
# Проверить статус Caddy
sshpass -p 'astranet' ssh root@144.31.166.46 "docker ps | grep caddy"

# Проверить логи Caddy
sshpass -p 'astranet' ssh root@144.31.166.46 "docker-compose -f /root/astranet/docker-compose.yml logs caddy"

# Проверить что файлы есть
sshpass -p 'astranet' ssh root@144.31.166.46 "ls -la /root/astranet/web/"
```

### Push-уведомления не работают

```bash
# Проверить что Firebase инициализирован
sshpass -p 'astranet' ssh root@144.31.166.46 "docker-compose -f /root/astranet/docker-compose.yml logs api | grep -i 'Push Notification Service initialized'"

# Проверить device tokens в БД
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && sqlite3 data/amino.db 'SELECT * FROM device_tokens;'"

# Проверить настройки пользователей
sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && sqlite3 data/amino.db 'SELECT uid, push_enabled FROM users WHERE ndc_id=0;'"
```

## 11. Быстрые команды (скопируй в терминал)

### Быстрый деплой backend:
```bash
cd /home/avgust/Документы/new_projects/projects/GoMino && sshpass -p 'astranet' scp -r cmd internal pkg root@144.31.166.46:/root/astranet/ && sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose build api && docker-compose up -d api"
```

### Быстрый деплой frontend:
```bash
cd /home/avgust/Документы/new_projects/projects/github/Nulla-Client && flutter build web --wasm --release && sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && mv web web_backup_\$(date +%Y%m%d_%H%M%S)" && sshpass -p 'astranet' scp -r build/web root@144.31.166.46:/root/astranet/
```

### Полный деплой (все сразу):
```bash
cd /home/avgust/Документы/new_projects/projects/GoMino && sshpass -p 'astranet' scp -r cmd internal pkg root@144.31.166.46:/root/astranet/ && cd /home/avgust/Документы/new_projects/projects/github/Nulla-Client && flutter build web --wasm --release && sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && mv web web_backup_\$(date +%Y%m%d_%H%M%S)" && sshpass -p 'astranet' scp -r build/web root@144.31.166.46:/root/astranet/ && sshpass -p 'astranet' ssh root@144.31.166.46 "cd /root/astranet && docker-compose build api && docker-compose down && docker-compose up -d"
```

## 12. Полезные ссылки

- **Frontend**: https://astranetapp.com
- **API**: https://api.astranetapp.com
- **Media**: https://media.astranetapp.com
- **Firebase Console**: https://console.firebase.google.com/project/astranet-afac1
- **Server SSH**: `sshpass -p 'astranet' ssh root@144.31.166.46`

## Готово! 🚀

Теперь у тебя есть полная инструкция для деплоя новых версий Astranet!
