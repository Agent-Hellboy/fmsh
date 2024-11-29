package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/h2non/filetype"
)

// HandleAnalytics implements the "analytics" command
func HandleAnalytics(args []string) {
	fmt.Println("Analytics command not implemented yet.")
}

// HandleSummarise summarizes a directory
func HandleSummarise(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: fmsh summarise <directory>")
		return
	}

	directory := args[0]
	fileSummary := make(map[string]int) // To count number of files per type
	fileSizes := make(map[string]int64) // To sum sizes of files per type
	untypedSize := int64(0)             // Total size of untyped files
	untypedCount := 0                   // Count of untyped files

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			// Read the first 261 bytes to detect file type
			head := make([]byte, 261)
			n, _ := file.Read(head)
			head = head[:n]

			fileType := "untyped" // Default to untyped
			if kind, _ := filetype.Match(head); kind != filetype.Unknown {
				fileType = kind.MIME.Value
			}

			// Update the summary
			if fileType == "untyped" {
				untypedCount++
				untypedSize += info.Size()
			} else {
				fileSummary[fileType]++
				fileSizes[fileType] += info.Size()
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error summarising directory: %v\n", err)
		return
	}

	// Print the summary
	fmt.Println("Directory Summary:")
	for fileType, count := range fileSummary {
		fmt.Printf("Type: %s\n", fileType)
		fmt.Printf("  Files: %d\n", count)
		fmt.Printf("  Total Size: %d bytes\n", fileSizes[fileType])
	}

	// Print untyped file summary
	fmt.Println("\nUntyped Files:")
	fmt.Printf("  Files: %d\n", untypedCount)
	fmt.Printf("  Total Size: %d bytes\n", untypedSize)
}
