package utils

import (
	"fmt"
	"os"
)

type ActionType int

const (
	Create ActionType = iota
	Delete
	Move
)

type Action struct {
	Type    ActionType
	Source  string
	Dest    string // Used for Move operations
	Content []byte // Used for Create/Delete operations
}

type UndoManager struct {
	history []Action
}

var GlobalUndoManager = &UndoManager{}

// Push an action onto the stack
func (um *UndoManager) Push(action Action) {
	um.history = append(um.history, action)
}

// Pop the last action from the stack
func (um *UndoManager) Pop() (Action, bool) {
	if len(um.history) == 0 {
		return Action{}, false
	}
	action := um.history[len(um.history)-1]
	um.history = um.history[:len(um.history)-1]
	return action, true
}

// Undo the last action
func (um *UndoManager) Undo() {
	action, ok := um.Pop()
	if !ok {
		fmt.Println("Nothing to undo!")
		return
	}

	switch action.Type {
	case Delete:
		if action.Content != nil {
			// Undo file deletion
			err := os.WriteFile(action.Source, action.Content, 0644)
			if err != nil {
				fmt.Printf("Undo: Failed to restore file: %v\n", err)
			} else {
				fmt.Println("Undo: File restored:", action.Source)
			}
		} else {
			// Undo directory deletion
			err := os.Mkdir(action.Source, 0755)
			if err != nil {
				fmt.Printf("Undo: Failed to restore directory: %v\n", err)
			} else {
				fmt.Println("Undo: Directory restored:", action.Source)
			}
		}
	case Move:
		// Undo file move
		err := os.Rename(action.Dest, action.Source)
		if err != nil {
			fmt.Printf("Undo: Failed to move file: %v\n", err)
		} else {
			fmt.Printf("Undo: Moved file back to %s\n", action.Source)
		}
	default:
		fmt.Println("Unknown action type")
	}
}

// Redo the last undone action
func (um *UndoManager) Redo() {
	action, ok := um.Pop()
	if !ok {
		fmt.Println("Nothing to redo!")
		return
	}

	switch action.Type {
	case Create:
		// Redo file creation
		if action.Content != nil {
			err := os.WriteFile(action.Source, action.Content, 0644)
			if err != nil {
				fmt.Printf("Redo: Failed to restore file: %v\n", err)
			} else {
				fmt.Println("Redo: File restored:", action.Source)
			}
		} else {
			// Redo directory creation
			err := os.Mkdir(action.Source, 0755)
			if err != nil {
				fmt.Printf("Redo: Failed to restore directory: %v\n", err)
			} else {
				fmt.Println("Redo: Directory restored:", action.Source)
			}
		}
	case Move:
		// Redo file move
		err := os.Rename(action.Source, action.Dest)
		if err != nil {
			fmt.Printf("Redo: Failed to move file: %v\n", err)
		} else {
			fmt.Printf("Redo: Moved file to %s\n", action.Dest)
		}
	default:
		fmt.Println("Unknown action type")
	}
}
