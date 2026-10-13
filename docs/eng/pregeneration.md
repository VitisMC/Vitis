# Chunk pregeneration

[Русский](../rus/pregeneration.md) · [Documentation](README.md)

The built-in `chunky` command prepares chunks ahead of player exploration. No Chunky plugin installation is needed. Permission level **4** is required; use the console to start an administrative job.

## Select and start

Console example for `world`. Coordinates and radius are in **blocks**:

```text
chunky world world
chunky shape circle
chunky center 0 0
chunky radius 256
chunky selection
chunky start
chunky progress world
```

Use `square` or `circle`. `center` without coordinates uses a player position and requires a player sender. The radius must be 1–30,000 blocks and the selection must stay within allowed world coordinates. Selection covers chunk centers; the displayed total is more useful than estimating by area.

World names come from the runtime: default configured world name, `minecraft:the_nether`, `minecraft:the_end`, and any additional loaded dimensions. Keep commands for one selection under the same sender context.

## Control a job

```text
chunky pause world
chunky continue world
chunky cancel world
```

An omitted world uses the sender's selected/default world. Only one running or paused job exists per world; cancel it before starting another selection.

Pause stops issuing new work; outstanding work can finish. Cancel does not remove already generated world data. Shutdown checkpoints unfinished jobs; restart restores them **paused**, requiring `continue`. Preserve `pregeneration.json` in each dimension's storage directory alongside the world's other files.

## Read progress

Completed counts chunks confirmed on disk, not merely generated chunks or packets sent. Average chunks/s excludes paused time; maximum tracks completion windows of at least one second. At completion it is also raised to at least the overall average and the remaining window count, so it is not a pure sliding one-second peak. Generation count can differ from completion count because existing chunks and async saves participate in the job.

Generation uses the existing bounded chunk workers and save queues. Large jobs consume disk, CPU and memory; begin with a small radius and inspect tick duration and process memory before enlarging it.

Sources: [command](../../internal/command/chunky.go), [selection adapter](../../internal/session/pregeneration_commands.go), [job lifecycle](../../internal/world/pregeneration.go), [memory reclamation](../../internal/world/pregeneration_memory.go).
