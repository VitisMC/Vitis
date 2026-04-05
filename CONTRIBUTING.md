# Contributing to Vitis

[Русская версия ниже](#участие-в-разработке-vitis) | [Russian version below](#участие-в-разработке-vitis)

Thank you for your interest in contributing to Vitis! This document covers how to set up the project, coding conventions, and how to submit changes.

## Getting Started

### Prerequisites

- **Go 1.23+**
- **Git**

### Build & Run

```bash
# Clone
git clone https://github.com/vitismc/vitis.git
cd vitis

# Build
go build -o vitis ./cmd/vitis

# Run
cp configs/vitis.yaml vitis.yaml
./vitis -config vitis.yaml

# Run tests
go test ./internal/... -count=1
```

### Using the Makefile

```bash
make build    # Build the binary
make run      # Build and run
make test     # Run all tests
make lint     # Run go vet
make clean    # Remove build artifacts
```

## Coding Style

- Follow standard Go conventions ([Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments))
- Write **docstrings in English only**
- Do **not** write inline comments — use docstrings instead
- Use `gofmt` (or `goimports`) to format code
- Keep functions short and focused
- Use meaningful variable names
- Error messages should be lowercase and not end with punctuation

### Logging

Use the structured logger from `internal/logger`:

```go
import "github.com/vitismc/vitis/internal/logger"

logger.Info("player joined", "name", name, "session", s.ID())
logger.Error("failed to load chunk", "x", cx, "z", cz, "error", err)
logger.Debug("packet received", "id", packetID)
```

Do **not** use `log.Printf` or `fmt.Println` in production code.

### Generated Code

Files under `internal/data/generated/` are auto-generated. Do **not** edit them manually. To regenerate:

```bash
./scripts/update_version.sh 1.21.4
```

## Branch Model

### Main Branches

| Branch | Purpose |
|--------|---------|
| `main` | Stable, production-ready code. Only updated via merges from `dev` by maintainers. |
| `dev`  | Active development. All PRs target this branch. |

### Feature Branches

Create branches from `dev` using the following prefixes:

| Prefix | Purpose | Example |
|--------|---------|---------|
| `feature/` | New functionality | `feature/inventory-drag` |
| `fix/` | Bug fixes | `fix/chunk-unload-crash` |
| `hotfix/` | Critical fixes for `main` | `hotfix/login-timeout` |
| `docs/` | Documentation only | `docs/readme-badges` |
| `refactor/` | Code refactoring (no behavior change) | `refactor/session-cleanup` |
| `test/` | Adding or updating tests | `test/physics-collision` |
| `chore/` | Maintenance, dependencies, configs | `chore/update-go-mod` |

### Branch Naming Rules

- Use lowercase with hyphens: `feature/player-death-event`
- Keep names short but descriptive
- Include issue number if applicable: `fix/123-chunk-leak`

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `style` | Formatting, no code change |
| `refactor` | Code change without adding features or fixing bugs |
| `perf` | Performance improvement |
| `test` | Adding or updating tests |
| `chore` | Maintenance (deps, configs, scripts) |
| `ci` | CI/CD changes |
| `revert` | Revert a previous commit |

### Scope (optional)

The affected module or area: `protocol`, `world`, `entity`, `session`, `network`, `inventory`, `command`, `config`, etc.

### Examples

```
feat(inventory): add drag-click support

fix(world): prevent chunk unload during player teleport

docs: update README badges

refactor(session): extract packet routing to separate file

perf(protocol): use buffer pool for packet encoding

test(physics): add AABB intersection edge cases

chore: update go.mod dependencies
```

### Rules

- Use imperative mood: "add feature" not "added feature"
- First line max 72 characters
- No period at the end of the subject line
- Separate subject from body with a blank line
- Body explains *what* and *why*, not *how*

## Pull Requests

1. Fork the repository
2. Create a feature branch from `dev`: `git checkout -b feature/my-change dev`
3. Make your changes
4. Run tests: `go test ./internal/... -count=1`
5. Run vet: `go vet ./...`
6. Commit with a clear message
7. Open a pull request against **`dev`** (not `main`)

### PR Guidelines

- Keep PRs focused — one feature or fix per PR
- Include a short description of what changed and why
- Ensure all tests pass and the build is clean
- Add tests for new functionality when practical
- Do **not** open PRs directly against `main`

## Reporting Issues

Open an issue on GitHub with:
- A clear title
- Steps to reproduce (if applicable)
- Expected vs actual behavior
- Go version and OS

## License

By contributing to Vitis, you agree that your contributions will be licensed under the [GPLv3](LICENSE).

---

# Участие в разработке Vitis

[English version above](#contributing-to-vitis) | [Английская версия выше](#contributing-to-vitis)

Спасибо за интерес к участию в разработке Vitis! Этот документ описывает настройку проекта, стиль кода и процесс внесения изменений.

## Начало работы

### Требования

- **Go 1.23+**
- **Git**

### Сборка и запуск

```bash
# Клонирование
git clone https://github.com/vitismc/vitis.git
cd vitis

# Сборка
go build -o vitis ./cmd/vitis

# Запуск
cp configs/vitis.yaml vitis.yaml
./vitis -config vitis.yaml

# Запуск тестов
go test ./internal/... -count=1
```

### Использование Makefile

```bash
make build    # Собрать бинарник
make run      # Собрать и запустить
make test     # Запустить все тесты
make lint     # Запустить go vet
make clean    # Удалить артефакты сборки
```

## Стиль кода

- Следуйте стандартным Go-конвенциям ([Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments))
- Пишите **docstrings только на английском**
- **Не** пишите инлайн-комментарии — используйте docstrings
- Используйте `gofmt` (или `goimports`) для форматирования
- Делайте функции короткими и сфокусированными
- Используйте осмысленные имена переменных
- Сообщения об ошибках должны быть в нижнем регистре и без точки в конце

### Логирование

Используйте структурированный логгер из `internal/logger`:

```go
import "github.com/vitismc/vitis/internal/logger"

logger.Info("player joined", "name", name, "session", s.ID())
logger.Error("failed to load chunk", "x", cx, "z", cz, "error", err)
logger.Debug("packet received", "id", packetID)
```

**Не** используйте `log.Printf` или `fmt.Println` в продакшен-коде.

### Сгенерированный код

Файлы в `internal/data/generated/` генерируются автоматически. **Не** редактируйте их вручную. Для перегенерации:

```bash
./scripts/update_version.sh 1.21.4
```

## Модель веток

### Основные ветки

| Ветка | Назначение |
|-------|------------|
| `main` | Стабильный, готовый к продакшену код. Обновляется только мержами из `dev` мейнтейнерами. |
| `dev`  | Активная разработка. Все PR идут сюда. |

### Фича-ветки

Создавайте ветки от `dev` со следующими префиксами:

| Префикс | Назначение | Пример |
|---------|------------|--------|
| `feature/` | Новая функциональность | `feature/inventory-drag` |
| `fix/` | Исправление багов | `fix/chunk-unload-crash` |
| `hotfix/` | Критические фиксы для `main` | `hotfix/login-timeout` |
| `docs/` | Только документация | `docs/readme-badges` |
| `refactor/` | Рефакторинг (без изменения поведения) | `refactor/session-cleanup` |
| `test/` | Добавление или обновление тестов | `test/physics-collision` |
| `chore/` | Обслуживание, зависимости, конфиги | `chore/update-go-mod` |

### Правила именования веток

- Используйте нижний регистр с дефисами: `feature/player-death-event`
- Имена должны быть короткими, но понятными
- Указывайте номер issue, если применимо: `fix/123-chunk-leak`

## Сообщения коммитов

Используйте формат [Conventional Commits](https://www.conventionalcommits.org/):

```
<тип>(<область>): <описание>

[опциональное тело]

[опциональный футер]
```

### Типы

| Тип | Описание |
|-----|----------|
| `feat` | Новая функциональность |
| `fix` | Исправление бага |
| `docs` | Изменения документации |
| `style` | Форматирование, без изменения кода |
| `refactor` | Изменение кода без добавления фич или исправления багов |
| `perf` | Улучшение производительности |
| `test` | Добавление или обновление тестов |
| `chore` | Обслуживание (зависимости, конфиги, скрипты) |
| `ci` | Изменения CI/CD |
| `revert` | Откат предыдущего коммита |

### Область (опционально)

Затронутый модуль или область: `protocol`, `world`, `entity`, `session`, `network`, `inventory`, `command`, `config` и т.д.

### Примеры

```
feat(inventory): add drag-click support

fix(world): prevent chunk unload during player teleport

docs: update README badges

refactor(session): extract packet routing to separate file

perf(protocol): use buffer pool for packet encoding

test(physics): add AABB intersection edge cases

chore: update go.mod dependencies
```

### Правила

- Используйте повелительное наклонение: "add feature", а не "added feature"
- Первая строка — максимум 72 символа
- Без точки в конце темы
- Отделяйте тему от тела пустой строкой
- Тело объясняет *что* и *почему*, а не *как*

## Pull Requests

1. Сделайте форк репозитория
2. Создайте фича-ветку от `dev`: `git checkout -b feature/my-change dev`
3. Внесите изменения
4. Запустите тесты: `go test ./internal/... -count=1`
5. Запустите vet: `go vet ./...`
6. Закоммитьте с понятным сообщением
7. Откройте pull request в **`dev`** (не в `main`)

### Рекомендации по PR

- Держите PR сфокусированными — одна фича или фикс на PR
- Включите краткое описание что изменилось и почему
- Убедитесь, что все тесты проходят и сборка чистая
- Добавляйте тесты для новой функциональности, когда это уместно
- **Не** открывайте PR напрямую в `main`

## Сообщения об ошибках

Откройте issue на GitHub с:
- Понятным заголовком
- Шагами воспроизведения (если применимо)
- Ожидаемое vs фактическое поведение
- Версия Go и ОС

## Лицензия

Внося вклад в Vitis, вы соглашаетесь, что ваши изменения будут лицензированы под [GPLv3](LICENSE).
