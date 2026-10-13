# Packet IDs and registration

[Русский](../../rus/packets/packet-ids.md) · [Documentation](../README.md)

An ID belongs to a **version + state + direction**. Login, configuration and play reuse numeric values; compare mappings only inside the correct context. Wire direction names in source data (`toClient`/`toServer`) must match runtime outbound/inbound registration.

## Two ID packages

| Package | Role |
| --- | --- |
| [internal/protocol/packetid](../../../internal/protocol/packetid/) | Runtime constants imported by existing typed packet codecs |
| [internal/data/generated/packet](../../../internal/data/generated/packet/) | Compatibility constants produced by the [packet generator](../../../internal/data/generator/packet/main.go) |

The generator updates the canonical `internal/protocol/packetid/packetid.go` table and produces compatibility constants in `internal/data/generated/packet`. Existing names remain unchanged. Run `go run internal/data/generator/packet/main.go --check` to verify both outputs against pinned protocol 769 data.

Both trees contain stringer directives. The full updater does not run every `go:generate` directive; inspect enum/string outputs when changing mappings.

## Codec and handler registration

[Protocol states](../../../internal/protocol/states/) register typed factories by direction/state. [Protocol Registry](../../../internal/protocol/registry.go) stores copy-on-write lookup maps and rejects duplicate context keys. [Session routing](../../../internal/session/handler.go) separately maps state/ID to behavior.

For a new packet, add its constant, codec and handler. If the action is intentionally unavailable, document it in the [play policy](../../../internal/session/play_policy.go).

## Check a change

Compare against `.mcdata/<version>/protocol.json` and versioned source/reports. Verify actual numeric values, changed names, state/direction and packet field layouts; avoid inferring IDs from alphabetical order or historical iota blocks.

Run protocol and session tests, including malformed payloads and phase ordering, then validate with the target client. The consolidated [version upgrade guide](../generators/version-upgrade.md) contains the complete workflow.
