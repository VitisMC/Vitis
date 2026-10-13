# Developer setup

[Русский](../rus/development.md) · [Documentation](README.md)

## Build and test

Use Go 1.23+ and Git. From the repository root:

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

Tests and compilation normally use committed generated Go data. The server prepares a missing vanilla pack automatically as described in [installation](getting-started.md), without Java. Full data regeneration still needs Java and additional tools.

Use package entrypoints: `go run ./cmd/vitis -config vitis.yaml`. `scripts/linux/run.sh` builds the complete server package and executes the binary, forwarding arguments and signals. Review the existing example YAML before using `make run`; missing resources are prepared automatically.

## Build and run scripts

Linux (Bash):

```bash
./scripts/linux/build.sh
./scripts/linux/run.sh -config configs/vitis.yaml
make verify
```

Windows (PowerShell **7.2+**):

```powershell
pwsh -NoProfile -File .\scripts\windows\build.ps1
pwsh -NoProfile -File .\scripts\windows\run.ps1 -config configs\vitis.yaml
pwsh -NoProfile -File .\scripts\windows\verify.ps1
```

`build.ps1` accepts an optional output path, such as `build.ps1 "build\vitis.exe"`. `run.ps1` forwards server arguments and console input; `stop` saves the world and exits. Both run from the repository root, matching the Bash scripts. `verify.ps1` runs the same tests, vet, race, fuzz and documentation checks as `make verify`, without Make. Race checks need cgo and a compatible C compiler; see [Windows generation prerequisites](generators/README.md). PowerShell scripts do not change execution policy.

Platform scripts live in `scripts/linux/` and `scripts/windows/`. The generator manifest and Go tooling tests in `scripts/` are shared.

## Repository map

| Area | Responsibility |
| --- | --- |
| `cmd/vitis` | Server composition, callbacks, startup and shutdown |
| `cmd/vitis-data` | Developer-only vanilla data generation, including reports, using Java |
| `cmd/vitis-worldgen-compare` | Saved vanilla chunk comparison |
| `cmd/vitis-doccheck` | Offline documentation checks |
| `internal/network`, `internal/protocol` | Transport and binary codec |
| `internal/session` | Protocol flow, player lifecycle and gameplay adapters |
| `internal/world` | Runtime worlds, simulation, chunk lifecycle and persistence |
| `internal/entity`, gameplay packages | Entity and mechanic logic |
| `internal/data/generated` | Committed generated data |
| `internal/data/generator` | Standalone generators, often with build-ignore tags |
| `internal/datapack` | Validated resource discovery and snapshot publication |

Read [architecture](architecture.md) before changing ownership or shutdown paths, and [extension guides](extending.md) before introducing a command/packet/mechanic.

## Contribution workflow

Follow [CONTRIBUTING](../../CONTRIBUTING.md): branch from `dev`, open PRs against `dev`, use existing branch prefixes and Conventional Commits. Check baseline failures before editing; do not overwrite someone else's uncommitted work.

Use idiomatic Go and the project's existing modular structure. Keep protocol parsing, gameplay and storage separate where needed. Add abstractions for concrete requirements.

New logic needs focused tests and edge cases. Concurrent code needs race coverage; wire codecs need malformed-input coverage. English Go documentation comments describe packages, types and functions; use gofmt. Prefer structured logging through `internal/logger`.

## Documentation changes

Update both languages when behavior changes. Every configuration leaf key must have one backticked first-cell table row in each configuration reference. Link to authoritative source files instead of copying long implementations.

`make docs-check` validates link destinations, page parity and configuration-key coverage. It does not validate translated meaning, anchors or external sites. Check the [test guide](testing.md) for full verification.
