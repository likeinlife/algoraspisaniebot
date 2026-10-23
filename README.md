# Алгоритм-бот

Скаффолд Telegram-бота на Go: состояние хранится в SQLite, бот привязывается к одной группе, отвечает на `/chat_id` и по расписанию публикует открытый опрос о дне проведения алгоритмов.

## Запуск

1. Создайте бота через [@BotFather](https://t.me/BotFather) и скопируйте токен.
2. Скопируйте `.env.example` в `.env`, задайте `TELEGRAM_BOT_TOKEN` и при желании остальные параметры. Переменные окружения нужно экспортировать перед запуском (файл `.env` сам по себе не загружается).
3. Добавьте бота в нужную группу и запустите:

```sh
go run ./cmd/bot
```

4. Если `TELEGRAM_CHAT_ID` пустой, отправьте `/chat_id` в нужной группе: бот сохранит эту группу как единственную разрешённую. Если ID задан, бот принимает команды только из этой группы.

## Запуск на Linux через Docker Compose

Установите Docker Engine с плагином Docker Compose, скопируйте пример настроек и укажите токен бота:

```sh
cp .env.example .env
# Откройте .env и задайте TELEGRAM_BOT_TOKEN
docker compose up -d --build
```

Посмотреть логи:

```sh
docker compose logs -f bot
```

Если `TELEGRAM_CHAT_ID` не задан, добавьте бота в группу и отправьте `/chat_id`. База SQLite хранится в именованном Docker volume `bot-data` и переживает пересоздание контейнера. Остановить контейнер можно командой `docker compose down`.

## Запуск на Linux через systemd

Инструкция рассчитана на Linux с `systemd` и архитектуру amd64. На компьютере с Go соберите бинарный файл:

```sh
make build-linux
scp dist/algobot-linux-amd64 user@server:/tmp/algobot
```

Скопируйте бинарный файл на сервер в `/tmp/algobot`, затем на сервере создайте пользователя и каталоги:

```sh
sudo useradd --system --home-dir /var/lib/algobot --shell /usr/sbin/nologin algobot
sudo install -d -o root -g root -m 0755 /opt/algobot
sudo install -d -o algobot -g algobot -m 0750 /etc/algobot /var/lib/algobot
sudo install -o root -g root -m 0755 /tmp/algobot /opt/algobot/algobot
```

Создайте файл настроек `/etc/algobot/algobot.env` и укажите токен:

```sh
sudo install -o root -g algobot -m 0640 /dev/null /etc/algobot/algobot.env
sudoedit /etc/algobot/algobot.env
```

Содержимое файла — по одной переменной на строку, без `export`:

```dotenv
TELEGRAM_BOT_TOKEN=your-bot-token
# Необязательно: ID группы. Если не задан, отправьте боту /chat_id.
TELEGRAM_CHAT_ID=
POLL_SCHEDULER_ENABLED=true
POLL_CRON=0 12 * * 5
TIMEZONE=Europe/Moscow
```

Создайте `/etc/systemd/system/algobot.service`:

```ini
[Unit]
Description=Алгоритм-бот Telegram
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=algobot
Group=algobot
WorkingDirectory=/var/lib/algobot
EnvironmentFile=/etc/algobot/algobot.env
Environment=SQLITE_PATH=/var/lib/algobot/bot.db
ExecStart=/opt/algobot/algobot
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Загрузите unit и включите запуск бота:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now algobot
sudo systemctl status algobot
```

Посмотреть текущие логи:

```sh
sudo journalctl -u algobot -f
```

После замены бинарного файла перезапустите сервис. Полезные команды управления:

```sh
sudo systemctl restart algobot
sudo systemctl stop algobot
sudo systemctl start algobot
sudo systemctl disable algobot
```

Команда `/chat_id` возвращает ID текущей группы, `/poll_now` немедленно отправляет опрос. В опросе три варианта: «Суббота», «Воскресенье» и «Узнать результаты». Можно выбрать несколько вариантов и менять голос; результаты видны в самом Telegram-опросе. Бот не сохраняет ответы и результаты в SQLite. Расписание задаётся стандартным cron-выражением, по умолчанию — каждую пятницу в 12:00 по `Europe/Moscow`.

При запуске бот пишет в stderr, под каким Telegram-аккаунтом он авторизовался и доступна ли настроенная группа. Если `TELEGRAM_CHAT_ID` пустой, он пишет, что ждёт `/chat_id`; после команды в группе выводит её ID и подтверждает привязку. Если на `/chat_id` нет ответа, проверьте, что бот добавлен в группу и что в логе нет ошибок `getUpdates` (например, конфликта с другим запущенным экземпляром).

Чтобы отключить автоматические опросы, задайте `POLL_SCHEDULER_ENABLED=false`. Бот продолжит отвечать на команды и хранить ID привязанной группы.

## Разработка

Нужны Go и `make`; для lint установите [golangci-lint v2.13.2](https://golangci-lint.run/welcome/install/). Для обновления сгенерированного SQL-кода установите `sqlc` командой `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`.

```sh
make fmt        # применить gofmt
make fmt-check  # проверить форматирование
make generate   # сгенерировать Go-код из SQL-схемы и запросов через sqlc
make lint       # запустить golangci-lint
make check      # fmt-check и lint
make build-linux   # Linux amd64
make build-windows # Windows amd64
make build-macos   # macOS Intel и Apple Silicon
make build         # собрать все платформы
```

Артефакты сборки появятся в `dist/`. Сборка выполняется с `CGO_ENABLED=0`; SQLite драйвер на pure Go, поэтому кросс-компиляция не требует C-компилятора.

GitHub Actions запускает проверку gofmt и golangci-lint при push и pull request.

## Переменные окружения

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | — | Токен Telegram-бота (обязателен) |
| `TELEGRAM_CHAT_ID` | пусто | ID единственной группы; пустое значение включает привязку через `/chat_id` |
| `SQLITE_PATH` | `./data/bot.db` | Путь к файлу SQLite |
| `POLL_SCHEDULER_ENABLED` | `true` | Включить/выключить запланированные опросы |
| `POLL_CRON` | `0 12 * * 5` | Пятизначное cron-выражение |
| `TIMEZONE` | `Europe/Moscow` | Часовой пояс расписания |

## Структура

- `cmd/bot` — точка входа и конфигурация из окружения.
- `internal/bot` — обработка Telegram-команд, опросов и расписания.
- `internal/store` — SQLite-схема, SQL-запросы и операции с привязкой чата; `sqlc` генерирует код в `internal/store/sqlcgen`.
