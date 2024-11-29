package main

import (
	"fmsh/shell"
	"fmsh/utils"
	"fmt"
)

func main() {
	// Define a color for the shell text (cyan)
	const shellColor = utils.Cyan
	const resetColor = utils.Reset

	fmt.Printf("%sWelcome to fmsh (File Management Shell)!\n", shellColor)
	fmt.Printf("Type 'exit' to quit the shell.%s\n", resetColor)

	// Start the shell
	shell.Start()
}
