# Обновление Packet ID при переходе на новую версию Minecraft

Это пошаговое руководство описывает, как обновить систему Packet ID в Vitis при выходе новой версии Minecraft. Следуйте инструкциям по порядку.

---

## Предварительные требования

- Установленный Go (1.22+)
- Установленный `curl`
- Установленный `stringer`:
  ```bash
  go install golang.org/x/tools/cmd/stringer@latest
  ```
- Доступ к интернету для скачивания файлов

---

## Шаг 1. Узнать идентификатор новой версии

Перейдите в репозиторий PrismarineJS/minecraft-data на GitHub:

```
https://github.com/PrismarineJS/minecraft-data/tree/master/data/pc
```

Найдите папку с нужной версией. Например:
- `1.21.4` — текущая версия Vitis
- `1.21.5` — гипотетическая новая версия

**Важно:** версия должна уже присутствовать в PrismarineJS. Если её нет — данные протокола ещё не задокументированы сообществом, и придётся подождать.

Убедитесь, что в папке новой версии есть файл `protocol.json`:

```
https://github.com/PrismarineJS/minecraft-data/blob/master/data/pc/1.21.5/protocol.json
```

---

## Шаг 2. Обновить скрипт загрузки

Скрипт `scripts/update_version.sh` принимает версию Minecraft как аргумент и скачивает `protocol.json` автоматически:

```bash
./scripts/update_version.sh 1.21.5
```

Файл будет сохранён в `.mcdata/1.21.5/protocol.json`.

---

## Шаг 3. Скачать protocol.json

Запустите скрипт из корня проекта:

```bash
./scripts/update_version.sh 1.21.5
```

Ожидаемый вывод (Шаг 1 скрипта):
```
=== Step 1: Download PrismarineJS data ===
  protocol.json                 ✓ (XXXXX bytes)
```

**Проверьте**, что файл скачался и не пустой:

```bash
head -c 200 .mcdata/1.21.5/protocol.json
```

Вы должны увидеть начало JSON-файла. Если файл пустой или содержит HTML-ошибку — проверьте URL и наличие данных в репозитории PrismarineJS.

---

## Шаг 4. Обновить путь в генераторе

Откройте файл `internal/protocol/packetid/generator/main.go`.

Найдите константу `protocolJSONPath` в начале файла (строка ~15):

```go
// Было:
const protocolJSONPath = ".mcdata/1.21.4/protocol.json"

// Стало:
const protocolJSONPath = ".mcdata/1.21.5/protocol.json"
```

Это единственное место, которое нужно менять в генераторе.

---

## Шаг 5. Запустить генератор

Из корня проекта выполните:

```bash
go run ./internal/protocol/packetid/generator/
```

Ожидаемый вывод:
```
Generated /home/.../internal/protocol/packetid/packetid.go (XXXX bytes)
```

**Что произошло:**
- Генератор прочитал новый `protocol.json`
- Извлёк все маппинги packet_id → packet_name для всех состояний и направлений
- Сгенерировал новый `internal/protocol/packetid/packetid.go` с обновлёнными константами

---

## Шаг 6. Перегенерировать stringer-файлы

Из папки `internal/protocol/packetid/` выполните:

```bash
go generate ./internal/protocol/packetid/
```

Это обновит два файла:
- `clientboundpacketid_string.go`
- `serverboundpacketid_string.go`

Если команда завершилась с ошибкой `stringer: command not found`, установите его:

```bash
go install golang.org/x/tools/cmd/stringer@latest
```

И повторите `go generate`.

---

## Шаг 7. Проверить изменения в packetid.go

Посмотрите diff, чтобы понять что изменилось:

```bash
git diff internal/protocol/packetid/packetid.go
```

Типичные изменения при обновлении версии:

1. **Добавление новых пакетов** — появились новые константы
2. **Удаление пакетов** — некоторые константы исчезли
3. **Переименование пакетов** — изменились имена (редко)
4. **Смена порядка (ID)** — пакеты сместились, `iota` значения изменились

**Важно:** так как используется `iota`, числовые значения ID автоматически пересчитываются. Вам не нужно вручную проверять числа — генератор берёт их из авторитетного источника.

---

## Шаг 8. Обновить код пакетов

### 8.1. Проверить, нет ли ошибок компиляции

```bash
go build ./internal/protocol/...
```

Если сборка прошла успешно — все существующие ссылки на `packetid.*` константы валидны.

Если есть ошибки вида:

```
packetid.ClientboundSomeOldPacket undefined
```

Значит, этот пакет был **удалён или переименован** в новой версии. Нужно:

1. Найти файл, использующий эту константу
2. Проверить в новом `protocol.json`, как теперь называется этот пакет
3. Обновить ссылку на новое имя

### 8.2. Добавить новые пакеты (если нужно)

Если в новой версии появились новые пакеты, которые Vitis должен обрабатывать:

