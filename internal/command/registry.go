package command

import (
	"fmt"
	"strings"
	"sync"
)

// Registry stores all registered commands and provides dispatch.
type Registry struct {
	mu       sync.RWMutex
	commands map[string]*Command
	aliases  map[string]string // alias -> primary name
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command, 32),
		aliases:  make(map[string]string, 32),
	}
}

// Register adds a command to the registry. Returns error on duplicate name.
func (r *Registry) Register(cmd *Command) error {
	if cmd == nil {
		return fmt.Errorf("register command: nil command")
	}
	if cmd.Name == "" {
		return fmt.Errorf("register command: empty name")
	}
	if cmd.Execute == nil {
		return fmt.Errorf("register command %q: nil executor", cmd.Name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	lower := strings.ToLower(cmd.Name)
	if _, exists := r.commands[lower]; exists {
		return fmt.Errorf("register command %q: already registered", cmd.Name)
	}
	if _, aliased := r.aliases[lower]; aliased {
		return fmt.Errorf("register command %q: name conflicts with alias", cmd.Name)
	}

	r.commands[lower] = cmd

	for _, alias := range cmd.Aliases {
		aliasLower := strings.ToLower(alias)
		if _, exists := r.commands[aliasLower]; exists {
			continue
		}
		if _, exists := r.aliases[aliasLower]; exists {
			continue
		}
		r.aliases[aliasLower] = lower
	}

	return nil
}

// Get returns a command by name or alias.
func (r *Registry) Get(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getLocked(name)
}

func (r *Registry) getLocked(name string) (*Command, bool) {
	lower := strings.ToLower(name)
	if cmd, ok := r.commands[lower]; ok {
		return cmd, true
	}
	if primary, ok := r.aliases[lower]; ok {
		if cmd, ok := r.commands[primary]; ok {
			return cmd, true
		}
	}
	return nil, false
}

// All returns all registered commands (no duplicates for aliases).
func (r *Registry) All() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}
	return result
}

// AllVisible returns commands visible to the given sender based on permission level.
func (r *Registry) AllVisible(sender Sender) []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		if sender != nil && !sender.HasPermission(cmd.PermissionLevel) {
			continue
		}
		result = append(result, cmd)
	}
	return result
}

// Count returns the number of registered commands.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.commands)
}

// Dispatch parses input and executes the matching command.
// Input should not include the leading '/'.
func (r *Registry) Dispatch(sender Sender, input string) error {
	if sender == nil {
		return fmt.Errorf("dispatch command: nil sender")
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("dispatch command: empty input")
	}

	parts := strings.Fields(input)
	label := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	r.mu.RLock()
	cmd, found := r.getLocked(label)
	r.mu.RUnlock()

	if !found {
		sender.SendMessage("§cUnknown command: /" + label + ". Type /help for help.")
		return nil
	}

	if !sender.HasPermission(cmd.PermissionLevel) {
		sender.SendMessage("§cYou don't have permission to use this command.")
		return nil
	}

	ctx := &Context{
		Sender:   sender,
		RawInput: input,
		Label:    label,
		Args:     args,
	}

	return cmd.Execute(ctx)
}

// TabSuggestions returns tab completion suggestions for the given partial input.
func (r *Registry) TabSuggestions(sender Sender, input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	parts := strings.Fields(input)

	// If we're still typing the command name
	if len(parts) <= 1 && !strings.HasSuffix(input, " ") {
		prefix := strings.ToLower(input)
		r.mu.RLock()
		defer r.mu.RUnlock()

		var suggestions []string
		for name, cmd := range r.commands {
			if strings.HasPrefix(name, prefix) {
				if sender == nil || sender.HasPermission(cmd.PermissionLevel) {
					suggestions = append(suggestions, name)
				}
			}
		}
		for alias := range r.aliases {
			if strings.HasPrefix(alias, prefix) {
				primary := r.aliases[alias]
				if cmd, ok := r.commands[primary]; ok {
					if sender == nil || sender.HasPermission(cmd.PermissionLevel) {
						suggestions = append(suggestions, alias)
					}
				}
			}
		}
		return suggestions
	}

	// Delegate to command's tab completer
	label := strings.ToLower(parts[0])
	r.mu.RLock()
	cmd, found := r.getLocked(label)
	r.mu.RUnlock()

	if !found {
		return nil
	}
	if sender != nil && !sender.HasPermission(cmd.PermissionLevel) {
		return nil
	}
	if cmd.TabComplete == nil {
		return nil
	}

	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	ctx := &Context{
		Sender:   sender,
		RawInput: input,
		Label:    label,
		Args:     args,
	}
	return cmd.TabComplete(ctx)
}
