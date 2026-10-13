# Contributing to Vitis

[Русский](#участие-в-разработке-vitis) · [Developer setup](docs/eng/development.md) · [Documentation](docs/eng/README.md)

## Setup and verification

Use Go 1.23+ and Git. Build and test from the repository root:

Linux (Bash):

```bash
go mod download
go build ./...
go test ./... -count=1
go vet ./...
make docs-check
```

Windows (PowerShell 7.2+):

```powershell
go mod download
go build ./...
go test ./... -count=1
go vet ./...
go run ./cmd/vitis-doccheck
```

Startup prepares vanilla resources automatically; see [installation](docs/eng/getting-started.md). Full code generation is a separate development operation.

| Make target | Purpose |
| --- | --- |
| `build` | Build `vitis` from the complete command package |
| `run` | Build and run with the checked-in example YAML; missing resources are prepared automatically |
| `test` | Run all tests without cached results |
| `lint` | Run go vet |
| `test-race` | Test with the race detector |
| `fuzz-smoke` | Exercise the current protocol/NBT/decompression fuzz targets |
| `docs-check` | Check local Markdown link targets, EN/RU page sets and configuration-key coverage |
| `verify` | Combine test, vet, race, fuzz smoke and docs checks |
| `generate` | Automatic input preparation and full regeneration; `make generate MC_VERSION=1.21.4` (default: 1.21.4); see [prerequisites](docs/eng/generators/README.md) |
| `clean` | Remove the server binary |

On Windows, use the native PowerShell equivalents described in [developer setup](docs/eng/development.md) and [generation](docs/eng/generators/README.md).

Check the baseline before editing and report existing failures separately; see [known limitations](docs/eng/status.md).

## Coding conventions

Follow idiomatic Go, existing package boundaries, SOLID/KISS/DRY where they simplify the implementation, and the project's formatting. Keep functions focused; avoid speculative abstractions and unrelated rewrites.

- Use gofmt. Keep names meaningful and errors lowercase without trailing punctuation.
- Write concise English Go documentation comments for packages, types and functions. Preserve the existing preference for documentation comments over inline commentary.
- Use structured `internal/logger` logging with contextual fields; avoid `log.Printf` and `fmt.Println` in production logic.
- Validate untrusted packet/command input. Keep protocol decoding, gameplay authority and storage responsibilities separate.
- Keep blocking IO out of network/tick paths. Respect state ownership, handle bounded-queue refusal, and cancel/join owned goroutines on shutdown.
- Add focused tests for new logic and edge cases; use race checks for concurrency and malformed-input tests for codecs.
- Change generators/inputs instead of hand-editing `internal/data/generated`. Inspect every generated diff.

Read [architecture](docs/eng/architecture.md), [extension guides](docs/eng/extending.md) and [testing](docs/eng/testing.md) for runtime-specific contracts.

## Branches and commits

Create branches from `dev` and target PRs at **`dev`**. Maintainers merge changes into `main`.

| Prefix | Purpose |
| --- | --- |
| `feature/` | New functionality |
| `fix/` | Bug fixes |
| `hotfix/` | Critical fixes intended for main |
| `docs/` | Documentation |
| `refactor/` | Refactoring |
| `test/` | Tests |
| `chore/` | Maintenance |

Use short lowercase hyphenated names, optionally including an issue number: `fix/123-chunk-unload`.

Use Conventional Commits: `<type>(<optional scope>): <description>`. Existing types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `revert`. Use imperative mood, a subject of at most 72 characters with no final period, and a blank line before an optional body explaining what and why.

Example: `fix(world): preserve dirty state after asynchronous save`.

## Pull requests and issues

Keep each PR focused. Describe the concrete problem, resulting behavior and validation. Run relevant tests and vet; use full verification when runtime/concurrency changes require it. Document baseline failures and unavailable checks explicitly. Add tests for new functionality and update both documentation languages when behavior changes.

For documentation changes, review translations, anchors, external URLs, examples and defaults manually: `docs-check` only verifies structural properties. Each configuration key has exactly one backticked first-cell table row in both references.

Issue reports need revision, OS/toolchain/client versions, reproduction steps, expected/actual behavior and relevant logs. Redact credentials and personal/runtime data. See [troubleshooting](docs/eng/troubleshooting.md).

Contributions are licensed under [GPLv3](LICENSE).

---

# Участие в разработке Vitis

[English](#contributing-to-vitis) · [Окружение разработчика](docs/rus/development.md) · [Документация](docs/rus/README.md)

## Окружение и проверки

Нужны Go 1.23+ и Git. Выполняйте команды из корня репозитория.

Linux (Bash):

```bash
go mod download
go build ./...
go test ./... -count=1
go vet ./...
make docs-check
```

Windows (PowerShell 7.2+):

```powershell
go mod download
go build ./...
go test ./... -count=1
go vet ./...
go run ./cmd/vitis-doccheck
```

При запуске сервер сам подготавливает ресурсы Minecraft; см. [установку](docs/rus/getting-started.md). Полная кодогенерация выполняется отдельно при разработке.

| Цель Make | Назначение |
| --- | --- |
| `build` | Собрать `vitis` из пакета команды сервера |
| `run` | Собрать и запустить с примером YAML; отсутствующие ресурсы готовятся автоматически |
| `test` | Запустить все тесты без использования кешированных результатов |
| `lint` | Выполнить go vet |
| `test-race` | Проверить гонки данных |
| `fuzz-smoke` | Выполнить краткие проверки протокола, NBT и распаковки с генерируемыми входными данными |
| `docs-check` | Проверить локальные ссылки, парность EN/RU и покрытие ключей конфигурации |
| `verify` | Выполнить тесты, go vet, проверку гонок, фаззинг и проверку документации |
| `generate` | Подготовить входные данные и запустить генераторы; `make generate MC_VERSION=1.21.4` (по умолчанию 1.21.4); см. [зависимости](docs/rus/generators/README.md) |
| `clean` | Удалить бинарник сервера |

Команды PowerShell для Windows описаны в [окружении разработчика](docs/rus/development.md) и [генерации](docs/rus/generators/README.md).

До изменений проверьте результаты тестов и отдельно отметьте существующие ошибки; см. [ограничения](docs/rus/status.md).

## Правила кода

Следуйте принятым в Go подходам, границам пакетов и форматированию проекта. Используйте SOLID/KISS/DRY там, где они упрощают код. Функции должны решать конкретные задачи; избегайте абстракций без необходимости и несвязанных переписываний.

- Используйте gofmt и понятные имена. Сообщения ошибок начинайте со строчной буквы, без точки в конце.
- Пишите краткие документационные комментарии к пакетам, типам и функциям на английском. Предпочитайте их комментариям внутри реализации.
- Используйте `internal/logger` со структурированными полями; избегайте `log.Printf` и `fmt.Println` в коде сервера.
- Проверяйте входные пакеты и команды. Разделяйте декодирование протокола, проверку игровых действий и хранение.
- Выполняйте блокирующие операции вне обработки сети и тиков. Соблюдайте владение состоянием, обрабатывайте отказ очереди, отменяйте и дожидайтесь горутин при остановке.
- Покрывайте новую логику и граничные случаи тестами. Проверяйте гонки в конкурентном коде и некорректные данные в кодеках.
- Меняйте генераторы и входные данные, а не файлы в `internal/data/generated` вручную. Проверяйте все изменения результата генерации.

Работа подсистем описана в [архитектуре](docs/rus/architecture.md), [добавлении функциональности](docs/rus/extending.md) и [тестировании](docs/rus/testing.md).

## Ветки и коммиты

Создавайте ветки от `dev` и направляйте PR в **`dev`**. Сопровождающие проекта переносят изменения в `main`.

| Префикс | Назначение |
| --- | --- |
| `feature/` | Новая функциональность |
| `fix/` | Исправления |
| `hotfix/` | Критические исправления для main |
| `docs/` | Документация |
| `refactor/` | Рефакторинг |
| `test/` | Тесты |
| `chore/` | Обслуживание |

Имена веток короткие, строчными буквами, с дефисами; номер задачи необязателен: `fix/123-chunk-unload`.

Используйте Conventional Commits: `<type>(<optional scope>): <description>`. Типы: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `revert`. Заголовок пишется в повелительном наклонении, до 72 символов, без точки. Дополнительное описание отделяется пустой строкой и поясняет изменение и его причину.

Пример: `fix(world): preserve dirty state after asynchronous save`.

## Запросы на включение изменений и отчёты об ошибках

Описывайте проблему, итоговое поведение и выполненные проверки. Ограничивайте PR одной задачей. Запускайте подходящие тесты и go vet; для изменений работы сервера и конкурентности при необходимости выполняйте полную проверку. Указывайте исходные ошибки и недоступные проверки. Добавляйте тесты новой логики и обновляйте оба языка документации при изменении поведения.

В документации вручную проверяйте перевод, ссылки на разделы, внешние сайты, примеры и значения по умолчанию. `docs-check` проверяет структуру. Каждый ключ конфигурации должен иметь одну строку с ключом в обратных кавычках в первой ячейке обоих справочников.

В отчёте об ошибке укажите коммит, версии ОС, инструментов и клиента, шаги воспроизведения, ожидаемый и фактический результат, логи. Удалите пароли и личные данные. См. [диагностику](docs/rus/troubleshooting.md).

Изменения распространяются под [GPLv3](LICENSE).
