# CRUD-Gen — генератор CRUD кода для Go

**CRUD-Gen** — это инструмент командной строки, который автоматически генерирует типобезопасный код для работы с базами данных на основе структур Go. Поддерживает PostgreSQL, MySQL и SQLite, позволяет использовать кастомные шаблоны и работает как через CLI, так и через HTTP API.

## Возможности

- **AST парсинг** — анализирует структуры Go на уровне синтаксического дерева
- **Автоматическая генерация CRUD** — Create, Read (Get), Update, Delete, List операции
- **Поддержка 3 СУБД** — PostgreSQL, MySQL, SQLite с автоматической адаптацией синтаксиса SQL
- **Кастомные шаблоны** — переопределите генерацию под ваши нужды
- **CLI и HTTP API** — используйте как команду или как микросервис
- **Типобезопасность** — параметризованные запросы, защита от SQL injection
- **Автогенерация тестов** — базовый набор unit тестов с флагом `--with-tests`

## Быстрый старт

### Установка

```bash
# Клонируем репозиторий
git clone https://github.com/Arseniy-Shamelov/crud-gen
cd crud-gen

# Собираем проект
go build -o crud-gen main.go
```


Или используйте уже скомпилированный бинарник из релизов.

### Пример 1: Базовая генерация (CLI)

Структура:
```go
// models/user.go
package models

import "time"

type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Age       int       `json:"age"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Password  string    `json:"-"` // исключится из CRUD
    Bio       *string   `json:"bio"`
}
```

Генерация:
```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo
```

Результат — файл `repositories/user_repo.go` с интерфейсом и реализацией:

```go
type UserRepository interface {
    Create(m *models.User) error
    Get(id int) (*models.User, error)
    Update(id int, m *models.User) error
    Delete(id int) error
    List(limit, offset int) ([]*models.User, error)
}
```

### Пример 2: Генерация с тестами

```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo \
  --with-tests
```

Создаст `user_repo.go` и `user_repo_test.go` с готовыми тестами.

### Пример 3: Генерация для MySQL

```bash
./crud-gen \
  --model=Product \
  --input=./models/product.go \
  --output=./repositories/product_repo.go \
  --package=repo \
  --dialect=mysql
```

Генератор автоматически использует MySQL синтаксис (например, `LAST_INSERT_ID()` вместо `RETURNING`).

### Пример 4: Использование кастомного шаблона

```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo \
  --repo-template=./templates/my_repo.tmpl
```

## Флаги CLI

| Флаг | Описание | Обязателен | По умолчанию |
|------|---------|-----------|--------------|
| `--model` | Название структуры для генерации (например, User) | Да | — |
| `--input` | Путь к файлу с структурой (например, ./models/user.go) | Да | — |
| `--output` | Путь для сохранения сгенерированного кода | Нет | stdout |
| `--package` | Имя пакета для сгенерированного кода | Да | — |
| `--id-field` | Название поля первичного ключа | Нет | ID |
| `--dialect` | SQL диалект: postgres, mysql, sqlite | Нет | postgres |
| `--with-tests` | Генерировать unit тесты | Нет | false |
| `--repo-template` | Путь к кастомному шаблону репозитория | Нет | встроенный |
| `--test-template` | Путь к кастомному шаблону тестов | Нет | встроенный |
| `--serve` | Запустить HTTP сервер вместо CLI | Нет | false |
| `--port` | Порт для HTTP сервера | Нет | 8080 |

## HTTP API

Запустите сервер:
```bash
./crud-gen --serve --port=8080
```

### POST /generate

Создает код на основе JSON конфигурации.

**Запрос:**
```bash
curl -X POST http://localhost:8080/generate \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "User",
    "input": "./models/user.go",
    "output": "./repositories/user_repo.go",
    "package": "repo",
    "dialect": "postgres",
    "with_tests": true
  }'
