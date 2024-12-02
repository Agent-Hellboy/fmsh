package shell_test

import (
	"fmsh/utils"
	"os"
	"testing"
)

func TestGetRandomColor(t *testing.T) {
	color := utils.GetRandomColor()
	if color != utils.Red && color != utils.Green && color != utils.Yellow && color != utils.Blue && color != utils.Magenta && color != utils.Cyan {
		t.Errorf("Expected a random color, but got %s", color)
	}
}

func TestUndoManager(t *testing.T) {
	undoManager := utils.GlobalUndoManager

	t.Run("Undo file deletion", func(t *testing.T) {
		filePath := "test_file.txt"
		content := []byte("test content")

		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		undoManager.Push(utils.Action{
			Type:    utils.Delete,
			Source:  filePath,
			Content: content,
		})

		if err := os.Remove(filePath); err != nil {
			t.Fatalf("Failed to delete test file: %v", err)
		}

		undoManager.Undo()

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read restored file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("Expected file content: %s, got: %s", content, data)
		}

		os.Remove(filePath)
	})

}
