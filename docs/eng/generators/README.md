# Resources and code generators

[Русский](../../rus/generators/README.md) · [Documentation](../README.md)

## Running the server and generating data

| Task | Command / tool | Requirements |
| --- | --- | --- |
| Run a server | `./vitis` / `.\vitis.exe` | Binary; Internet only when no reusable local pack exists |
| Regenerate development data on Linux | `make generate` | Go 1.23+, JDK 21, Git, Make, Bash 4+ and GNU utilities |
| Regenerate development data on Windows | `pwsh -NoProfile -File .\scripts\windows\update_version.ps1` | Go 1.23+, JDK 21, Git, PowerShell 7.2+ and a compatible C compiler |
| Generate developer reports only | `go run ./cmd/vitis-data` | Go, compatible Java runtime and verified official server JAR |

Normal startup prepares `<world>/vanilla/1.21.4/` without Java. It checks and copies an existing `.mcdata/1.21.4/vanilla` when available; otherwise it extracts resources from the official JAR. The developer pipeline keeps its inputs and reports in `.mcdata/<version>/`. Building and testing committed Go outputs does not require full regeneration.

## Generate after cloning

From the repository root, with `java` and `javac` from **JDK 21** on PATH:

Linux (Bash):

```bash
make generate
# Explicit version (the default is 1.21.4):
make generate MC_VERSION=1.21.4
```

`MC_VERSION` selects the Minecraft inputs for the whole pipeline; it can also be set in the environment. A command-line assignment takes precedence. Vitis currently supports Minecraft 1.21.4; selecting another version requires adapting generators and runtime contracts as described in [version upgrades](version-upgrade.md). `build`, `run`, `test` and `verify` use the current source and compiled tables; `MC_VERSION` does not switch their protocol or runtime version.

Expect downloads, remapping/decompilation, Java report generation, all Go generators and `make verify`. Completion ends with `data generation completed for <version>`. The command rewrites generated code and reference fixtures; inspect the complete diff.

The Bash updater checks `curl`, `jq`, `sha1sum`, `sha256sum`, `java`, `javac`, `go`, `find`, `sort`, `xargs`, `awk`, `sed`, `cmp`, `cp`, `make` and `git`, plus standard shell/file utilities. GNU `sort -z`, `find -print0`, `xargs -0` and checksum tools are required. On macOS install Bash 4+ and GNU utilities and expose their GNU commands on PATH. No Gradle or manual Minecraft source extraction is required.

## Native Windows generation

Use Windows amd64, PowerShell **7.2+**, Go 1.23+, **JDK 21**, Git and a C compiler compatible with Go's mandatory race checks. Native generation does not require WSL, Bash, Make, curl, jq or GNU utilities:

```powershell
pwsh -NoProfile -File .\scripts\windows\update_version.ps1 -Version 1.21.4
```

Without `-Version`, the script uses `MC_VERSION` from the environment when set, otherwise 1.21.4. Paths, pinned inputs, generator order, cache checks and rollback match the Bash pipeline. The shared [manifest](../../../scripts/generation_manifest.json) defines tool versions/hashes, generators and generated outputs. Both platforms create LF-delimited checksum inventories and source locks; JDK-version or artifact differences invalidate the cache safely. After generation, [verify.ps1](../../../scripts/windows/verify.ps1) runs all mandatory checks.