```

**Ответ:**
```json
{
  "status": "ok",
  "output": "./repositories/user_repo.go"
}
```

### GET /health

Проверка статуса сервера.

```bash
curl http://localhost:8080/health
# ok
```

## Архитектура

### 1. Parser (`internal/parser/parser.go`)

Использует Go AST для анализа структур:
- Читает исходный Go файл
- Находит структуру по названию
- Извлекает названия полей, типы, struct tags
- Определяет excluded поля (`json:"-"`)
- Классифицирует типы (pointer, time.Time, и т.д.)

### 2. Generator (`internal/generator/`)

**data.go:**
- Преобразует метаданные структуры в SQL фрагменты
- Строит SELECT, INSERT, UPDATE, DELETE запросы
- Конвертирует CamelCase → snake_case
- Подготавливает данные для шаблонизации

**generator.go:**
- Загружает шаблоны
- Подставляет данные в шаблоны
- Форматирует код через gofmt
- Пишет результат в файл или stdout

### 3. Templates (`internal/templates/templates.go`)

Встроенные Go шаблоны для:
- Интерфейса репозитория
- Реализации CRUD методов
- Unit тестов

Поддерживает переменные:
- `{{.SelectColumns}}` — SELECT список
- `{{.InsertColumns}}` — INSERT колонки
- `{{.SetClause}}` — UPDATE SET
- `{{.TableName}}` — таблица БД
- `{{.ModelRef}}` — ссылка на модель

### 4. Main (`main.go`)

- Парсинг флагов
- Валидация конфигурации
- Детектирование модуля (go.mod) для cross-package генерации
- HTTP сервер
- Оркестрация process

## Примеры использования

### Генерация для всех моделей в проекте

```bash
for model in User Product Post; do
  ./crud-gen \
    --model=$model \
    --input=./models/${model,,}.go \
    --output=./repositories/${model,,}_repo.go \
    --package=repo \
    --with-tests
done
```

### Использование в Makefile

```makefile
.PHONY: generate

generate:
	@echo "Generating CRUD repositories..."
	./crud-gen --model=User --input=./models/user.go --output=./repositories/user_repo.go --package=repo
	./crud-gen --model=Product --input=./models/product.go --output=./repositories/product_repo.go --package=repo --dialect=mysql
	./crud-gen --model=Post --input=./models/post.go --output=./repositories/post_repo.go --package=repo
	@echo "Done!"
```

### Использование в коде

```go
package main

import (
    "database/sql"
    "log"
    
    "myapp/models"
    "myapp/repositories"
    
    _ "github.com/lib/pq"
)

func main() {
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Создаем репозиторий
    userRepo := repositories.NewUserRepository(db)
    
    // Используем CRUD операции
    user := &models.User{
        Name:  "John Doe",
        Email: "john@example.com",
        Age:   30,
    }
    
    if err := userRepo.Create(user); err != nil {
        log.Fatal(err)
    }
    
    // user.ID заполнится автоматически
    log.Printf("Created user with ID: %d\n", user.ID)
    
    // Получение
    retrieved, err := userRepo.Get(user.ID)
    if err != nil {
        log.Fatal(err)
    }
    
    // Обновление
    retrieved.Email = "newemail@example.com"
    if err := userRepo.Update(user.ID, retrieved); err != nil {
        log.Fatal(err)
    }
    
    // Листинг
    users, err := userRepo.List(10, 0)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Found %d users\n", len(users))
    
    // Удаление
    if err := userRepo.Delete(user.ID); err != nil {
        log.Fatal(err)
    }
}
```

## Кастомные шаблоны

Создайте файл `templates/user_repo.tmpl`:

```go
package {{.Package}}

import (
    "database/sql"
    "{{.ModelImport}}"
)

type {{.ReceiverName}}Repository struct{ db *sql.DB }

func New{{.ModelName}}Repository(db *sql.DB) *{{.ReceiverName}}Repository {
    return &{{.ReceiverName}}Repository{db: db}
}

// Create ...
func (r *{{.ReceiverName}}Repository) Create(m *{{.ModelRef}}) error {
    const q = `INSERT INTO {{.TableName}} ({{.InsertColumns}}) 
               VALUES ({{.InsertPlaceholders}}) RETURNING id`
    return r.db.QueryRow(q, {{.InsertArgs}}).Scan(&m.ID)
}
```

Используйте:
```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo \
  --repo-template=./templates/user_repo.tmpl
