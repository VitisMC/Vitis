# Обзор протокола

Vitis реализует протокол Minecraft Java Edition версии **769** (1.21.4).

## Состояния протокола

Соединение проходит через четыре состояния:

```
Handshake → Status (пинг списка серверов)
Handshake → Login → Configuration → Play
```

### Handshake

Один пакет от клиента, объявляющий намерение (status или login) и версию протокола.

### Status

Пинг/понг списка серверов. Сервер отвечает информацией о версии, количестве игроков, MOTD и опциональной иконкой.

### Login

1. **LoginStart** — клиент отправляет имя пользователя и UUID
2. **EncryptionRequest/Response** (только онлайн-режим) — обмен RSA-ключами, общий секрет, шифр CFB8
3. **SetCompression** — включение zlib-сжатия пакетов выше порога
4. **LoginSuccess** — сервер подтверждает логин с UUID и свойствами скина
5. **LoginAcknowledged** — клиент подтверждает, переход в Configuration

### Configuration

1. **PluginMessage** — сервер отправляет бренд (`minecraft:brand` = "Vitis")
2. **KnownPacks** — клиент и сервер согласовывают известные пакеты данных
3. **RegistryData** — сервер отправляет 78+ реестров и 12 конфигурационных реестров (NBT-кодировка)
4. **UpdateTags** — сервер отправляет группы тегов (теги биомов, предметов и т.д.)
5. **FinishConfiguration** — переход в Play

### Play

Все игровые пакеты. Основные категории:

- **Позиция** — `SetPlayerPosition`, `SetPlayerPositionAndRotation`, `SyncPlayerPosition`
- **Чанки** — `ChunkData`, `UnloadChunk`, `ChunkBatchStart/Finished`
- **Сущности** — `SpawnEntity`, `EntityPosition`, `EntityMetadata`, `EntityStatus`
- **Инвентарь** — `WindowClick`, `SetContainerContent`, `SetContainerSlot`
- **Бой** — `DamageEvent`, `HurtAnimation`, `DeathCombatEvent`, `EntityVelocity`
- **Мир** — `BlockUpdate`, `AcknowledgeBlockChange`, `UpdateTime`, `WorldEvent`
- **Чат** — `DisguisedChat`, `SystemChatMessage`, `ChatCommand`
- **Скорборд** — `UpdateObjectives`, `UpdateScore`, `UpdateTeams`
- **Таб** — `PlayerInfoUpdate/Remove`, `TabHeaderFooter`, `TabComplete`
- **KeepAlive** — двунаправленный хартбит каждые 10 секунд

## Формат на проводе

Все пакеты следуют структуре:

```
[VarInt packet_length] [VarInt packet_id] [payload...]
```

С включённым сжатием:

```
[VarInt packet_length] [VarInt data_length] [zlib_compressed([VarInt packet_id] [payload...])]
```

Если `data_length` равен 0, пакет не сжат (ниже порога).

### Кодирование VarInt

Кодирование целого числа переменной длины (1–5 байт). Каждый байт использует 7 бит данных и 1 бит продолжения.

```
Значение: 0x00–0x7F       → 1 байт
Значение: 0x80–0x3FFF     → 2 байта
Значение: 0x4000–0x1FFFFF → 3 байта
...до 5 байт для int32
```

## Регистрация пакетов

Пакеты регистрируются в `internal/protocol/states/` с маппингами, учитывающими версию и состояние:

```go
states.RegisterCore(registry, protocol.AnyVersion)
```

Каждый пакет реализует интерфейс `protocol.Packet`:

```go
type Packet interface {
    ID() int32
    Encode(buf *Buffer) error
    Decode(buf *Buffer) error
}
```

## Особенности 1.21.4

- **Текстовые компоненты** кодируются как **NBT**, а не JSON-строки (затрагивает MOTD, чат, дисконнект)
- **UpdateTime** включает завершающее булево поле `TickDayTime`
- **Данные реестров** используют формат датапаков 1.21.4 с NBT-кодированными записями
