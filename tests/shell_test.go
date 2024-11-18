package fmsh_test

import (
	"fmsh/shell"
	"testing"
)

// Test command dispatch functionality
func TestDispatchCommand(t *testing.T) {
	// Initialize the command registry
	shell.InitializeCommands()

	testCommandExecuted := false

	// Register a test command with a description and a callback
	shell.RegisterCommand("test-command", "A test command for unit testing", func(args []string) {
		testCommandExecuted = true
	})

	// Dispatch the test command
	shell.DispatchCommand("test-command")

	if !testCommandExecuted {
		t.Errorf("Expected command 'test-command' to be executed, but it was not")
	}
}
