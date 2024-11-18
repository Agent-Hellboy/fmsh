package shell

import (
	"bufio"
	"fmt"
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
)

func InitializeCommands() {
	RegisterCommand("echo", "Echoes back the input text", HandleEcho)
	RegisterCommand("ls", "Lists the contents of a directory", HandleLs)
	RegisterCommand("cd", "Changes the current directory", HandleCd)
	RegisterCommand("rm", "Removes files or directories", HandleRm)
	RegisterCommand("mkdir", "Creates a new directory", HandleMkdir)
	RegisterCommand("cp", "Copies files or directories", HandleCp)
	RegisterCommand("clear", "Clears the terminal screen", HandleClear)
	RegisterCommand("inspect", "Analyzes the file system", HandleFsAnalytics)
	RegisterCommand("search", "Searches for files or directories", HandleSearch)
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
}

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

// HandleSearch implements the search command with wildcard support
func HandleSearch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: search <pattern>")
		return
	}

	// Get the search pattern
	pattern := args[0]

	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Unable to get the current directory: %v\n", err)
		return
	}

	fmt.Printf("Searching for '%s' in '%s'...\n", pattern, currentDir)

	// Counter for matches
	matchCount := 0

	// Walk the directory and find matches
	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %s: %v\n", path, err)
			return nil // Skip the file on error
		}

		// Match the file or directory name with the pattern
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			fmt.Printf("Error matching pattern: %v\n", err)
			return nil
		}

		if matched {
			// Print the matched file/directory
			matchCount++
			if info.IsDir() {
				fmt.Printf("[DIR]  %s\n", path)
			} else {
				fmt.Printf("[FILE] %s\n", path)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through the directory: %v\n", err)
		return
	}

	// Summary of matches
	if matchCount == 0 {
		fmt.Println("No matches found.")
	} else {
		fmt.Printf("Found %d match(es).\n", matchCount)
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
