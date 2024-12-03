package commands

import (
	"bufio"
	"fmsh/utils"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/h2non/filetype"
)

// HandleHelp displays the list of available commands and their descriptions
func HandleHelp(args []string) {
	fmt.Println("\nAvailable commands:")

	// Extract and sort command names for consistent display
	names := make([]string, 0, len(CommandRegistry))
	for name := range CommandRegistry {
		names = append(names, name)
	}
	sort.Strings(names)

	// Determine the maximum widths for alignment
	maxIndexWidth := len(fmt.Sprintf("%d", len(names)-1)) // Maximum width of the index
	maxNameWidth := 0
	for _, name := range names {
		if len(name) > maxNameWidth {
			maxNameWidth = len(name)
		}
	}

	// Display commands with uniform spacing
	for i, name := range names {
		fmt.Printf("%-*d  %-*s -> %s\n", maxIndexWidth, i, maxNameWidth, name, CommandRegistry[name].Description)
	}
}

// HandleEcho handles the "echo" command with automatic colors
func HandleEcho(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: echo <message>")
		return
	}

	colorCode := utils.GetRandomColor()
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

// HandleRm implements the "rm" command with undo support
func HandleRm(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rm <file>")
		return
	}

	path := args[0]

	// Read the file content for undo
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("fmsh: rm: Failed to read file for undo: %v\n", err)
		return
	}

	// Attempt to remove the file
	err = os.Remove(path)
	if err != nil {
		fmt.Printf("fmsh: rm: %v\n", err)
		return
	}

	// Log the delete action in the global UndoManager
	utils.GlobalUndoManager.Push(utils.Action{
		Type:    utils.Delete,
		Source:  path,
		Content: content,
	})

	fmt.Println("File deleted:", path)
}

// HandleUndo implements the "undo" command
func HandleUndo(args []string) {
	utils.GlobalUndoManager.Undo()
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

// HandleFsAnalytics performs file system analytics with improved performance using goroutines
func HandleFsAnalytics(args []string) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Unable to get current directory: %v\n", err)
		return
	}

	var fileCount, dirCount int
	var totalSize int64
	var largestFile string
	var largestFileSize int64
	var mostRecentFile string
	var mostRecentModTime time.Time
	var mu sync.Mutex

	// Channel for processing files and directories
	fileChan := make(chan string, 100)

	// WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup

	// Worker goroutine to process files
	processFile := func() {
		defer wg.Done()
		for path := range fileChan {
			info, err := os.Stat(path)
			if err != nil {
				fmt.Printf("Warning: Unable to access %s: %v\n", path, err)
				continue
			}

			mu.Lock()
			if info.IsDir() {
				dirCount++
			} else {
				fileCount++
				totalSize += info.Size()
				if info.Size() > largestFileSize {
					largestFileSize = info.Size()
					largestFile = path
				}
				if info.ModTime().After(mostRecentModTime) {
					mostRecentModTime = info.ModTime()
					mostRecentFile = path
				}
			}
			mu.Unlock()
		}
	}

	// Start worker goroutines
	numWorkers := 4 // Number of worker goroutines
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go processFile()
	}

	// Walk the directory and send file paths to the channel
	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Warning: Unable to access %s: %v\n", path, err)
			return nil
		}
		fileChan <- path
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
	}

	close(fileChan) // Close the channel to signal workers to stop
	wg.Wait()       // Wait for all workers to finish

	// Display analytics
	fmt.Printf("File System Analytics for: %s\n", currentDir)
	fmt.Println("-----------------------------------------")
	fmt.Printf("Number of files: %d\n", fileCount)
	fmt.Printf("Number of directories: %d\n", dirCount)
	fmt.Printf("Total size of files: %d bytes\n", totalSize)
	if largestFile != "" {
		fmt.Printf("Largest file: %s (%d bytes)\n", largestFile, largestFileSize)
	}
	if mostRecentFile != "" {
		fmt.Printf("Most recently modified file: %s (Modified at: %s)\n", mostRecentFile, mostRecentModTime.Format(time.RFC1123))
	}
	fmt.Println("-----------------------------------------")
}

