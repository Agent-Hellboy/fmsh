package shell

import (
	"fmt"
	"strings"
)

// CommandCallback represents a function that executes a shell command
type CommandCallback func(args []string)

// CommandRegistry holds the mapping of command names to their callbacks
var CommandRegistry = map[string]CommandCallback{}

// RegisterCommand registers a new command in the command registry
func RegisterCommand(name string, callback CommandCallback) {
	CommandRegistry[name] = callback
}

// DispatchCommand dispatches the command based on user input
func DispatchCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	if callback, exists := CommandRegistry[cmd]; exists {
		callback(args)
	} else {
		fmt.Printf("fmsh: command not found: %s\n", cmd)
	}
}
