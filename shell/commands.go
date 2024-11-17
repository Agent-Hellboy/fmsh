package shell

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// InitializeCommands registers all the available commands
func InitializeCommands() {
	RegisterCommand("echo", HandleEcho)
	RegisterCommand("ls", HandleLs)
	RegisterCommand("cd", HandleCd)
	RegisterCommand("rm", HandleRm)
	RegisterCommand("mkdir", HandleMkdir)
	RegisterCommand("cp", HandleCp)
	RegisterCommand("clear", HandleClear)
}

// HandleEcho handles the "echo" command with automatic colors
func HandleEcho(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: echo <message>")
		return
	}

	colorCode := GetRandomColor()
	message := strings.Join(args, " ")
	fmt.Printf("%s%s\033[0m\n", colorCode, message)
}

// HandleLs implements the "ls" command
func HandleLs(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Printf("fmsh: ls: %v\n", err)
		return
	}

	for _, file := range files {
		if file.IsDir() {
			fmt.Printf("%s/\n", file.Name())
		} else {
			fmt.Println(file.Name())
		}
	}
}

// HandleCd implements the "cd" command
func HandleCd(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: cd <directory>")
		return
	}

	path := args[0]
	err := os.Chdir(path)
	if err != nil {
		fmt.Printf("fmsh: cd: %v\n", err)
	}
}

// HandleRm implements the "rm" command
func HandleRm(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rm <file>")
		return
	}

	path := args[0]
	err := os.Remove(path)
	if err != nil {
		fmt.Printf("fmsh: rm: %v\n", err)
	}
}

// HandleMkdir implements the "mkdir" command
func HandleMkdir(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mkdir <directory>")
		return
	}

	path := args[0]
	err := os.Mkdir(path, 0755)
	if err != nil {
		fmt.Printf("fmsh: mkdir: %v\n", err)
	}
}

// HandleCp implements the "cp" command
func HandleCp(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: cp <source> <destination>")
		return
	}

	source := args[0]
	destination := args[1]
	err := os.Rename(source, destination)
	if err != nil {
		fmt.Printf("fmsh: cp: %v\n", err)
	}
}

// HandleClear clears the terminal screen
func HandleClear(args []string) {
	fmt.Print("\033[H\033[2J")
}