// HandleFind implements the find command
func HandleFind(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: find <directory> [filename]")
		return
	}

	root := args[0]
	pattern := ""
	if len(args) > 1 {
		pattern = args[1]
	}

	numCores := runtime.NumCPU()               // Get the number of CPU cores
	semaphore := make(chan struct{}, numCores) // Limit concurrency to available cores

	var wg sync.WaitGroup
	results := make(chan string, 100)
	foundDirs := make(chan string, 100)

	// Recursive function for finding files
	var findFiles func(string)
	findFiles = func(dir string) {
		defer wg.Done()

		semaphore <- struct{}{}        // Acquire a slot
		defer func() { <-semaphore }() // Release the slot

		entries, err := os.ReadDir(dir)
		if err != nil {
			// Silently consume the error and move on
			return
		}

		dirHasMatches := false
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				wg.Add(1)
				go findFiles(path)
			} else if pattern == "" || entry.Name() == pattern {
				results <- path
				dirHasMatches = true
			}
		}

		if dirHasMatches {
			foundDirs <- dir
		}
	}

	// Start the search
	wg.Add(1)
	go findFiles(root)

	// Close results and directories channels after all goroutines complete
	go func() {
		wg.Wait()
		close(results)
		close(foundDirs)
	}()

	fmt.Println("Searching for files in", root, "with pattern", pattern)
	count := 0
	for result := range results {
		if count < 10 {
			fmt.Println(result)
		} else {
			fmt.Println("found in many more directories")
			break
		}
		count++
	}
}

// HandleDiskUsage calculates the total disk usage of the current directory
func HandleDiskUsage(args []string) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Unable to get the current directory: %v\n", err)
		return
	}

	var totalSize int64
	err = filepath.Walk(currentDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing file: %v\n", err)
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error calculating disk usage: %v\n", err)
		return
	}

	fmt.Printf("Total disk usage of '%s': %d bytes\n", currentDir, totalSize)
}

// HandleTree displays a tree-like structure of directories and files
func HandleTree(args []string) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Unable to get the current directory: %v\n", err)
		return
	}

	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing file: %v\n", err)
			return nil
		}

		// Create indentation based on depth
		relPath, _ := filepath.Rel(currentDir, path)
		depth := len(filepath.SplitList(relPath)) - 1
		indent := strings.Repeat("  ", depth)

		// Print directories with a slash
		if info.IsDir() {
			fmt.Printf("%s%s/\n", indent, info.Name())
		} else {
			fmt.Printf("%s%s\n", indent, info.Name())
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error generating tree: %v\n", err)
	}
}

// HandleCleanTmp identifies and optionally deletes temporary files
func HandleCleanTmp(args []string) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Unable to get the current directory: %v\n", err)
		return
	}

	fmt.Println("Identifying temporary files...")

	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing file: %v\n", err)
			return nil
		}

		// Match common temporary file extensions
		if strings.HasSuffix(info.Name(), ".tmp") || strings.HasSuffix(info.Name(), ".log") || strings.HasSuffix(info.Name(), ".bak") {
			fmt.Printf("Temporary file: %s\n", path)
			if len(args) > 0 && args[0] == "--delete" {
				err := os.Remove(path)
				if err != nil {
					fmt.Printf("Error deleting file %s: %v\n", path, err)
				} else {
					fmt.Printf("Deleted: %s\n", path)
				}
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error cleaning temporary files: %v\n", err)
	}
}

// HandlePreview displays the first few lines of a file
func HandlePreview(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: preview <filename> [lines]")
		return
	}

	filename := args[0]
	linesToRead := 10
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &linesToRead)
	}

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		fmt.Println(scanner.Text())
		line++
		if line >= linesToRead {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}
}

// HandleBackup creates a timestamped backup of a file
func HandleBackup(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: backup <filename>")
		return
	}

	filename := args[0]
	info, err := os.Stat(filename)
	if err != nil {
		fmt.Printf("Error accessing file: %v\n", err)
		return
	}

	if info.IsDir() {
		fmt.Println("Backup command is for files, not directories.")
		return
	}

	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s%s", base, timestamp, ext)

	err = os.Rename(filename, backupName)
	if err != nil {
		fmt.Printf("Error creating backup: %v\n", err)
		return
	}

	fmt.Printf("Backup created: %s\n", backupName)
}

