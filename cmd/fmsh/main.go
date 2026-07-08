// Command fmsh is the CLI and daemon entrypoint for the Forensic Machine
// Shell — a black box recorder for AI-driven development on macOS.
package main

import "fmsh/internal/cli"

func main() {
	cli.Execute()
}
