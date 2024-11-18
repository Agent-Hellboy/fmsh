package shell

import (
	"fmt"
	"strings"
)

// Command represents a shell command with a description and a callback
type Command struct {
	Description string
	Callback    CommandCallback
}

// CommandCallback represents a function that executes a shell command
type CommandCallback func(args []string)

// CommandRegistry holds the mapping of command names to their Command struct
var CommandRegistry = map[string]Command{}

// RegisterCommand registers a new command with its description and callback
func RegisterCommand(name, description string, callback CommandCallback) {
	CommandRegistry[name] = Command{
		Description: description,
		Callback:    callback,
	}
}

// DispatchCommand dispatches the command based on user input
func DispatchCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	if command, exists := CommandRegistry[cmd]; exists {
		command.Callback(args)
	} else {
		fmt.Printf("fmsh: command not found: %s\n", cmd)
	}
}
