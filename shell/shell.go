package shell

import (
	"fmsh/commands"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterh/liner"
)

var historyFile string
var history = []string{}

// Start begins the shell session
func Start() {
	commands.InitializeCommands()

	setHistoryFile()
	line := liner.NewLiner()
	defer func() {
		saveHistory(line)
		line.Close()
	}()
	loadHistory(line)

	line.SetCompleter(func(line string) (c []string) {
		for _, cmd := range history {
			if strings.HasPrefix(cmd, line) {
				c = append(c, cmd)
			}
		}
		return
	})

	for {
		input, err := line.Prompt("fmsh> ")
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("\nExiting fmsh...")
				break
			}
			fmt.Println("Error reading input:", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "exit" {
			fmt.Println("Exiting fmsh...")
			break
		}

		history = append(history, input)
		line.AppendHistory(input)

		commands.DispatchCommand(input)
	}
}

// setHistoryFile sets the history file path in the user's home directory
func setHistoryFile() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error determining user home directory: %v\n", err)
		homeDir = "." // Default to current directory if home dir can't be determined
	}
	historyFile = filepath.Join(homeDir, ".fmsh_history")
}

// loadHistory loads command history from the history file
func loadHistory(line *liner.State) {
	file, err := os.Open(historyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Error loading history: %v\n", err)
		}
		return
	}
	defer file.Close()

	if _, err := line.ReadHistory(file); err != nil {
		fmt.Printf("Error reading history: %v\n", err)
	}
}

// saveHistory saves the current command history to the history file
func saveHistory(line *liner.State) {
	file, err := os.Create(historyFile)
	if err != nil {
		fmt.Printf("Error saving history: %v\n", err)
		return
	}
	defer file.Close()

	if _, err := line.WriteHistory(file); err != nil {
		fmt.Printf("Error writing history: %v\n", err)
	}
}