// // HandleZip creates a zip archive from specified files
// func HandleZip(args []string) {
// 	if len(args) < 2 {
// 		fmt.Println("Usage: zip <archive_name> <file1> <file2> ...")
// 		return
// 	}

// 	archiveName := args[0]
// 	files := args[1:]

// 	zipFile, err := os.Create(archiveName)
// 	if err != nil {
// 		fmt.Printf("Error creating zip file: %v\n", err)
// 		return
// 	}
// 	defer zipFile.Close()

// 	zipWriter := zip.NewWriter(zipFile)
// 	defer zipWriter.Close()

// 	for _, file := range files {
// 		err := addFileToZip(zipWriter, file)
// 		if err != nil {
// 			fmt.Printf("Error adding file to zip: %v\n", err)
// 			return
// 		}
// 	}

// 	fmt.Printf("Archive created: %s\n", archiveName)
// }

// func addFileToZip(zipWriter *zip.Writer, filePath string) error {
// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()

// 	info, err := file.Stat()
// 	if err != nil {
// 		return err
// 	}

// 	header, err := zip.FileInfoHeader(info)
// 	if err != nil {
// 		return err
// 	}

// 	header.Name = filepath.Base(filePath)
// 	header.Method = zip.Deflate

// 	writer, err := zipWriter.CreateHeader(header)
// 	if err != nil {
// 		return err
// 	}

// 	_, err = os.Copy(writer, file)
// 	return err
// }

// // HandleUnzip extracts a zip archive
// func HandleUnzip(args []string) {
// 	if len(args) < 1 {
// 		fmt.Println("Usage: unzip <archive_name>")
// 		return
// 	}

// 	archiveName := args[0]

// 	zipReader, err := zip.OpenReader(archiveName)
// 	if err != nil {
// 		fmt.Printf("Error opening zip file: %v\n", err)
// 		return
// 	}
// 	defer zipReader.Close()

// 	for _, file := range zipReader.File {
// 		err := extractFile(file)
// 		if err != nil {
// 			fmt.Printf("Error extracting file: %v\n", err)
// 			return
// 		}
// 	}

// 	fmt.Printf("Archive extracted: %s\n", archiveName)
// }

// func extractFile(file *zip.File) error {
// 	filePath := file.Name
// 	fmt.Printf("Extracting: %s\n", filePath)

// 	reader, err := file.Open()
// 	if err != nil {
// 		return err
// 	}
// 	defer reader.Close()

// 	targetFile, err := os.Create(filePath)
// 	if err != nil {
// 		return err
// 	}
// 	defer targetFile.Close()

// 	_, err = os.Copy(targetFile, reader)
// 	return err
// }

// HandleChmod changes file permissions
func HandleChmod(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: chmod <permissions> <filename>")
		return
	}

	permissions := args[0]
	filename := args[1]

	perm, err := strconv.ParseUint(permissions, 8, 32)
	if err != nil {
		fmt.Printf("Error parsing permissions: %v\n", err)
		return
	}

	err = os.Chmod(filename, os.FileMode(perm))
	if err != nil {
		fmt.Printf("Error changing permissions: %v\n", err)
		return
	}

	fmt.Printf("Permissions of '%s' changed to '%s'\n", filename, permissions)
}

// HandleOpen opens a file with the system's default application
func HandleOpen(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: open <filename>")
		return
	}

	filename := args[0]

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", filename)
	case "darwin":
		cmd = exec.Command("open", filename)
	default:
		cmd = exec.Command("xdg-open", filename)
	}

	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}

	fmt.Printf("Opened file: %s\n", filename)
}

// HandleRename renames a file or directory
func HandleRename(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: rename <oldname> <newname>")
		return
	}

	oldName := args[0]
	newName := args[1]

	err := os.Rename(oldName, newName)
	if err != nil {
		fmt.Printf("Error renaming file: %v\n", err)
		return
	}

	fmt.Printf("Renamed '%s' to '%s'\n", oldName, newName)
}

