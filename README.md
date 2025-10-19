# Чек-лист речевого развития (ТНР)

Веб-приложение для оценки речевого развития детей с темпово-ритмическим недоразвитием речи (ТНР) в среднем дошкольном возрасте.

## Описание

Приложение представляет собой чек-лист для специалистов (логопедов, педагогов) для оценки речевого развития детей. Система позволяет:

- Заполнять чек-лист с критериями речевого развития
- Сохранять результаты в базу данных
- Просматривать историю оценок
- Работать в веб-интерфейсе

## Функциональность

### Критерии оценки:
- Проявление интереса к речевому взаимодействию
- Отклик на обращение по имени
- Построение простых предложений
- Участие в диалоге
- Использование невербальных средств коммуникации
- Совместная игра с другими детьми
- Поддержание эмоционального контакта

### Возможные оценки:
- **Да** - критерий полностью соответствует
- **Частично** - критерий частично проявляется
- **Нет** - критерий не проявляется

## Технический стек

- **Backend**: Go (Golang)
- **Frontend**: HTML/CSS/JavaScript (vanilla)
- **База данных**: PostgreSQL
- **Веб-сервер**: Nginx
- **Контейнеризация**: Docker

## Структура проекта

```
check_list_tnr/
├── main.go                 # Go API сервер
├── checklist_tnr_v2.html   # HTML интерфейс
├── go.mod                  # Go модули
├── go.sum                  # Зависимости Go
├── Dockerfile              # Dockerfile для frontend
├── Dockerfile.backend      # Dockerfile для backend
├── docker-compose.yml      # Docker Compose конфигурация
├── nginx.conf             # Конфигурация Nginx
└── README.md              # Документация
```

## Установка и запуск

### Предварительные требования

- Docker
- Docker Compose

### Запуск приложения

1. **Клонирование репозитория:**
   ```bash
   git clone https://github.com/ioganvaise83/check_list_tnr.git
   cd check_list_tnr
   ```

2. **Настройка переменных окружения:**

   Создайте файл `.env` в корне проекта:
   ```env
   PG_DSN=postgres://bpmn_user:bpmn_password@postgres:5432/bpmn_db?sslmode=disable
   ```

3. **Запуск с помощью Docker Compose:**
   ```bash
   docker-compose up -d
   ```

4. **Проверка работоспособности:**

   Откройте браузер и перейдите по адресу: `http://localhost`

   API будет доступен по адресу: `http://localhost/api/checklist`

### Ручная установка (без Docker)

1. **Установка PostgreSQL:**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install postgresql postgresql-contrib

   # Создание базы данных
   sudo -u postgres createdb bpmn_db
   sudo -u postgres createuser bpmn_user
   sudo -u postgres psql -c "ALTER USER bpmn_user PASSWORD 'bpmn_password';"
   sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE bpmn_db TO bpmn_user;"
   ```

2. **Установка Go:**
   ```bash
   # Скачайте и установите Go с официального сайта
   # https://golang.org/dl/
   ```

3. **Установка зависимостей:**
   ```bash
   go mod download
   ```

4. **Запуск сервера:**
   ```bash
   export PG_DSN="postgres://bpmn_user:bpmn_password@localhost:5432/bpmn_db?sslmode=disable"
   go run main.go
   ```

5. **Настройка веб-сервера:**

   Скопируйте `checklist_tnr_v2.html` в корень веб-сервера и настройте проксирование запросов к API.

## API документация

### POST /api/checklist

Сохранение результатов чек-листа.

**Запрос:**
```json
{
  "childName": "Иванов Иван Иванович",
  "date": "2024-01-15",
  "specialist": "Петрова Анна Сергеевна",
  "createdAt": "2024-01-15T10:30:00Z",
  "answers": [
    {
      "key": "need_communication",
      "label": "Проявляет интерес к речевому взаимодействию",
      "value": "Да",
      "comment": "Активно инициирует общение"
    }
  ]
}
```

**Ответ:**
```json
{
  "id": 123
}
```

**Коды ответов:**
- `201` - Успешно сохранено
- `400` - Неверный запрос (невалидный JSON, отсутствуют ответы)
- `500` - Внутренняя ошибка сервера

## Структура базы данных

### Таблица `checklists`
```sql
CREATE TABLE checklists (
  id BIGSERIAL PRIMARY KEY,
  child_name TEXT,
  date_of_check DATE,
  specialist TEXT,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
```

### Таблица `answers`
```sql
CREATE TABLE answers (
  id BIGSERIAL PRIMARY KEY,
  checklist_id BIGINT NOT NULL REFERENCES checklists(id) ON DELETE CASCADE,
  key_name TEXT NOT NULL,
  label TEXT,
  value TEXT,
  comment TEXT
);
```

## Разработка

### Локальная разработка

1. **Запуск только базы данных:**
   ```bash
   docker-compose up -d postgres
   ```

2. **Запуск backend для разработки:**
   ```bash
   export PG_DSN="postgres://bpmn_user:bpmn_password@localhost:5432/bpmn_db?sslmode=disable"
   go run main.go
   ```

3. **Frontend разработка:**
   Откройте `checklist_tnr_v2.html` в браузере напрямую для тестирования интерфейса.

### Тестирование

Для тестирования API можно использовать curl:

```bash
curl -X POST http://localhost/api/checklist \
  -H "Content-Type: application/json" \
  -d '{
    "childName": "Тестовый Ребенок",
    "date": "2024-01-15",
    "specialist": "Тестовый Специалист",
    "answers": [
      {
        "key": "need_communication",
        "label": "Проявляет интерес к речевому взаимодействию",
        "value": "Да"
      }
    ]
  }'
```

## Безопасность

- Приложение использует PostgreSQL с аутентификацией
- Все запросы к API должны быть POST с правильным Content-Type
- Валидация входных данных на стороне сервера

## Производительность

- Настроены оптимальные параметры пула соединений с базой данных
- Используется graceful shutdown для корректного завершения работы
- Статические файлы обслуживаются через Nginx

## Лицензия

Этот проект является проприетарным программным обеспечением.

## Поддержка

Для получения поддержки или сообщений об ошибках обращайтесь к разработчикам проекта.

---

*На основе п. 32.3.3 Стандарта (Приказ ДОгМ №666)*
