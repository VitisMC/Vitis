# Tests and performance diagnosis

[Русский](../rus/testing.md) · [Documentation](README.md)

Run commands from the repository root. Test output, profiles and temporary worlds must stay outside valuable live data.

The full test suite requires `.mcdata/1.21.4/vanilla/` and `.mcdata/1.21.4/loot_tables/`. Prepare them using the [data-generation scripts](generators/README.md). CI downloads the server JAR from Mojang, verifies its checksum and generates these fixtures with Java 21 before running tests.

## Correctness

Linux (Bash):

```bash
go build ./...
make test
make lint
make docs-check
```

Windows (PowerShell 7.2+):

```powershell
go build ./...
go test ./... -count=1
go vet ./...
go run ./cmd/vitis-doccheck
```

For concurrent changes, run `make test-race`; race detection requires a supported toolchain/platform and CGO support. `make fuzz-smoke` runs the bounded-read/decompression and item NBT/component fuzz targets. `make verify` combines tests, vet, race, fuzz smoke and documentation checks.

Start with focused packages during development:

Linux (Bash):

```bash
go test ./internal/protocol/... ./internal/network/... ./internal/session -count=1
go test ./internal/world/... -count=1
go test ./cmd/vitis-doccheck -count=1
```

Windows (PowerShell 7.2+):

```powershell
go test ./internal/protocol/... ./internal/network/... ./internal/session -count=1
go test ./internal/world/... -count=1
go test ./cmd/vitis-doccheck -count=1
```

Inspect skipped tests and fixture requirements. Some worldgen comparisons/benchmarks need the uncommitted vanilla pack or a saved reference world. Check client compatibility separately.

## Scenarios to cover

| Change | Meaningful checks |
| --- | --- |
| Packet codec | Round trip, truncation, size limits, malformed VarInts/NBT/components, invalid numeric values |
| Session lifecycle | Wrong phase, duplicate acknowledgement, authentication failure, duplicate login, disconnect/shutdown |
| World queue | Saturation, rejection, bounded drain, cancellation and mutation ownership |
| Persistence | Snapshot changed after capture, write failure, flush, unload, reopen and metadata preservation |
| Gameplay | Invalid input, game mode/permissions, client synchronization, death/reconnect and persisted state |
| Resources | Bad metadata, invalid/unsupported definitions, failed compiler retains old snapshot |
| Documentation | Broken local target, missing translation/key, duplicate/stale key, fenced examples ignored |

## Client smoke test

Prepare resources, create a disposable world, start Vitis and connect a 1.21.4 client. Check login/configuration, chunk delivery, movement, an inventory action, a supported block action, disconnect/reconnect and `stop`/restart. Test online mode with an authenticated account separately from offline-mode development.

Use [status](status.md) to choose supported actions and [troubleshooting](troubleshooting.md) to report failures.

## Benchmarks and profiles

Use an existing benchmark matching the suspected hot path:

Linux (Bash):

```bash
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression -benchmem -count=3
go test ./internal/entity -run='^$' -bench=. -benchmem -count=3
```

Windows (PowerShell 7.2+):

```powershell
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression -benchmem -count=3
go test ./internal/entity -run='^$' -bench=. -benchmem -count=3
```

After preparing vanilla resources, a small worldgen workload is:

Linux (Bash):

```bash
VITIS_BENCH_SIDE=3 go test ./internal/world -run='^$' \
  -bench=BenchmarkPregenerationFreshWorld -benchtime=1x -count=1 -benchmem
```

Windows (PowerShell 7.2+):

```powershell
$previousSide = $env:VITIS_BENCH_SIDE
try {
    $env:VITIS_BENCH_SIDE = '3'
    go test ./internal/world -run='^$' -bench=BenchmarkPregenerationFreshWorld -benchtime=1x -count=1 -benchmem
}
finally { $env:VITIS_BENCH_SIDE = $previousSide }
```

The benchmark defaults to a much larger selection if you omit the environment variable. Inspect [its options](../../internal/world/pregeneration_perf_test.go) before using dimensions, deadlines, trace or idle/cache comparisons.

