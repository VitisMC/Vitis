# Updating Packet IDs When Upgrading to a New Minecraft Version

This step-by-step guide describes how to update the Packet ID system in Vitis when a new Minecraft version is released. Follow the instructions in order.

---

## Prerequisites

- Go 1.22+ installed
- `curl` installed
- `stringer` installed:
  ```bash
  go install golang.org/x/tools/cmd/stringer@latest
  ```
- Internet access for downloading files

---

## Step 1. Find the new version identifier

Go to the PrismarineJS/minecraft-data repository on GitHub:

```
https://github.com/PrismarineJS/minecraft-data/tree/master/data/pc
```

Find the folder for the desired version. For example:
- `1.21.4` — current Vitis version
- `1.21.5` — hypothetical new version

**Important:** the version must already be present in PrismarineJS. If it's not there — the protocol data hasn't been documented by the community yet, and you'll need to wait.

Make sure the new version folder contains a `protocol.json` file:

```
https://github.com/PrismarineJS/minecraft-data/blob/master/data/pc/1.21.5/protocol.json
```

---

## Step 2. Update the download script

Open `scripts/update_version.sh`.

The script accepts the Minecraft version as a command-line argument and downloads `protocol.json` automatically:

```bash
./scripts/update_version.sh 1.21.5
```

The file will be saved to `.mcdata/1.21.5/protocol.json`.

---

## Step 3. Download protocol.json

Run the script from the project root:

```bash
./scripts/update_version.sh 1.21.5
```

Expected output (Step 1 of the script):
```
=== Step 1: Download PrismarineJS data ===
  protocol.json                 ✓ (XXXXX bytes)
```

**Verify** the file was downloaded and is not empty:

```bash
head -c 200 .mcdata/1.21.5/protocol.json
```

You should see the beginning of a JSON file. If the file is empty or contains an HTML error — check the URL and data availability in the PrismarineJS repository.

---

## Step 4. Update the path in the generator

Open `internal/protocol/packetid/generator/main.go`.

Find the `protocolJSONPath` constant near the beginning of the file (~line 15):

```go
// Was:
const protocolJSONPath = ".mcdata/1.21.4/protocol.json"

// Becomes:
const protocolJSONPath = ".mcdata/1.21.5/protocol.json"
```

This is the only place you need to change in the generator.

---

## Step 5. Run the generator

From the project root, execute:

```bash
go run ./internal/protocol/packetid/generator/
```

Expected output:
```
Generated /home/.../internal/protocol/packetid/packetid.go (XXXX bytes)
```

**What happened:**
- The generator read the new `protocol.json`
- Extracted all packet_id → packet_name mappings for all states and directions
- Generated a new `internal/protocol/packetid/packetid.go` with updated constants

---

## Step 6. Regenerate stringer files

From the `internal/protocol/packetid/` directory, execute:

```bash
go generate ./internal/protocol/packetid/
```

This updates two files:
- `clientboundpacketid_string.go`
- `serverboundpacketid_string.go`

If the command fails with `stringer: command not found`, install it:

```bash
go install golang.org/x/tools/cmd/stringer@latest
```

Then re-run `go generate`.

---

## Step 7. Review changes in packetid.go

Look at the diff to understand what changed:

```bash
git diff internal/protocol/packetid/packetid.go
```

Typical changes when upgrading versions:

1. **New packets added** — new constants appeared
2. **Packets removed** — some constants disappeared
3. **Packets renamed** — names changed (rare)
4. **Order changed (IDs)** — packets shifted, `iota` values changed

**Important:** since `iota` is used, numeric ID values are recalculated automatically. You don't need to manually verify numbers — the generator takes them from the authoritative source.

---

## Step 8. Update packet code

### 8.1. Check for compilation errors

```bash
go build ./internal/protocol/...
```

If the build succeeds — all existing references to `packetid.*` constants are valid.

If there are errors like:

```
packetid.ClientboundSomeOldPacket undefined
```

This means the packet was **removed or renamed** in the new version. You need to:

1. Find the file using this constant
2. Check in the new `protocol.json` what this packet is now called
3. Update the reference to the new name

### 8.2. Add new packets (if needed)

If the new version introduced new packets that Vitis should handle:

1. Create a packet file in the appropriate directory (`internal/protocol/packets/{state}/`)
2. Implement the `protocol.Packet` interface with `ID()`, `Decode()`, `Encode()` methods
3. In the `ID()` method, use the new constant from `packetid`
4. Register the packet in `internal/protocol/states/{state}.go`

