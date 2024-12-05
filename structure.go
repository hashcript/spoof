package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Header represents the ICNS file header
type Header struct {
	Magic  [4]byte // Magic identifier (should be "icns")
	Length uint32  // Total length of the ICNS file
}

// FileData represents an individual icon entry in the ICNS file
type FileData struct {
	Type   [4]byte // Type of the entry (e.g., "ic08", "ic10")
	Length uint32  // Length of the entry including the header
	Data   []byte  // Icon data
}

// FileStructure represents the entire ICNS file structure
type FileStructure struct {
	Header
	Body []FileData
}

// ReadFile decodes the ICNS file into a FileStructure
func ReadFile(r *bytes.Reader) (*FileStructure, error) {
	// Parse the header
	var icns FileStructure
	if err := binary.Read(r, binary.BigEndian, &icns.Header); err != nil {
		return nil, fmt.Errorf("error reading header: %s", err)
	}

	// Ensure the file has the correct magic value
	if string(icns.Magic[:]) != "icns" {
		return nil, fmt.Errorf("invalid magic value: %s", string(icns.Magic[:]))
	}

	// Parse file entries
	for {
		// Read the next entry's type and length
		var entry FileData
		err := binary.Read(r, binary.BigEndian, &entry.Type)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error reading entry type: %s", err)
		}
		if err := binary.Read(r, binary.BigEndian, &entry.Length); err != nil {
			return nil, fmt.Errorf("error reading entry length: %s", err)
		}

		// Read the entry data
		dataLength := entry.Length - 8 // Subtract header size (4 bytes for type, 4 bytes for length)
		entry.Data = make([]byte, dataLength)
		if _, err := r.Read(entry.Data); err != nil {
			return nil, fmt.Errorf("error reading entry data: %s", err)
		}

		// Append to the ICNS body
		icns.Body = append(icns.Body, entry)
	}

	return &icns, nil
}
