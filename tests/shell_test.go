package fmsh_test

import (
	"fmsh/commands"
	"testing"
)

// Test command dispatch functionality
func TestDispatchCommand(t *testing.T) {
	// Initialize the command registry
	commands.InitializeCommands()

	testCommandExecuted := false

	// Register a test command with a description and a callback
	commands.RegisterCommand("test-command", "A test command for unit testing", func(args []string) {
		testCommandExecuted = true
	})

	// Dispatch the test command
	commands.DispatchCommand("test-command")

	if !testCommandExecuted {
		t.Errorf("Expected command 'test-command' to be executed, but it was not")
	}
}
