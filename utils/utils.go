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
		// Undo file deletion (restore file from Content)
		if action.Content != nil {
			err := os.WriteFile(action.Source, action.Content, 0644)
			if err != nil {
				fmt.Printf("Undo: Failed to restore file: %v\n", err)
			} else {
				fmt.Println("Undo: File restored:", action.Source)
			}
		} else {
			// Restore directory
			err := os.Mkdir(action.Source, 0755)
			if err != nil {
				fmt.Printf("Undo: Failed to restore directory: %v\n", err)
			} else {
				fmt.Println("Undo: Directory restored:", action.Source)
			}
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
		// Redo file creation (restore file from Content)
		if action.Content != nil {
			err := os.WriteFile(action.Source, action.Content, 0644)
			if err != nil {
				fmt.Printf("Redo: Failed to restore file: %v\n", err)
			} else {
				fmt.Println("Redo: File restored:", action.Source)
			}
		} else {
			// Restore directory
			err := os.Mkdir(action.Source, 0755)
			if err != nil {
				fmt.Printf("Redo: Failed to restore directory: %v\n", err)
			} else {
				fmt.Println("Redo: Directory restored:", action.Source)
			}
		}
	default:
		fmt.Println("Unknown action type")
	}
}
