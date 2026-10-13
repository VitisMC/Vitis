# Registries, tags and configuration sync

[Русский](../../rus/registry/overview.md) · [Documentation](../README.md)

## Data roles

- Builtin registries map canonical names to versioned numeric IDs.
- Configuration registries provide entries with pre-encoded NBT for client synchronization.
- Tags group registry members by numeric IDs.
- Runtime datapacks load recipes, loot, advancements and worldgen into a separate resource snapshot.

Datapack reload does **not** rebuild the immutable protocol registry manager or automatically synchronize new configuration entries with connected clients.

## Runtime construction

[Manager](../../../internal/registry/manager.go) loads the builtin map and generated configuration data, then validates the result. [IDRegistry](../../../internal/registry/registry.go) exposes name/ID lookup; missing lookups return sentinel values that callers must handle.

[Builtin adapters](../../../internal/registry/builtin.go) reference [generated registry data](../../../internal/data/generated/registry/). Configuration entries contain names and encoded NBT. Their IDs and NBT numeric types must match the Minecraft version.

[Tags](../../../internal/registry/tags.go) build UpdateTags from generated data. Check every referenced registry and member ID after a version change.

## Configuration flow

[Session configuration](../../../internal/session/configuration_handler.go) offers the known vanilla core pack, processes the client's response, sends registry data and tags, then finish configuration. The client acknowledgement completes the transition.

When the client knows the offered pack of the same version, matching entry NBT can be omitted. Unknown and custom data is sent in full.

## Generation

The [registry generator](../../../internal/data/generator/registry/main.go) reads versioned `registries.json` and datapack directories. It generates ID tables and configuration NBT with explicit type handling. Tags use their own generator.

Use [data sources](../generators/data-sources.md) for artifact roles and the [version upgrade checklist](../generators/version-upgrade.md) for additions/removals, NBT hints, builtin adapters and client tests.
