# Protocol Overview

Vitis implements the Minecraft Java Edition protocol version **769** (1.21.4).

## Protocol States

The connection progresses through four states:

```
Handshake → Status (server list ping)
Handshake → Login → Configuration → Play
```

### Handshake

Single packet from client declaring intent (status or login) and protocol version.

### Status

Server list ping/pong. The server responds with version info, player count, MOTD, and optional favicon.

### Login

1. **LoginStart** — client sends username and UUID
2. **EncryptionRequest/Response** (online mode only) — RSA key exchange, shared secret, CFB8 cipher
3. **SetCompression** — enables zlib packet compression above threshold
4. **LoginSuccess** — server confirms login with UUID and skin properties
5. **LoginAcknowledged** — client confirms, transitions to Configuration

### Configuration

1. **PluginMessage** — server sends brand (`minecraft:brand` = "Vitis")
2. **KnownPacks** — client/server negotiate known data packs
3. **RegistryData** — server sends 78+ registries and 12 config registries (NBT-encoded)
4. **UpdateTags** — server sends tag groups (biome tags, item tags, etc.)
5. **FinishConfiguration** — transitions to Play

### Play

All gameplay packets. Key categories:

- **Position** — `SetPlayerPosition`, `SetPlayerPositionAndRotation`, `SyncPlayerPosition`
- **Chunks** — `ChunkData`, `UnloadChunk`, `ChunkBatchStart/Finished`
- **Entities** — `SpawnEntity`, `EntityPosition`, `EntityMetadata`, `EntityStatus`
- **Inventory** — `WindowClick`, `SetContainerContent`, `SetContainerSlot`
- **Combat** — `DamageEvent`, `HurtAnimation`, `DeathCombatEvent`, `EntityVelocity`
- **World** — `BlockUpdate`, `AcknowledgeBlockChange`, `UpdateTime`, `WorldEvent`
- **Chat** — `DisguisedChat`, `SystemChatMessage`, `ChatCommand`
- **Scoreboard** — `UpdateObjectives`, `UpdateScore`, `UpdateTeams`
- **Tab** — `PlayerInfoUpdate/Remove`, `TabHeaderFooter`, `TabComplete`
- **KeepAlive** — bidirectional heartbeat every 10 seconds

## Wire Format

All packets follow this structure:

```
[VarInt packet_length] [VarInt packet_id] [payload...]
```

With compression enabled:

```
[VarInt packet_length] [VarInt data_length] [zlib_compressed([VarInt packet_id] [payload...])]
```

If `data_length` is 0, the packet is uncompressed (below threshold).

### VarInt Encoding

Variable-length integer encoding (1–5 bytes). Each byte uses 7 data bits and 1 continuation bit.

```
Value:  0x00–0x7F      → 1 byte
Value:  0x80–0x3FFF     → 2 bytes
Value:  0x4000–0x1FFFFF → 3 bytes
...up to 5 bytes for int32
```

## Packet Registration

Packets are registered in `internal/protocol/states/` with version-aware and state-aware mappings:

```go
states.RegisterCore(registry, protocol.AnyVersion)
```

Each packet implements the `protocol.Packet` interface:

```go
type Packet interface {
    ID() int32
    Encode(buf *Buffer) error
    Decode(buf *Buffer) error
}
```

## 1.21.4 Specifics

- **Text Components** are encoded as **NBT**, not JSON strings (affects MOTD, chat, disconnect)
- **UpdateTime** includes a trailing `TickDayTime` boolean field
- **Registry data** uses the 1.21.4 datapack format with NBT-encoded entries
