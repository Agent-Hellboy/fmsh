package shell_test

import (
	"bytes"
	"fmsh/commands"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHandleFind(t *testing.T) {
	// Redirect output for testing
	var output bytes.Buffer
	originalStdout := os.Stdout
	defer func() { os.Stdout = originalStdout }()

	// Create a pipe to capture output
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	os.Stdout = w

	// Capture the output in a separate goroutine
	done := make(chan error)
	go func() {
		_, err := io.Copy(&output, r)
		r.Close() // Close the read end after copying
		done <- err
	}()

	// Perform the action to test
	args := []string{"./test_directory", "file.txt"}
	commands.HandleFind(args)

	// Close the write end of the pipe
	w.Close()

	// Wait for the output capture to finish
	if err := <-done; err != nil {
		t.Fatalf("Failed to copy output: %v", err)
	}

	// Validate the captured output
	result := output.String()
	if !strings.Contains(result, "file.txt") {
		t.Errorf("Expected 'file.txt' in output, but got: %s", result)
	}
}
