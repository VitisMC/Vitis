# Vitis

[Русский](README_RU.md) · [Documentation](docs/eng/README.md) · [Contributing](CONTRIBUTING.md)

Minecraft Java Edition server software written in Go.

**Experimental:** gameplay and vanilla compatibility are incomplete. Read the [features and limitations](docs/eng/status.md) before choosing Vitis for a world you intend to keep.

| Compatibility | Version |
| --- | --- |
| Client | Minecraft Java Edition **1.21.4** |
| Wire protocol | **769** |
| World DataVersion | **4189** |
| Build from source | Go **1.23+** |
| License | [GPLv3](LICENSE) |

## Start here

Download the archive for your OS and CPU from [GitHub Releases](https://github.com/VitisMC/Vitis/releases), unpack it into your server directory, and run a command there.

Linux/macOS:

```bash
./vitis
```

Windows PowerShell:

```powershell
.\vitis.exe
```

On the first run, Vitis creates missing configuration/access files, downloads the official Minecraft 1.21.4 server JAR, verifies it and extracts vanilla resources using Go. Go, Java and source files are not needed. Existing files are preserved; subsequent starts reuse the local pack.

Connect with a Java Edition 1.21.4 client. Use `stop` in the console to save and shut down. See [installation](docs/eng/getting-started.md) for platform selection, generated files and troubleshooting, or [developer setup](docs/eng/development.md) to build from source.

The runtime vanilla pack is stored in `<world>/vanilla/1.21.4/`, with `world` selected by configuration. User datapacks belong in `<world>/datapacks/`.

## Documentation

- **Run a server:** [installation](docs/eng/getting-started.md), [configuration](docs/eng/configuration.md), [commands](docs/eng/commands.md), [operations](docs/eng/operations.md), [troubleshooting](docs/eng/troubleshooting.md).
- **Develop the core:** [development setup](docs/eng/development.md), [architecture](docs/eng/architecture.md), [protocol](docs/eng/protocol.md), [world runtime](docs/eng/world.md), [extension guides](docs/eng/extending.md).
- **Project status:** [features and limitations](docs/eng/status.md), [development directions](docs/eng/roadmap.md).
- **Maintain Minecraft data:** [resources and generators](docs/eng/generators/README.md), [version upgrades](docs/eng/generators/version-upgrade.md).

Bukkit/Paper plugins and Forge/Fabric mods are not supported.
