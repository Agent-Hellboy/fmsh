package shell_test

import (
	"fmsh/utils"
	"testing"
)

func TestGetRandomColor(t *testing.T) {
	color := utils.GetRandomColor()
	if color != utils.Red && color != utils.Green && color != utils.Yellow && color != utils.Blue && color != utils.Magenta && color != utils.Cyan {
		t.Errorf("Expected a random color, but got %s", color)
	}
}
