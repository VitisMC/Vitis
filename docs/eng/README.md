# Vitis documentation

[Русский](../rus/README.md) · [Project](../../README.md)

Minecraft Java Edition 1.21.4, protocol 769, DataVersion 4189. Vitis is experimental. These guides describe the current source.

## Run a server

- [Installation and first connection](getting-started.md).
- [Configuration reference](configuration.md).
- [Commands and operator permissions](commands.md).
- [Operation, access, RCON and Query](operations.md).
- [Datapacks and tab list](resources.md).
- [Chunk pregeneration](pregeneration.md).
- [World storage, backups and recovery](storage.md).
- [Troubleshooting](troubleshooting.md).

## Develop the core

- [Developer setup and contribution workflow](development.md).
- [Architecture, ownership and lifecycle](architecture.md).
- [Network transport and protocol flows](protocol.md).
- [Tick, chunks, streaming and world generation](world.md).
- [Gameplay systems](gameplay.md).
- [Add a command, packet or mechanic](extending.md).
- [Tests, fuzzing and performance diagnosis](testing.md).
- [Resources and code generators](generators/README.md).
- [Minecraft data sources](generators/data-sources.md).
- [Minecraft version upgrades](generators/version-upgrade.md).
- [Packet IDs and registration](packets/packet-ids.md).
- [Registries, tags and configuration sync](registry/overview.md).

## Project status

- [Features and limitations](status.md).
- [Development directions](roadmap.md).

Start with installation to connect a client. To change the core, set up development first, then read the architecture and the relevant subsystem guide.

## Documentation changes

Update both languages when behavior changes. The configuration reference has one table row per YAML leaf key. Run:

Linux (Bash):

```bash
make docs-check
```

Windows (PowerShell 7.2+):

```powershell
go run ./cmd/vitis-doccheck
```

The checker runs offline and validates local link targets, EN/RU page pairs and configuration-key coverage. Review section links, external sites, translations, defaults and examples manually.
