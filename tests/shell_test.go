package fmsh_test

import (
	"fmsh/shell"
	"testing"
)

// Test command dispatch functionality
func TestDispatchCommand(t *testing.T) {
	shell.InitializeCommands()

	testCommandExecuted := false
	shell.RegisterCommand("test-command", func(args []string) {
		testCommandExecuted = true
	})

	shell.DispatchCommand("test-command")
	if !testCommandExecuted {
		t.Errorf("Expected command 'test-command' to be executed, but it was not")
	}
}
