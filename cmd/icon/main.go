package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
)

func main() {
	pngPath := "web/logo.png"
	icoPath := "app.ico"

	fmt.Printf("Reading %s...\n", pngPath)
	pngDisplay, err := os.ReadFile(pngPath)
	if err != nil {
		log.Fatal(err)
	}

	// Create ICO file
	file, err := os.Create(icoPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// 1. INFO HEADER (6 bytes)
	// Reserved (2) | Type (2) | Count (2)
	// Type 1 = Icon
	binary.Write(file, binary.LittleEndian, uint16(0))
	binary.Write(file, binary.LittleEndian, uint16(1))
	binary.Write(file, binary.LittleEndian, uint16(1))

	// 2. DIRECTORY ENTRY (16 bytes)
	// Width (1) | Height (1) | Colors (1) | Reserved (1) | Planes (2) | BitCount (2) | Size (4) | Offset (4)
	
	// We assume the PNG is 1024x1024 or 512x512 from the generator.
	// For ICO, 0 means 256px.
	// Large PNGs inside ICO are valid in modern Windows.
	
	width := uint8(0)  // 256 or larger
	height := uint8(0) // 256 or larger
	
	binary.Write(file, binary.LittleEndian, width)
	binary.Write(file, binary.LittleEndian, height)
	binary.Write(file, binary.LittleEndian, uint8(0)) // No palette
	binary.Write(file, binary.LittleEndian, uint8(0)) // Reserved
	binary.Write(file, binary.LittleEndian, uint16(1)) // Planes
	binary.Write(file, binary.LittleEndian, uint16(32)) // BitCount (32-bit timestamp)
	
	size := uint32(len(pngDisplay))
	binary.Write(file, binary.LittleEndian, size)
	
	offset := uint32(6 + 16) // Header + 1 Entry
	binary.Write(file, binary.LittleEndian, offset)

	// 3. PNG DATA
	_, err = file.Write(pngDisplay)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Success! Generated %s (%d bytes)\n", icoPath, size)
}
