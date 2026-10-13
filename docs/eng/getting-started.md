# Installation and first connection

[Русский](../rus/getting-started.md) · [Documentation](README.md)

## Download a binary

Download the archive for your OS and CPU from [GitHub Releases](https://github.com/VitisMC/Vitis/releases). Choose a binary release rather than a source archive.

| Platform | Archive |
| --- | --- |
| Linux x86-64 | `vitis-linux-amd64.tar.gz` |
| Linux ARM64 | `vitis-linux-arm64.tar.gz` |
| Windows x86-64 | `vitis-windows-amd64.zip` |
| macOS Intel | `vitis-darwin-amd64.tar.gz` |
| macOS Apple Silicon | `vitis-darwin-arm64.tar.gz` |

Each release includes `SHA256SUMS` for the archives. Unpack the selected archive into a writable server directory; Unix archives preserve executable permissions. Neither Go nor Java is required. You need Internet access for the initial resource download and a Minecraft Java Edition **1.21.4** client. Bedrock and other protocol versions are unsupported.

## Start with one command

Open a terminal in the directory where you want the server files. On Linux/macOS:

```bash
./vitis
```

Windows PowerShell:

```powershell
.\vitis.exe
```

Vitis automatically creates missing files and prepares resources before opening the listener:

| Path, relative to the working directory | First-run behavior |
| --- | --- |
| `configs/vitis.yaml` | Write default settings: account authentication enabled, port 25565 |
| `configs/tab.yaml` | Write templates for the tab-list header and footer |
| `ops.json`, `whitelist.json`, `banned-players.json`, `banned-ips.json` | Create empty JSON arrays |
| `world/vanilla/1.21.4/` | Download the official JAR, verify size/SHA-1 and extract `data/`, `pack.mcmeta` and version identity |
| `world/` | Create and open the configured world |

Preparation uses a temporary directory. After validating the Minecraft version, protocol, DataVersion and datapack format, the server compiles the resources. The JAR and temporary files are removed; the prepared pack stays inside the world. Developer `reports/` are not needed. Existing files are preserved.

Expected startup includes `preparing server resources`, `server resources loaded` and `Vitis listening`. Connect to `127.0.0.1:25565` with an authenticated 1.21.4 client when running locally. Built-in defaults listen on `0.0.0.0:25565`; review [configuration](configuration.md) for deployment settings.

## Configuration and working directory

All relative paths are resolved against the **working directory**, not the directory containing the binary. To select another configuration:

Linux (Bash):

```bash
./vitis -config local.yaml
```

Windows (PowerShell 7.2+):

```powershell
.\vitis.exe -config local.yaml
```

A missing file selected by `-config` is created with default settings. Its `world.default_world_name` determines the world root and `<world>/vanilla/1.21.4/`; tab templates and access lists remain relative to the working directory. An existing configuration remains unchanged and is validated normally; unknown keys and invalid values fail startup.

The [production example](../../configs/vitis.example.yaml) uses authenticated Survival and built-in distances. Running a source build uses local `configs/vitis.yaml`, which may differ, unless another path is selected. Startup creates missing runtime files; they are excluded from Git.

## Stop and restart

Type `stop` in the console, or send SIGINT/SIGTERM. Wait for `Vitis shutdown complete` and a successful exit before copying world files. Restart using the same command, working directory and configuration.

A valid `<world>/vanilla/1.21.4/` pack is reused without contacting download servers. When it is missing, an existing `.mcdata/1.21.4/vanilla` pack is checked and copied into the world without modifying the source. If neither exists, Vitis downloads the official JAR. A damaged source also fails startup and is preserved. Online player authentication can still require Internet access. Existing worlds retain their saved metadata and need DataVersion 4189; read [storage and backups](storage.md) before opening an existing world.

## Preparation failures

Download requests have a two-minute timeout; total preparation is limited to five minutes. Cancellation, HTTP errors, wrong sizes/checksums or invalid archives fail startup without publishing a partial pack. Fix the cause and rerun the same command.

An existing damaged pack causes `prepare server resources failed` and is preserved. Inspect the error and keep a copy before moving the damaged vanilla directory out of the way and allowing a new download. Fix custom datapack errors separately. See [troubleshooting](troubleshooting.md).

## Build from source

For core development, install Go 1.23+ and Git. From the repository root:

Linux (Bash):

```bash
go build -o vitis ./cmd/vitis
./vitis -config local.yaml
```

Windows (PowerShell 7.2+):

```powershell
go build -o vitis.exe ./cmd/vitis
.\vitis.exe -config local.yaml
```

Resource preparation is identical to a release binary and needs no Java. Full code generation remains a separate [developer workflow](generators/README.md).

Sources: [first-run files](../../cmd/vitis/first_run.go), [runtime preparation](../../internal/data/vanilla/prepare.go), [downloads](../../internal/data/vanilla/download.go), [release workflow](../../.github/workflows/release.yml).