1. Создайте файл пакета в соответствующей папке (`internal/protocol/packets/{state}/`)
2. Реализуйте интерфейс `protocol.Packet` с методами `ID()`, `Decode()`, `Encode()`
3. В методе `ID()` используйте новую константу из `packetid`
4. Зарегистрируйте пакет в `internal/protocol/states/{state}.go`

### 8.3. Обработать удалённые пакеты

Если пакет удалён из протокола:

1. Удалите файл пакета
2. Удалите его регистрацию из `internal/protocol/states/{state}.go`
3. Удалите все ссылки на этот пакет в обработчиках

---

## Шаг 9. Запустить тесты

```bash
go test ./internal/protocol/...
```

Все тесты должны пройти. Если тест падает — скорее всего, он проверяет конкретный числовой ID, который изменился. Обновите ожидаемое значение в тесте.

---

## Шаг 10. Обновить прочие зависимости от версии

Проверьте и обновите версию протокола в других местах проекта:

```bash
# Найти все упоминания старой версии
grep -r "1.21.4" --include="*.go" --include="*.yaml" --include="*.json" --include="*.sh"
grep -r "769" --include="*.go"  # 769 — protocol version для 1.21.4
```

Места, которые обычно нужно обновить:

- `configs/vitis.yaml` — если там указана версия
- `.mcdata/1.21.4/version.json` — скачать новый `version.json`
- `scripts/update_version.sh` — версионные массивы
- Регистры (`internal/registry/`) — могут потребовать перегенерации
- Код в `internal/session/` — если проверяется protocol version

---

## Шаг 11. Финальная проверка

```bash
# Полная сборка
go build ./internal/...

# Все тесты
go test ./internal/...

# Интеграционные тесты (если есть)
go test ./test/...
```

---

## Краткая шпаргалка

Для быстрого обновления — минимальные шаги:

```bash
# 1. Запустить полный скрипт обновления
./scripts/update_version.sh 1.21.5

# 2. Обновить путь в generator/main.go (const protocolJSONPath)

# 3. Перегенерировать
go run ./internal/protocol/packetid/generator/
go generate ./internal/protocol/packetid/

# 4. Собрать и проверить
go build ./internal/protocol/...
go test ./internal/protocol/...
```

---

## Где брать файлы — справочник источников

| Что | Откуда | URL |
|-----|--------|-----|
| **protocol.json** (Packet ID, имена пакетов) | PrismarineJS/minecraft-data | `https://github.com/PrismarineJS/minecraft-data/tree/master/data/pc/{VERSION}` |
| **Реестры** (блоки, сущности, биомы и т.д.) | misode/mcmeta | `https://github.com/misode/mcmeta` (тег `{VERSION}-data-json`) |
| **Документация протокола** (человекочитаемая) | wiki.vg | `https://minecraft.wiki/w/Minecraft_Wiki:Projects/wiki.vg_merge/Protocol?oldid=2938097` |
| **Protocol version number** | wiki.vg | `https://minecraft.wiki/w/Minecraft_Wiki:Projects/wiki.vg_merge/` |

### PrismarineJS/minecraft-data

Основной источник для Packet ID. Содержит `protocol.json` для каждой версии Minecraft.

- Репозиторий: https://github.com/PrismarineJS/minecraft-data
- Данные для конкретной версии: `data/pc/{VERSION}/protocol.json`
- Raw URL для скачивания:
  ```
  https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/{VERSION}/protocol.json
  ```

### misode/mcmeta

Источник данных реестров (registry data). Используется скриптом `scripts/update_version.sh`.

- Репозиторий: https://github.com/misode/mcmeta
- Теги для конкретных версий: `{VERSION}-data-json` (например, `1.21.4-data-json`)

### wiki.vg

Человекочитаемая документация протокола. Полезна для понимания структуры отдельных пакетов (какие поля, типы данных, порядок байтов). Не используется автоматически, но незаменима при написании `Decode()`/`Encode()` для новых пакетов.

---

## Решение типичных проблем

### `stringer: command not found`

```bash
go install golang.org/x/tools/cmd/stringer@latest
```

Убедитесь, что `$GOPATH/bin` (или `$HOME/go/bin`) в вашем `$PATH`.

### Генератор выдаёт `read protocol.json: no such file or directory`

Скрипт `update_version.sh` не был запущен, или путь в `protocolJSONPath` не совпадает с реальным расположением файла.

### `go build` выдаёт `undefined: packetid.SomePacketName`

Пакет был переименован или удалён в новой версии. Посмотрите новый `packetid.go` и найдите актуальное имя. Используйте grep:

```bash
grep -i "somepacket" internal/protocol/packetid/packetid.go
```

### `iota` значения не совпадают с ожидаемыми

Это нормально! При смене версии ID пакетов меняются. Весь смысл системы `packetid` в том, что правильные числа генерируются автоматически. Не сравнивайте числа вручную — доверяйте `protocol.json`.

### Файл protocol.json пустой или содержит ошибку

Проверьте, что версия существует в PrismarineJS:

```bash
curl -sI "https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/1.21.5/protocol.json"
```

Если ответ `404` — данные для этой версии ещё не добавлены.
