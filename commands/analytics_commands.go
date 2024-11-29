package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/h2non/filetype"
)

// HandleAnalytics implements the "analytics" command
func HandleAnalytics(args []string) {
	fmt.Println("Analytics command not implemented yet.")
}

type FileInfo struct {
	Type string
	Size int64
}

// HandleSummarise summarizes a directory using goroutines
func HandleSummarise(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: fmsh summarise <directory>")
		return
	}

	directory := args[0]
	fileChan := make(chan FileInfo)
	var wg sync.WaitGroup

	// Goroutine to collect and summarize file data
	fileSummary := make(map[string]int)
	fileSizes := make(map[string]int64)
	var untypedCount int
	var untypedSize int64
	var summaryWg sync.WaitGroup
	summaryWg.Add(1)
	go func() {
		defer summaryWg.Done()
		for info := range fileChan {
			if info.Type == "untyped" {
				untypedCount++
				untypedSize += info.Size
			} else {
				fileSummary[info.Type]++
				fileSizes[info.Type] += info.Size
			}
		}
	}()

	// Walk the directory and start goroutines for file processing
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			wg.Add(1)
			go func(path string, size int64) {
				defer wg.Done()
				processFile(path, size, fileChan)
			}(path, info.Size())
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error summarising directory: %v\n", err)
		return
	}

	wg.Wait()
	close(fileChan)

	summaryWg.Wait()

	printSummary(fileSummary, fileSizes, untypedCount, untypedSize)

}

func printSummary(fileSummary map[string]int, fileSizes map[string]int64, untypedCount int, untypedSize int64) {
	maxTypeWidth := len("File Type")
	maxCountWidth := len("File Count")
	maxSizeWidth := len("Total Size (bytes)")

	for fileType, count := range fileSummary {
		if len(fileType) > maxTypeWidth {
			maxTypeWidth = len(fileType)
		}
		if len(fmt.Sprintf("%d", count)) > maxCountWidth {
			maxCountWidth = len(fmt.Sprintf("%d", count))
		}
		if len(fmt.Sprintf("%d", fileSizes[fileType])) > maxSizeWidth {
			maxSizeWidth = len(fmt.Sprintf("%d", fileSizes[fileType]))
		}
	}

	if len("Untyped Files") > maxTypeWidth {
		maxTypeWidth = len("Untyped Files")
	}
	if len(fmt.Sprintf("%d", untypedCount)) > maxCountWidth {
		maxCountWidth = len(fmt.Sprintf("%d", untypedCount))
	}
	if len(fmt.Sprintf("%d", untypedSize)) > maxSizeWidth {
		maxSizeWidth = len(fmt.Sprintf("%d", untypedSize))
	}

	// Print the header
	fmt.Println(strings.Repeat("=", maxTypeWidth+maxCountWidth+maxSizeWidth+8))
	fmt.Printf("%-*s | %-*s | %-*s\n",
		maxTypeWidth, "File Type",
		maxCountWidth, "File Count",
		maxSizeWidth, "Total Size (bytes)")
	fmt.Println(strings.Repeat("-", maxTypeWidth+maxCountWidth+maxSizeWidth+8))

	// Print the summary for each file type
	for fileType, count := range fileSummary {
		fmt.Printf("%-*s | %-*d | %-*d\n",
			maxTypeWidth, fileType,
			maxCountWidth, count,
			maxSizeWidth, fileSizes[fileType])
	}

	fmt.Println(strings.Repeat("-", maxTypeWidth+maxCountWidth+maxSizeWidth+8))

	// Print the untyped file summary
	fmt.Printf("%-*s | %-*d | %-*d\n",
		maxTypeWidth, "Untyped Files",
		maxCountWidth, untypedCount,
		maxSizeWidth, untypedSize)
	fmt.Println(strings.Repeat("=", maxTypeWidth+maxCountWidth+maxSizeWidth+8))
}

func processFile(path string, size int64, fileChan chan<- FileInfo) {
	file, err := os.Open(path)
	if err != nil {
		fileChan <- FileInfo{Type: "untyped", Size: size}
		return
	}
	defer file.Close()

	head := make([]byte, 261)
	n, _ := file.Read(head)
	head = head[:n]

	fileType := "untyped"
	if kind, _ := filetype.Match(head); kind != filetype.Unknown {
		fileType = kind.MIME.Value
	}

	fileChan <- FileInfo{Type: fileType, Size: size}
}