### 8.3. Handle removed packets

If a packet was removed from the protocol:

1. Delete the packet file
2. Remove its registration from `internal/protocol/states/{state}.go`
3. Remove all references to this packet in handlers

---

## Step 9. Run tests

```bash
go test ./internal/protocol/...
```

All tests should pass. If a test fails — it's likely checking a specific numeric ID that has changed. Update the expected value in the test.

---

## Step 10. Update other version dependencies

Check and update the protocol version in other project locations:

```bash
# Find all mentions of the old version
grep -r "1.21.4" --include="*.go" --include="*.yaml" --include="*.json" --include="*.sh"
grep -r "769" --include="*.go"  # 769 — protocol version for 1.21.4
```

Places that usually need updating:

- `configs/vitis.yaml` — if the version is specified there
- `.mcdata/1.21.4/version.json` — download the new `version.json`
- `scripts/update_version.sh` — version-specific arrays
- Registries (`internal/registry/`) — may require regeneration
- Code in `internal/session/` — if protocol version is checked

---

## Step 11. Final verification

```bash
# Full build
go build ./internal/...

# All tests
go test ./internal/...

# Integration tests (if any)
go test ./test/...
```

---

## Quick cheat sheet

For a quick update — minimal steps:

```bash
# 1. Run the full update script
./scripts/update_version.sh 1.21.5

# 2. Update path in generator/main.go (const protocolJSONPath)

# 3. Regenerate
go run ./internal/protocol/packetid/generator/
go generate ./internal/protocol/packetid/

# 4. Build and verify
go build ./internal/protocol/...
go test ./internal/protocol/...
```

---

## Where to get files — source reference

| What | Where | URL |
|------|-------|-----|
| **protocol.json** (Packet IDs, packet names) | PrismarineJS/minecraft-data | `https://github.com/PrismarineJS/minecraft-data/tree/master/data/pc/{VERSION}` |
| **Registries** (blocks, entities, biomes, etc.) | misode/mcmeta | `https://github.com/misode/mcmeta` (tag `{VERSION}-data-json`) |
| **Protocol documentation** (human-readable) | wiki.vg | `https://minecraft.wiki/w/Minecraft_Wiki:Projects/wiki.vg_merge/Protocol?oldid=2938097` |
| **Protocol version number** | wiki.vg | `https://minecraft.wiki/w/Minecraft_Wiki:Projects/wiki.vg_merge/` |

### PrismarineJS/minecraft-data

Primary source for Packet IDs. Contains `protocol.json` for each Minecraft version.

- Repository: https://github.com/PrismarineJS/minecraft-data
- Data for a specific version: `data/pc/{VERSION}/protocol.json`
- Raw URL for downloading:
  ```
  https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/{VERSION}/protocol.json
  ```

### misode/mcmeta

Registry data source. Used by `scripts/update_version.sh`.

- Repository: https://github.com/misode/mcmeta
- Tags for specific versions: `{VERSION}-data-json` (e.g. `1.21.4-data-json`)

### wiki.vg

Human-readable protocol documentation. Useful for understanding the structure of individual packets (fields, data types, byte order). Not used automatically, but invaluable when writing `Decode()`/`Encode()` for new packets.

---

## Solving common problems

### `stringer: command not found`

```bash
go install golang.org/x/tools/cmd/stringer@latest
```

Make sure `$GOPATH/bin` (or `$HOME/go/bin`) is in your `$PATH`.

### Generator outputs `read protocol.json: no such file or directory`

The `update_version.sh` script wasn't run, or the path in `protocolJSONPath` doesn't match the actual file location.

### `go build` outputs `undefined: packetid.SomePacketName`

The packet was renamed or removed in the new version. Look at the new `packetid.go` and find the current name. Use grep:

```bash
grep -i "somepacket" internal/protocol/packetid/packetid.go
```

### `iota` values don't match expectations

This is normal! When changing versions, packet IDs change. The whole point of the `packetid` system is that correct numbers are generated automatically. Don't compare numbers manually — trust `protocol.json`.

### protocol.json file is empty or contains an error

Check that the version exists in PrismarineJS:

```bash
curl -sI "https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/1.21.5/protocol.json"
```

If the response is `404` — the data for this version hasn't been added yet.
