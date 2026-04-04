# Packet ID System in Vitis

## What are packets in Minecraft

Minecraft uses a client-server architecture. The client (game) and server communicate over a TCP connection by exchanging **packets** — structured blocks of binary data.

Each packet consists of:

| Field | Type | Description |
|-------|------|-------------|
| Packet Length | VarInt | Total length of ID + data |
| Packet ID | VarInt | Numeric identifier of the packet type |
| Data | byte[] | Payload specific to each packet type |

**Packet ID** is a numeric identifier that tells the receiving side which packet has arrived and how to decode it. For example, `0x2C` in the Play state means the "Login (Play)" packet — the server sends it to the client when a player enters the world.

## Packet directions

Packets are divided into two directions:

- **Clientbound (S→C)** — sent by the server to the client (e.g. "update entity position", "load chunk")
- **Serverbound (C→S)** — sent by the client to the server (e.g. "player moved", "player dug a block")

Clientbound and serverbound packets have **independent** ID spaces: Packet ID `0x00` clientbound is a completely different packet from Packet ID `0x00` serverbound.

## Protocol states

The Minecraft protocol has 5 states (connection phases). Each state has its own set of packets with its own numbering:

| State | Description | Example packets |
|-------|-------------|-----------------|
| **Handshake** | First client packet, determines where to go next | `SetProtocol` (the only packet) |
| **Status** | Server information request (server list) | `PingStart`, `Ping`, `ServerInfo` |
| **Login** | Authentication and encryption | `LoginStart`, `EncryptionBegin`, `Success`, `Compress` |
| **Configuration** | Client setup (registries, tags, packs) | `RegistryData`, `Tags`, `SelectKnownPacks`, `FinishConfiguration` |
| **Play** | Main gameplay | `KeepAlive`, `Position`, `MapChunk`, `SpawnEntity` and ~200 more |

This means Packet ID `0x00` in Login is `Disconnect`, while in Play it's `SpawnEntity` (for clientbound). The state context determines which packet is implied.

## Why the Packet ID system exists in Vitis

### Problem: hardcoded numbers

Before the `packetid` system was introduced, each packet file contained a hardcoded constant:

```go
// Before — each file had its own magic constant
const chunkDataPacketID int32 = 0x28
const keepAlivePacketID int32 = 0x27
const unloadChunkPacketID int32 = 0x21  // ← and this ID was wrong!
```

Problems with this approach:

1. **Duplication** — each file declares its own constant, no single source of truth
2. **Errors** — easy to specify the wrong number (real example: `UnloadChunk` had ID `0x21` instead of the correct `0x22`)
3. **Updating** — when Minecraft version changes, dozens of files need manual checking and changing
4. **Readability** — numbers like `0x2C`, `0x47` say nothing when reading code

### Solution: centralized generation

The `packetid` system solves all these problems:

```go
// After — readable, typed, generated constants
func (p *ChunkDataAndUpdateLight) ID() int32 {
    return int32(packetid.ClientboundMapChunk)
}
```

## Architecture

### Data source

The authoritative source for Packet IDs for each Minecraft version is the `protocol.json` file from the [PrismarineJS/minecraft-data](https://github.com/PrismarineJS/minecraft-data) project. This community-maintained project contains machine-readable protocol data for all versions.

The file is downloaded by `scripts/update_version.sh` into `.mcdata/1.21.4/protocol.json` (it's in `.gitignore` — not committed to the repository).

URL format:
```
https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/{VERSION}/protocol.json
```

### Generator

File: `internal/protocol/packetid/generator/main.go`

The generator is a Go program that:

1. Finds the project root (looks for `go.mod`)
2. Reads `protocol.json`
3. Parses the JSON structure, extracting `hex_id → packet_name` mappings for each state and direction
4. Sorts packets by ID (for correct `iota` operation)
5. Generates Go code with `const` blocks
6. Formats via `go/format`
7. Writes the result to `internal/protocol/packetid/packetid.go`

### protocol.json structure

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

The generator extracts the `mappings` object from each `state.direction.types.packet`.

### Generated code

File: `internal/protocol/packetid/packetid.go`

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

Key points:

- **Typing**: `ClientboundPacketID` and `ServerboundPacketID` are separate types. The compiler won't allow accidentally passing a clientbound ID where serverbound is expected
- **`iota`**: each `const` block starts with `iota = 0`, matching packet numbering from `0x00`. The order of constants in the block **must** match the Packet ID order
- **Naming**: `{Direction}{State}{PascalCaseName}`, e.g. `ClientboundConfigRegistryData`, `ServerboundLoginLoginStart`

### Stringer

The `//go:generate stringer` directives in `packetid.go` generate files:
- `clientboundpacketid_string.go`
- `serverboundpacketid_string.go`

They add a `String()` method to the types, enabling human-readable names in logs:

```go
fmt.Println(packetid.ClientboundMapChunk)
// Output: "ClientboundMapChunk" (instead of just "40")
```

## File structure

```
internal/protocol/packetid/
├── generator/
│   └── main.go                          # Generator (reads protocol.json → generates packetid.go)
├── packetid.go                          # Generated file with constants (DO NOT EDIT MANUALLY)
├── clientboundpacketid_string.go        # Generated by stringer (DO NOT EDIT MANUALLY)
└── serverboundpacketid_string.go        # Generated by stringer (DO NOT EDIT MANUALLY)

scripts/
└── update_version.sh                    # Downloads protocol.json (among other data)

.mcdata/1.21.4/
└── protocol.json                        # Downloaded protocol data (in .gitignore)
```

## How Packet IDs are used in code

Each packet implements the `protocol.Packet` interface with the `ID() int32` method. Here's how it looks:

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

Packet registration in the registry (`internal/protocol/states/*.go` files) uses the `ID()` method to map packets by number.

## Statistics

In the current version (1.21.4, protocol version 769):

| State | Clientbound | Serverbound |
|-------|-------------|-------------|
| Login | 6 | 5 |
| Status | 2 | 2 |
| Configuration | 14 | 8 |
| Play | 131 | 61 |
| **Total** | **153** | **76** |

Total of **229 unique packets** (excluding Handshake).

## Bugs fixed during migration

During the migration to the `packetid` system, the following incorrect IDs were discovered and automatically fixed:

| Packet | Was (incorrect) | Became (correct) |
|--------|-----------------|-------------------|
| `UnloadChunk` | `0x21` | `0x22` |
| `SetHeadRotation` | `0x4E` | `0x4D` |
| `TeleportEntity` | `0x70` | `0x77` |
| `UpdateEntityPosition` | `0x30` | `0x2F` |
| `UpdateEntityPositionAndRotation` | `0x31` | `0x30` |

This demonstrates the main advantage of generating from an authoritative source — **impossible to have ID errors**.
