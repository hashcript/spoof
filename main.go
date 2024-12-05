package main

import (
	"bytes"
	"fmt"
	"os"
)

func main() {
	// Open the ICNS file
	file, err := os.Open("OpenEmu.icns")
	if err != nil {
		fmt.Println("Error loading the file:", err)
		return
	}
	defer file.Close() // Ensure the file is closed

	// Get the file size
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Error getting file info:", err)
		return
	}
	fileSize := fileInfo.Size()
	fmt.Printf("File size: %d bytes\n", fileSize)

	// Read the entire file into a byte buffer
	data := make([]byte, fileSize)
	_, err = file.Read(data)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	// Decode the file using ReadFile function
	reader := bytes.NewReader(data)
	icns, err := ReadFile(reader)
	if err != nil {
		fmt.Println("Error decoding ICNS file:", err)
		return
	}

	// Print the parsed file structure
	fmt.Printf("Header:\n")
	fmt.Printf("  Magic: %s\n", icns.Magic)
	fmt.Printf("  Length: %d bytes\n", icns.Length)
	fmt.Printf("\nBody (%d entries):\n", len(icns.Body))
	for i, entry := range icns.Body {
		fmt.Printf("  Entry %d:\n", i+1)
		fmt.Printf("    Type: %s\n", entry.Type)
		fmt.Printf("    Length: %d bytes\n", entry.Length)
		fmt.Printf("    Data: (First 16 bytes) %x...\n", entry.Data[:min(16, len(entry.Data))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