```

## Запуск тестов

### Локально с PostgreSQL (Docker)

```bash
# Запустить PostgreSQL
docker run -d \
  --name postgres_test \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  postgres:15

# Создать таблицы
docker exec postgres_test psql -U postgres -d testdb -c "
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    email VARCHAR(255),
    age INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    bio TEXT
);"

# Запустить тесты
TEST_DB_URL="postgres://postgres:testpass@localhost:5432/testdb?sslmode=disable" \
go test ./repositories -v

# Остановить контейнер
docker stop postgres_test && docker rm postgres_test
```

### Через docker-compose

Создайте `docker-compose.yml`:

```yaml
version: '3.9'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: testpass
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
    volumes:
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql

  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: testpass
      MYSQL_DATABASE: testdb
    ports:
      - "3306:3306"
    volumes:
      - ./init-mysql.sql:/docker-entrypoint-initdb.d/init.sql

  sqlite:
    image: sqlite
    volumes:
      - ./test.db:/data/test.db
```

Запуск:
```bash
docker-compose up -d
TEST_DB_URL="postgres://postgres:testpass@localhost:5432/testdb?sslmode=disable" go test ./repositories -v
docker-compose down
```

## Валидация

Генератор проверяет:

1. **Обязательные флаги** — `--model`, `--input`, `--package`
2. **Существование файла** — файл с моделью должен существовать
3. **Наличие структуры** — структура должна быть найдена в файле
4. **Диалект SQL** — postgres, mysql или sqlite
5. **Синтаксис шаблонов** — если задан кастомный шаблон

Примеры ошибок:
```
error: required flags missing: --model, --input, --package
error: struct "User" not found in ./models/user.go
error: unknown dialect "oracle": must be postgres, mysql, or sqlite
```

## Ограничения

1. **Один первичный ключ** — поддерживается только один ID field
2. **Простые типы** — базовые Go типы (int, string, bool, time.Time, float64, и т.д.)
3. **Плоские структуры** — вложенные структуры не поддерживаются
4. **Исключение полей** — только через `json:"-"` tag
5. **PostgreSQL по умолчанию** — если не указан `--dialect`

## Структура проекта

```
crud-gen/
├── main.go                           # Точка входа, CLI, HTTP обработчик
├── go.mod                            # Module definition
├── go.sum                            # Dependencies checksum
├── models/
│   ├── user.go                       # Пример: User структура
│   ├── product.go                    # Пример: Product структура
│   └── post.go                       # Пример: Post структура
├── repositories/
│   ├── user_repo.go                  # СГЕНЕРИРОВАННЫЙ репозиторий
│   └── user_repo_test.go             # СГЕНЕРИРОВАННЫЕ тесты
├── internal/
│   ├── parser/
│   │   └── parser.go                 # AST парсер структур
│   ├── generator/
│   │   ├── generator.go              # Оркестрация генерации
│   │   ├── data.go                   # SQL фрагменты и подготовка
│   │   └── templates.go              # Встроенные шаблоны
│   └── templates/
│       └── templates.go              # (включено в generator)
├── README.md                         # Документация
└── Dockerfile                        # Контейнеризация
```

## Диагностика проблем

### "struct not found"

Убедитесь, что название структуры совпадает с флагом `--model`:
```bash
# ✗ неправильно
./crud-gen --model=user --input=./models/user.go

# ✓ правильно
./crud-gen --model=User --input=./models/user.go
```

### Поле не генерируется в SQL

Проверьте struct tag — если `json:"-"`, поле исключится:
```go
type User struct {
    Password string `json:"-"` // не будет в CRUD
    Email    string `json:"email"` // будет в CRUD
}
```

### SQL синтаксис не совпадает с БД

Убедитесь, что используете правильный диалект:
```bash
# для MySQL
./crud-gen --model=User --input=./models/user.go --dialect=mysql