Capture profiles from a selected test workload:

Linux (Bash):

```bash
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression \
  -cpuprofile=/tmp/vitis-cpu.pprof -memprofile=/tmp/vitis-heap.pprof
go tool pprof /tmp/vitis-cpu.pprof
```

Windows (PowerShell 7.2+):

```powershell
$cpuProfile = Join-Path $env:TEMP 'vitis-cpu.pprof'
$heapProfile = Join-Path $env:TEMP 'vitis-heap.pprof'
go test ./internal/network -run='^$' -bench=BenchmarkChunkCompression `
  -cpuprofile=$cpuProfile -memprofile=$heapProfile
go tool pprof $cpuProfile
```

Profiles show CPU and memory costs of the selected test. The command may create a test executable; keep it out of Git.

To compare a saved vanilla chunk, provide an independent 1.21.4 reference world:

Linux (Bash):

```bash
go run ./cmd/vitis-worldgen-compare -version 1.21.4 -world /tmp/vitis-reference-world \
  -dimension overworld -x 0 -z 0 -top 10
```

Windows (PowerShell 7.2+):

```powershell
$referenceWorld = Join-Path $env:TEMP 'vitis-reference-world'
go run ./cmd/vitis-worldgen-compare -version 1.21.4 -world $referenceWorld `
  -dimension overworld -x 0 -z 0 -top 10
```

`-version` selects `.mcdata/<version>/vanilla`; use `-resources` to override that path. The saved world's DataVersion and generated tables must still match the supported runtime.

Record revision, resource version, seed, dimensions, distances, players, CPU/RAM and disk. Compare repeated equal workloads before claiming improvements. Live tab and pregeneration metrics measure different stages; there is no configured HTTP profiler/export endpoint in the current bootstrap.

Full generation snapshots and restores its declared outputs on failure; see the [generator workflow](generators/README.md). Optional worldgen comparisons that read `.mcdata` need those developer inputs; starting a binary only creates runtime resources inside its world.

On Windows, `pwsh -NoProfile -File .\scripts\windows\verify.ps1` runs the mandatory suite without Make. Native tooling tests run with `go test ./scripts -run 'TestPowerShell|TestGenerationManifest' -count=1`; they use PowerShell 7.2+ without Pester. Windows CI runs these tests in addition to the existing Linux checks.

For Windows process metrics, use `pwsh -NoProfile -File .\scripts\windows\measure_worldgen.ps1 metrics.json <program> <arguments>`. Output keeps the Linux helper's JSON fields; `peak_rss_kib` reports Windows peak working set, so it is not directly comparable to Linux `Maxrss`. CPU frequency and temperature are unavailable and remain zero. Failed workloads still write metrics and preserve the nonzero exit status. This helper keeps the caller's working directory.

Process measurement example: build the benchmark executable from the repository root, then run it from its package directory so relative resource paths resolve correctly.

Linux (Bash):

```bash
go test -c -o /tmp/vitis-worldgen-bench ./internal/world
(
  cd internal/world
  VITIS_BENCH_SIDE=3 go run ../../scripts/linux/measure_worldgen.go /tmp/vitis-metrics.json /tmp/vitis-worldgen-bench \
    '-test.run=^$' -test.bench=BenchmarkPregenerationFreshWorld -test.benchtime=1x -test.count=1 -test.benchmem
)
```

Windows (PowerShell 7.2+):

```powershell
$benchmark = Join-Path $env:TEMP 'vitis-worldgen-bench.exe'
$metrics = Join-Path $env:TEMP 'vitis-metrics.json'
go test -c -o $benchmark ./internal/world
$previousSide = $env:VITIS_BENCH_SIDE
Push-Location internal/world
try {
    $env:VITIS_BENCH_SIDE = '3'
    pwsh -NoProfile -File ..\..\scripts\windows\measure_worldgen.ps1 $metrics $benchmark `
      '-test.run=^$' -test.bench=BenchmarkPregenerationFreshWorld -test.benchtime=1x -test.count=1 -test.benchmem
}
finally {
    $env:VITIS_BENCH_SIDE = $previousSide
    Pop-Location
}
```