Go race checks require `CGO_ENABLED=1` and, on Windows, a C compiler with mingw-w64 runtime libraries **8+**. The updater checks these dependencies before downloading. See the [official Go requirements](https://go.dev/doc/articles/race_detector#Requirements). Windows PowerShell 5.1 is not supported; invoke `pwsh` rather than `powershell.exe`.

## Files and stages

| Path | Role |
| --- | --- |
| `.mc-decompiled/downloads/<version>/` | Verified official server JAR and Mojang mappings |
| `.mc-decompiled/downloads/tools/` | Pinned MinecraftDecompiler 3.3.2 and Vineflower 1.9.3, verified with SHA-256 |
| `.mc-decompiled/<version>-remapped.jar` | Named runtime for Java exporters |
| `.mc-decompiled/<version>-decompiled/` | Java source inputs for source-based generators |
| `.mc-decompiled/<version>.sources.lock`, `.sources.sha256` | Source/tool identity and inventory of the prepared artifacts |
| `.mcdata/<version>/` | Official resources/reports, PrismarineJS inputs, ancillary copies, source lock and checksums |
| `internal/data/generated/` and worldgen outputs | Generated Go tables/evaluators and reference fixtures |
| `<world>/vanilla/<version>/` | Resources consumed by the running server |

The [updater](../../../scripts/linux/update_version.sh) verifies official hashes and sizes, downloads pinned tools, and prepares named inputs in temporary directories. It uses the same toolchain for source parsing and remapped JVM exporters. Verified named artifacts are reused only when the input/tool fingerprint and complete checksum inventory match. An old source directory without that provenance is rebuilt safely.

PrismarineJS data is selected at a pinned commit, with inherited file locations resolved through that commit's `dataPaths.json`. An existing `source.lock` supplies the commit; a clean 1.21.4 checkout starts from the commit pinned in the updater. The official data generator supplies reports; embedded JAR resources supply files it omits.

Generators run in an explicit order. Single-file programs with build-ignore tags run by filename; the multi-file worldgen generator runs as a package with a versioned resource path. Adding a generator requires updating the command and output inventories.

## Individual generators

With inputs already prepared:

Linux (Bash):

```bash
go run internal/data/generator/registry/main.go -version 1.21.4
go run ./internal/data/generator/worldgen -version 1.21.4
```

Windows (PowerShell 7.2+):

```powershell
go run internal/data/generator/registry/main.go -version 1.21.4
go run ./internal/data/generator/worldgen -version 1.21.4
```

Standalone generators, `vitis-data` and `vitis-worldgen-compare` accept `-version`. Worldgen defaults to `.mcdata/<version>/vanilla/data/minecraft/worldgen`; comparison defaults to `.mcdata/<version>/vanilla`. An explicit `-resources` overrides those defaults. For `vitis-data`, `-version` selects the default JAR/output paths; explicit `-jar` and `-output` override those paths. `scripts/linux/update_version.sh <version>` remains available directly. Standalone commands use their own `-version` flag rather than reading `MC_VERSION`.

`vitis-data -unpack -version 1.21.4 -jar <verified-jar> -output <new-directory>` extracts the nested server JAR and libraries for the updater, checking their contents and sizes. Normal mode generates reports.

## Failures and recovery

Before publication, the updater snapshots all declared generated outputs, reference fixtures, versioned data and any replaced named-source artifacts. A failed command, failed check, SIGINT or SIGTERM stops the active command group and restores the previous files, including uncommitted contents and previously absent outputs. Downloads are published only after checksum validation.

Retry the same command after fixing the first reported error. A failed remap/decompilation leaves existing named artifacts intact. Concurrent full generation is refused by `.mcdata/.generate-lock`; avoid editing generated outputs while generation runs.

SIGKILL or a machine failure cannot run rollback. Inspect any retained `.generation-backup.*` snapshot before removing a stale generation lock; the updater never silently deletes another run's recovery snapshot. A restoration error reports the retained snapshot path.

Sources: [transaction helpers](../../../scripts/linux/generation_transaction.sh), [runtime preparation](../../../internal/data/vanilla/prepare.go), [local validation and copy](../../../internal/data/vanilla/local.go), [developer reports](../../../internal/data/vanilla/generate.go), [specialized exporter](../../../internal/data/generator/block/worldgen/README.md).

Related: [artifact sources](data-sources.md), [version upgrades](version-upgrade.md), [testing](../testing.md).