# для SQLite
./crud-gen --model=User --input=./models/user.go --dialect=sqlite
```

### Тесты требуют переменную окружения

Установите `TEST_DB_URL`:
```bash
export TEST_DB_URL="postgres://user:pass@localhost/dbname"
go test ./repositories -v
```

## Производительность

Типичные показатели на MacBook M1:

- Парсинг структуры: ~2ms
- Генерация репозитория: ~5ms
- Форматирование (gofmt): ~10ms
- **Итого**: ~17ms для одной структуры

Сборка бинарника:
```bash
# ~3 секунды
time go build -o crud-gen main.go
```

## Вклад

Если вы нашли баг или хотите добавить функцию:

1. Откройте Issue
2. Создайте Pull Request с описанием
3. Убедитесь, что код проходит `go fmt` и `go test`

## Лицензия

MIT License

---

# CRUD-Gen — CRUD code generator for Go

**CRUD-Gen** is a command-line tool that automatically generates type-safe database code based on Go structures. Supports PostgreSQL, MySQL and SQLite, allows custom templates and works both via CLI and HTTP API.

## Features

- **AST parsing** — analyzes Go structures at the syntax tree level
- **Automatic CRUD generation** — Create, Read (Get), Update, Delete, List operations
- **3 database support** — PostgreSQL, MySQL, SQLite with automatic SQL syntax adaptation
- **Custom templates** — override generation for your needs
- **CLI and HTTP API** — use as a command or as a microservice
- **Type safety** — parameterized queries, SQL injection protection
- **Auto-generated tests** — basic unit tests with `--with-tests` flag

## Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/yourusername/crud-gen.git
cd crud-gen

# Build project
go build -o crud-gen main.go
```

Or use a pre-compiled binary from releases.

### Example 1: Basic generation (CLI)

Structure:
```go
// models/user.go
package models

import "time"

type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Age       int       `json:"age"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Password  string    `json:"-"` // will be excluded from CRUD
    Bio       *string   `json:"bio"`
}
```

Generation:
```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo
```

Result — file `repositories/user_repo.go` with interface and implementation:

```go
type UserRepository interface {
    Create(m *models.User) error
    Get(id int) (*models.User, error)
    Update(id int, m *models.User) error
    Delete(id int) error
    List(limit, offset int) ([]*models.User, error)
}
```

### Example 2: Generation with tests

```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo \
  --with-tests
```

Creates `user_repo.go` and `user_repo_test.go` with ready-made tests.

### Example 3: Generation for MySQL

```bash
./crud-gen \
  --model=Product \
  --input=./models/product.go \
  --output=./repositories/product_repo.go \
  --package=repo \
  --dialect=mysql
```

Generator automatically uses MySQL syntax (e.g., `LAST_INSERT_ID()` instead of `RETURNING`).

### Example 4: Using custom template

```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo \
  --repo-template=./templates/my_repo.tmpl
```

## CLI Flags

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `--model` | Struct name for generation (e.g., User) | Yes | — |
| `--input` | Path to file with struct (e.g., ./models/user.go) | Yes | — |
| `--output` | Path to save generated code | No | stdout |
| `--package` | Package name for generated code | Yes | — |
| `--id-field` | Primary key field name | No | ID |
| `--dialect` | SQL dialect: postgres, mysql, sqlite | No | postgres |
| `--with-tests` | Generate unit tests | No | false |
| `--repo-template` | Path to custom repository template | No | built-in |
| `--test-template` | Path to custom test template | No | built-in |
| `--serve` | Run HTTP server instead of CLI | No | false |
| `--port` | HTTP server port | No | 8080 |

## HTTP API

Start the server:
```bash
./crud-gen --serve --port=8080
```

### POST /generate

Creates code based on JSON configuration.

**Request:**
```bash
curl -X POST http://localhost:8080/generate \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "User",
    "input": "./models/user.go",
    "output": "./repositories/user_repo.go",
    "package": "repo",
    "dialect": "postgres",
    "with_tests": true
  }'
