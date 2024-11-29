package utils

import (
	"math/rand"
	"time"
)

// Color definitions
const (
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Reset   = "\033[0m"
)

// GetRandomColor returns a random color from the defined colors
func GetRandomColor() string {
	colors := []string{Red, Green, Yellow, Blue, Magenta, Cyan}
	rand.Seed(time.Now().UnixNano())
	return colors[rand.Intn(len(colors))]
}
