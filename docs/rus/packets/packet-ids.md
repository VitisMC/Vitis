# Система Packet ID в Vitis

## Что такое пакеты в Minecraft

Minecraft использует клиент-серверную архитектуру. Клиент (игра) и сервер общаются через TCP-соединение, обмениваясь **пакетами** — структурированными блоками бинарных данных.

Каждый пакет состоит из:

| Поле | Тип | Описание |
|------|-----|----------|
| Длина пакета | VarInt | Общая длина ID + данных |
| Packet ID | VarInt | Числовой идентификатор типа пакета |
| Данные | byte[] | Полезная нагрузка, специфичная для каждого типа пакета |

**Packet ID** — это числовой идентификатор, который говорит принимающей стороне, какой именно пакет пришёл и как его декодировать. Например, `0x2C` в состоянии Play означает пакет "Login (Play)" — сервер отправляет его клиенту при входе игрока в мир.

## Направления пакетов

Пакеты делятся на два направления:

- **Clientbound (S→C)** — отправляются сервером клиенту (например, «обнови позицию сущности», «загрузи чанк»)
- **Serverbound (C→S)** — отправляются клиентом серверу (например, «игрок переместился», «игрок копнул блок»)

У clientbound и serverbound пакетов **независимые** пространства ID: Packet ID `0x00` clientbound — это совершенно другой пакет, чем Packet ID `0x00` serverbound.

## Состояния протокола

Протокол Minecraft имеет 5 состояний (фаз подключения). В каждом состоянии свой набор пакетов со своей нумерацией:

| Состояние | Описание | Пример пакетов |
|-----------|----------|----------------|
| **Handshake** | Первый пакет клиента, определяет куда переходить дальше | `SetProtocol` (единственный пакет) |
| **Status** | Запрос информации о сервере (список серверов) | `PingStart`, `Ping`, `ServerInfo` |
| **Login** | Аутентификация и шифрование | `LoginStart`, `EncryptionBegin`, `Success`, `Compress` |
| **Configuration** | Настройка клиента (реестры, теги, паки) | `RegistryData`, `Tags`, `SelectKnownPacks`, `FinishConfiguration` |
| **Play** | Основной игровой процесс | `KeepAlive`, `Position`, `MapChunk`, `SpawnEntity` и ещё ~200 пакетов |

Это значит, что Packet ID `0x00` в Login — это `Disconnect`, а в Play — это `SpawnEntity` (для clientbound). Контекст состояния определяет, какой именно пакет подразумевается.

## Зачем нужна система Packet ID в Vitis

### Проблема: захардкоженные числа

До введения системы `packetid` каждый файл пакета содержал захардкоженную константу:

```go
// Было — в каждом файле своя магическая константа
const chunkDataPacketID int32 = 0x28
const keepAlivePacketID int32 = 0x27
const unloadChunkPacketID int32 = 0x21  // ← ещё и неправильный ID!
```

Проблемы такого подхода:

1. **Дублирование** — каждый файл объявляет свою константу, нет единого источника правды
2. **Ошибки** — легко указать неправильное число (реальный пример: `UnloadChunk` имел ID `0x21` вместо правильного `0x22`)
3. **Обновление** — при смене версии Minecraft нужно вручную проверять и менять десятки файлов
4. **Нечитаемость** — числа `0x2C`, `0x47` ничего не говорят при чтении кода

### Решение: централизованная генерация

Система `packetid` решает все эти проблемы:

```go
// Стало — читаемые, типизированные, сгенерированные константы
func (p *ChunkDataAndUpdateLight) ID() int32 {
    return int32(packetid.ClientboundMapChunk)
}
```

## Архитектура системы

### Источник данных

