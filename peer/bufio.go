package main

import (
	"bufio"
	"os"
)

// LineScanner is a helper for reading lines from stdin
type LineScanner struct {
	scanner *bufio.Scanner
}

// NewLineScanner creates a new line scanner
func NewLineScanner() *LineScanner {
	return &LineScanner{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// ReadLine reads a line from stdin
func (ls *LineScanner) ReadLine() string {
	ls.scanner.Scan()
	return ls.scanner.Text()
}
