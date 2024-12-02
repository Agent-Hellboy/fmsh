package commands

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

func InitializeCommands() {
	RegisterCommand("echo", "Echoes back the input text", HandleEcho)
	RegisterCommand("ls", "Lists the contents of a directory", HandleLs)
	RegisterCommand("cd", "Changes the current directory", HandleCd)
	RegisterCommand("rm", "Removes files or directories", HandleRm)
	RegisterCommand("mkdir", "Creates a new directory", HandleMkdir)
	RegisterCommand("cp", "Copies files or directories", HandleCp)
	RegisterCommand("clear", "Clears the terminal screen", HandleClear)
	RegisterCommand("inspect", "Analyzes the file system", HandleFsAnalytics)
	RegisterCommand("disk-usage", "Shows disk usage of a directory", HandleDiskUsage)
	RegisterCommand("tree", "Displays a tree-like structure of directories", HandleTree)
	RegisterCommand("clean-tmp", "Cleans up temporary files", HandleCleanTmp)
	RegisterCommand("preview", "Previews the contents of a file", HandlePreview)
	RegisterCommand("backup", "Backs up files or directories", HandleBackup)
	RegisterCommand("chmod", "Changes file permissions", HandleChmod)
	RegisterCommand("open", "Opens a file with its default application", HandleOpen)
	RegisterCommand("rename", "Renames a file or directory", HandleRename)
	RegisterCommand("file-history", "Shows the history of a file", HandleFileHistory)
	RegisterCommand("help", "Lists all available commands", HandleHelp)
	RegisterCommand("exit", "Exits the shell", HandleExit)
	RegisterCommand("quit", "Exits the shell", HandleExit)
	RegisterCommand("q", "Exits the shell", HandleExit)
	RegisterCommand("summarise", "Summarizes a directory", HandleSummarise)
	RegisterCommand("analytics", "Analyzes file access patterns", HandleAnalytics)
	RegisterCommand("time", "Shows the current time", HandleTime)
	RegisterCommand("find", "Finds files or directories", HandleFind)
	RegisterCommand("undo", "Undoes the last command", HandleUndo)
}
