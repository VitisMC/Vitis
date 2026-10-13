# Add a command, packet or mechanic

[Русский](../rus/extending.md) · [Documentation](README.md)

## Command

Start with [command definitions](../../internal/command/) and [Command/Context](../../internal/command/command.go). Registration example:

```go
package example

import "github.com/vitismc/vitis/internal/command"

// RegisterExample registers a bounded, permission-checked console/player command.
func RegisterExample(registry *command.Registry) error {
    return registry.Register(&command.Command{
        Name:            "example",
        Description:     "Replies with a fixed message",
        Usage:           "/example",
        PermissionLevel: 2,
        Execute: func(ctx *command.Context) error {
            if ctx.ArgCount() != 0 {
                ctx.ReplyError("Usage: /example")
                return nil
            }
            ctx.Reply("Example command is active")
            return nil
        },
    })
}
```

The example is a standalone package. In the core, add commands to the existing package using its constructors and registration functions.

Wire the command into the relevant registration function and check registration errors. Define argument graph/tab completion where useful. A nontrivial command validates arguments and delegates to an existing core/adapter; do not put world generation or disk IO in its executor.

For a world mutation, use the established command bridge and tick scheduling. Handle queue refusal. Test permissions, bad arguments, console/player behavior, the invoked mutation and failure propagation. Check help/DeclareCommands and update the [catalogue](commands.md) in both languages.

## Packet

1. Confirm the target's state, direction, ID and wire layout from versioned data/source.
2. Add the typed packet in the relevant protocol packet package. Implement `ID`, `Decode`, `Encode` with bounded reads and validated lengths.
3. Register its codec through [protocol states](../../internal/protocol/states/).
4. Register a session handler for real behavior, or add an explicit bounded no-op policy with a reason if acceptance is intentional.
5. Validate phase, sender and authority; schedule world mutation when required. Keep response order correct for negotiation and state changes.
6. Test round trips, truncation, malformed lengths, trailing bytes, invalid phases and relevant gameplay effects. Fuzz attacker-controlled decoders where appropriate.

Do not assign an ID from its numeric position in an unrelated packet list. See [packet IDs](packets/packet-ids.md) for the generated/runtime distinction.

## Mechanic

Locate the existing [integration path](gameplay.md). Keep rule calculation in the smallest suitable core package; add an adapter only when an actual dependency requires it.

Define which goroutine owns state, how a queued action can fail, and how reconnect/death/world unload affects it. Do not retain Player/World/session pointers beyond their lifecycle.

Wire the rule into an existing action or tick callback. Synchronize client state, mark persistence dirty and include save/load handling if the feature persists. Test ordinary behavior, invalid client intent, disconnect/cancellation and state restoration.

## Verify the result

Verify the action through the server handler, client updates and saved data. Update [capabilities](status.md), relevant operating instructions and both language versions. Use [testing](testing.md) for required checks.