```

**Response:**
```json
{
  "status": "ok",
  "output": "./repositories/user_repo.go"
}
```

### GET /health

Check server status.

```bash
curl http://localhost:8080/health
# ok
```

## Architecture

### 1. Parser (`internal/parser/parser.go`)

Uses Go AST for structure analysis:
- Reads source Go file
- Finds struct by name
- Extracts field names, types, struct tags
- Identifies excluded fields (`json:"-"`)
- Classifies types (pointer, time.Time, etc.)

### 2. Generator (`internal/generator/`)

**data.go:**
- Transforms struct metadata into SQL fragments
- Builds SELECT, INSERT, UPDATE, DELETE queries
- Converts CamelCase → snake_case
- Prepares data for templating

**generator.go:**
- Loads templates
- Injects data into templates
- Formats code through gofmt
- Writes result to file or stdout

### 3. Templates (`internal/templates/templates.go`)

Built-in Go templates for:
- Repository interface
- CRUD methods implementation
- Unit tests

Supports variables:
- `{{.SelectColumns}}` — SELECT list
- `{{.InsertColumns}}` — INSERT columns
- `{{.SetClause}}` — UPDATE SET
- `{{.TableName}}` — database table
- `{{.ModelRef}}` — model reference

### 4. Main (`main.go`)

- Flag parsing
- Configuration validation
- Module detection (go.mod) for cross-package generation
- HTTP server
- Process orchestration

## Usage Examples

### Generate for all models in project

```bash
for model in User Product Post; do
  ./crud-gen \
    --model=$model \
    --input=./models/${model,,}.go \
    --output=./repositories/${model,,}_repo.go \
    --package=repo \
    --with-tests
done
```

### Using in Makefile

```makefile
.PHONY: generate

generate:
	@echo "Generating CRUD repositories..."
	./crud-gen --model=User --input=./models/user.go --output=./repositories/user_repo.go --package=repo
	./crud-gen --model=Product --input=./models/product.go --output=./repositories/product_repo.go --package=repo --dialect=mysql
	./crud-gen --model=Post --input=./models/post.go --output=./repositories/post_repo.go --package=repo
	@echo "Done!"
```

### Using in code

```go
package main

import (
    "database/sql"
    "log"
    
    "myapp/models"
    "myapp/repositories"
    
    _ "github.com/lib/pq"
)

func main() {
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Create repository
    userRepo := repositories.NewUserRepository(db)
    
    // Use CRUD operations
    user := &models.User{
        Name:  "John Doe",
        Email: "john@example.com",
        Age:   30,
    }
    
    if err := userRepo.Create(user); err != nil {
        log.Fatal(err)
    }
    
    // user.ID is auto-filled
    log.Printf("Created user with ID: %d\n", user.ID)
    
    // Read
    retrieved, err := userRepo.Get(user.ID)
    if err != nil {
        log.Fatal(err)
    }
    
    // Update
    retrieved.Email = "newemail@example.com"
    if err := userRepo.Update(user.ID, retrieved); err != nil {
        log.Fatal(err)
    }
    
    // List
    users, err := userRepo.List(10, 0)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Found %d users\n", len(users))
    
    // Delete
    if err := userRepo.Delete(user.ID); err != nil {
        log.Fatal(err)
    }
}
```

## Custom Templates

Create `templates/user_repo.tmpl`:

```go
package {{.Package}}

import (
    "database/sql"
    "{{.ModelImport}}"
)

type {{.ReceiverName}}Repository struct{ db *sql.DB }

func New{{.ModelName}}Repository(db *sql.DB) *{{.ReceiverName}}Repository {
    return &{{.ReceiverName}}Repository{db: db}
}

// Create ...
func (r *{{.ReceiverName}}Repository) Create(m *{{.ModelRef}}) error {
    const q = `INSERT INTO {{.TableName}} ({{.InsertColumns}}) 
               VALUES ({{.InsertPlaceholders}}) RETURNING id`
    return r.db.QueryRow(q, {{.InsertArgs}}).Scan(&m.ID)
}
```

Use it:
```bash
./crud-gen \
  --model=User \
  --input=./models/user.go \
  --output=./repositories/user_repo.go \
  --package=repo \
  --repo-template=./templates/user_repo.tmpl
```

## Running Tests

### Locally with PostgreSQL (Docker)

```bash
# Run PostgreSQL
docker run -d \
  --name postgres_test \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  postgres:15