Авторитетный источник Packet ID для каждой версии Minecraft — файл `protocol.json` из проекта [PrismarineJS/minecraft-data](https://github.com/PrismarineJS/minecraft-data). Этот проект поддерживается сообществом и содержит машиночитаемые данные протокола для всех версий.

Файл скачивается скриптом `scripts/update_version.sh` в `.mcdata/1.21.4/protocol.json` (он в `.gitignore` — не коммитится в репозиторий).

URL формат:
```
https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/{VERSION}/protocol.json
```

### Генератор

Файл: `internal/protocol/packetid/generator/main.go`

Генератор — это Go-программа, которая:

1. Находит корень проекта (ищет `go.mod`)
2. Читает `protocol.json`
3. Парсит JSON-структуру, извлекая маппинги `hex_id → packet_name` для каждого состояния и направления
4. Сортирует пакеты по ID (для корректной работы `iota`)
5. Генерирует Go-код с `const` блоками
6. Форматирует через `go/format`
7. Записывает результат в `internal/protocol/packetid/packetid.go`

### Структура protocol.json

```json
{
  "login": {
    "toClient": {
      "types": {
        "packet": ["container", [
          {
            "name": "name",
            "type": ["mapper", {
              "mappings": {
                "0x00": "disconnect",
                "0x01": "encryption_begin",
                "0x02": "success"
              }
            }]
          }
        ]]
      }
    },
    "toServer": { ... }
  },
  "status": { ... },
  "configuration": { ... },
  "play": { ... }
}
```

Генератор извлекает объект `mappings` из каждого `state.direction.types.packet`.

### Сгенерированный код

Файл: `internal/protocol/packetid/packetid.go`

```go
type (
    ClientboundPacketID int32
    ServerboundPacketID int32
)

// Clientbound Login
const (
    ClientboundLoginDisconnect      ClientboundPacketID = iota  // 0x00
    ClientboundLoginEncryptionBegin                             // 0x01
    ClientboundLoginSuccess                                     // 0x02
    ClientboundLoginCompress                                    // 0x03
    ...
)

// Serverbound Play
const (
    ServerboundTeleportConfirm ServerboundPacketID = iota  // 0x00
    ServerboundQueryBlockNbt                               // 0x01
    ...
)
```

Ключевые моменты:

- **Типизация**: `ClientboundPacketID` и `ServerboundPacketID` — отдельные типы. Компилятор не позволит случайно передать clientbound ID туда, где ожидается serverbound
- **`iota`**: каждый блок `const` начинается с `iota = 0`, что соответствует нумерации пакетов с `0x00`. Порядок констант в блоке **обязан** совпадать с порядком Packet ID
- **Именование**: `{Direction}{State}{PascalCaseName}`, например `ClientboundConfigRegistryData`, `ServerboundLoginLoginStart`

### Stringer

Директивы `//go:generate stringer` в `packetid.go` генерируют файлы:
- `clientboundpacketid_string.go`
- `serverboundpacketid_string.go`

Они добавляют метод `String()` к типам, что позволяет выводить человекочитаемые имена в логах:

```go
fmt.Println(packetid.ClientboundMapChunk)
// Вывод: "ClientboundMapChunk" (вместо просто "40")
```

## Структура файлов

```
internal/protocol/packetid/
├── generator/
│   └── main.go                          # Генератор (читает protocol.json → генерирует packetid.go)
├── packetid.go                          # Сгенерированный файл с константами (НЕ РЕДАКТИРОВАТЬ ВРУЧНУЮ)
├── clientboundpacketid_string.go        # Сгенерирован stringer (НЕ РЕДАКТИРОВАТЬ ВРУЧНУЮ)
└── serverboundpacketid_string.go        # Сгенерирован stringer (НЕ РЕДАКТИРОВАТЬ ВРУЧНУЮ)

scripts/
└── update_version.sh                    # Скачивает protocol.json (среди прочих данных)

.mcdata/1.21.4/
└── protocol.json                        # Скачанные данные протокола (в .gitignore)
```

## Как используются Packet ID в коде

Каждый пакет реализует интерфейс `protocol.Packet` с методом `ID() int32`. Вот как выглядит использование:

```go
package play

import (
    "vitis/internal/protocol"
    "vitis/internal/protocol/packetid"
)

type KeepAliveClientbound struct {
    Value int64
}

func (p *KeepAliveClientbound) ID() int32 {
    return int32(packetid.ClientboundKeepAlive)
}
```

Регистрация пакетов в реестре (файлы `internal/protocol/states/*.go`) использует метод `ID()` для маппинга пакетов по номерам.

## Статистика

В текущей версии (1.21.4, protocol version 769):

| Состояние | Clientbound | Serverbound |
|-----------|-------------|-------------|
| Login | 6 | 5 |
| Status | 2 | 2 |
| Configuration | 14 | 8 |
| Play | 131 | 61 |
| **Итого** | **153** | **76** |

Всего **229 уникальных пакетов** (не считая Handshake).

## Исправленные ошибки при миграции

При переходе на систему `packetid` были обнаружены и автоматически исправлены следующие неправильные ID:

| Пакет | Был (неверный) | Стал (правильный) |
|-------|----------------|-------------------|
| `UnloadChunk` | `0x21` | `0x22` |
| `SetHeadRotation` | `0x4E` | `0x4D` |
| `TeleportEntity` | `0x70` | `0x77` |
| `UpdateEntityPosition` | `0x30` | `0x2F` |
| `UpdateEntityPositionAndRotation` | `0x31` | `0x30` |

Это демонстрирует главное преимущество генерации из авторитетного источника — **невозможность ошибки в ID**.