var fileHistory []string

// LogFileHistory logs an operation to file history
func LogFileHistory(operation string) {
	fileHistory = append(fileHistory, operation)
}

// HandleFileHistory displays session-level file history
func HandleFileHistory(args []string) {
	if len(fileHistory) == 0 {
		fmt.Println("No file operations recorded in this session.")
		return
	}

	fmt.Println("File Operation History:")
	for _, entry := range fileHistory {
		fmt.Println(entry)
	}
}

func HandleExit(args []string) {
	fmt.Println("Exiting fmsh...")
	os.Exit(0)
}

// HandleTime measures the time taken to execute a command
func HandleTime(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: time <command> [arguments...]")
		return
	}

	command := args[0]
	commandArgs := args[1:]

	start := time.Now() // Record start time

	DispatchCommand(command + " " + strings.Join(commandArgs, " "))

	elapsed := time.Since(start)

	// Print the elapsed time
	fmt.Printf("\nCommand executed in: %v\n", elapsed)
}

// OrganizeDirectory organizes files into folders based on their type in parallel
func OrganiseDirectory(directory string) {
	undoDir := filepath.Join(directory, ".undo")
	os.MkdirAll(undoDir, os.ModePerm) // Create an undo directory to track changes

	var wg sync.WaitGroup         // WaitGroup to synchronize goroutines
	var mu sync.Mutex             // Mutex to protect shared resources
	fileChan := make(chan string) // Channel for file paths
	errorChan := make(chan error) // Channel for errors
	done := make(chan struct{})   // Done channel to signal completion
	fileCount := 0                // Count of files processed (for tracking)

	// Worker goroutine to process files
	go func() {
		for path := range fileChan {
			func(path string) {
				defer wg.Done()

				file, err := os.Open(path)
				if err != nil {
					errorChan <- err
					return
				}
				defer file.Close()

				buf := make([]byte, 261)
				_, err = file.Read(buf)
				if err != nil && err != io.EOF {
					errorChan <- err
					return
				}

				kind, _ := filetype.Match(buf)
				var fileType string
				if kind != filetype.Unknown {
					fileType = kind.Extension
				} else {
					fileType = "unknown"
				}

				// Ensure directory creation is thread-safe
				targetDir := filepath.Join(directory, fileType)
				mu.Lock()
				os.MkdirAll(targetDir, os.ModePerm)
				mu.Unlock()

				// Move the file to its corresponding folder
				targetPath := filepath.Join(targetDir, filepath.Base(path))
				undoPath := filepath.Join(undoDir, filepath.Base(path))
				err = os.Rename(path, targetPath)
				if err != nil {
					errorChan <- err
					return
				}

				// Track the original location in the undo directory
				mu.Lock()
				undoFile, err := os.Create(undoPath)
				mu.Unlock()
				if err != nil {
					errorChan <- err
					return
				}
				defer undoFile.Close()
				_, err = undoFile.WriteString(path)
				if err != nil {
					errorChan <- err
					return
				}

				// Increment file count
				mu.Lock()
				fileCount++
				mu.Unlock()
			}(path)
		}
		close(done)
	}()

	// Walk the directory and send files to the channel
	go func() {
		defer close(fileChan)
		err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories and the undo directory itself
			if info.IsDir() || strings.Contains(path, ".undo") {
				return nil
			}

			wg.Add(1)
			fileChan <- path
			return nil
		})

		if err != nil {
			errorChan <- err
		}
	}()

	go func() {
		wg.Wait()
		close(errorChan)
	}()

	for {
		select {
		case err := <-errorChan:
			if err != nil {
				fmt.Printf("Error organizing file: %v\n", err)
			}
		case <-done:
			fmt.Printf("Directory organized successfully. Total files processed: %d\n", fileCount)
			return
		}
	}
}

func HandleOrganize(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: organize <directory>")
		return
	}

	directory := args[0]
	OrganiseDirectory(directory)
}