# Create tables
docker exec postgres_test psql -U postgres -d testdb -c "
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255),
    email VARCHAR(255),
    age INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    bio TEXT
);"

# Run tests
TEST_DB_URL="postgres://postgres:testpass@localhost:5432/testdb?sslmode=disable" \
go test ./repositories -v

# Stop container
docker stop postgres_test && docker rm postgres_test
```

### Via docker-compose

Create `docker-compose.yml`:

```yaml
version: '3.9'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: testpass
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
    volumes:
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql

  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: testpass
      MYSQL_DATABASE: testdb
    ports:
      - "3306:3306"
    volumes:
      - ./init-mysql.sql:/docker-entrypoint-initdb.d/init.sql

  sqlite:
    image: sqlite
    volumes:
      - ./test.db:/data/test.db
```

Run:
```bash
docker-compose up -d
TEST_DB_URL="postgres://postgres:testpass@localhost:5432/testdb?sslmode=disable" go test ./repositories -v
docker-compose down
```

## Validation

Generator checks:

1. **Required flags** — `--model`, `--input`, `--package`
2. **File existence** — model file must exist
3. **Struct presence** — struct must be found in file
4. **SQL dialect** — postgres, mysql or sqlite
5. **Template syntax** — if custom template is provided

Error examples:
```
error: required flags missing: --model, --input, --package
error: struct "User" not found in ./models/user.go
error: unknown dialect "oracle": must be postgres, mysql, or sqlite
```

## Limitations

1. **Single primary key** — only one ID field is supported
2. **Simple types** — basic Go types (int, string, bool, time.Time, float64, etc.)
3. **Flat structures** — nested structures are not supported
4. **Field exclusion** — only via `json:"-"` tag
5. **PostgreSQL by default** — if `--dialect` is not specified

## Project Structure

```
crud-gen/
├── main.go                           # Entry point, CLI, HTTP handler
├── go.mod                            # Module definition
├── go.sum                            # Dependencies checksum
├── models/
│   ├── user.go                       # Example: User struct
│   ├── product.go                    # Example: Product struct
│   └── post.go                       # Example: Post struct
├── repositories/
│   ├── user_repo.go                  # GENERATED repository
│   └── user_repo_test.go             # GENERATED tests
├── internal/
│   ├── parser/
│   │   └── parser.go                 # AST struct parser
│   ├── generator/
│   │   ├── generator.go              # Generation orchestration
│   │   ├── data.go                   # SQL fragments and prep
│   │   └── templates.go              # Built-in templates
│   └── templates/
│       └── templates.go              # (included in generator)
├── README.md                         # Documentation
└── Dockerfile                        # Containerization
```

## Troubleshooting

### "struct not found"

Make sure struct name matches the `--model` flag:
```bash
# ✗ incorrect
./crud-gen --model=user --input=./models/user.go

# ✓ correct
./crud-gen --model=User --input=./models/user.go
```

### Field not generated in SQL

Check struct tag — if `json:"-"`, field is excluded:
```go
type User struct {
    Password string `json:"-"` // won't be in CRUD
    Email    string `json:"email"` // will be in CRUD
}
```

### SQL syntax doesn't match database

Make sure you use correct dialect:
```bash
# for MySQL
./crud-gen --model=User --input=./models/user.go --dialect=mysql

# for SQLite
./crud-gen --model=User --input=./models/user.go --dialect=sqlite
```

### Tests require environment variable

Set `TEST_DB_URL`:
```bash
export TEST_DB_URL="postgres://user:pass@localhost/dbname"
go test ./repositories -v
```

## Performance

Typical metrics on MacBook M1:

- Struct parsing: ~2ms
- Repository generation: ~5ms
- Formatting (gofmt): ~10ms
- **Total**: ~17ms per struct

Binary build:
```bash
# ~3 seconds
time go build -o crud-gen main.go
```

## Contributing

If you found a bug or want to add a feature:

1. Open an Issue
2. Create a Pull Request with description
3. Make sure code passes `go fmt` and `go test`

## License

MIT License
