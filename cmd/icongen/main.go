package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	// Need extended draw for resizing if possible, but standard might suffice for simple scaling
)

// Since I cannot easily get external deps like x/image without go get (which might fail on restricted envs),
// I will use nearest neighbor or basic subsampling if standard lib allows,
// OR simpler: just assume I can fetch "golang.org/x/image/draw".
// "go get" worked before for other packages.

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Read Input PNG
	inputFile := "web/icon.png"
	f, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("open input: %v", err)
	}
	defer f.Close()

	srcImg, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode png: %v", err)
	}

	// 2. Resize to 256x256 (Windows Max Standard Icon Size)
	// We'll implement a simple box sampling resize here to avoid external deps if possible,
	// but let's try to do a simple resize using standard library if possible?
	// Standard lib doesn't have high quality resize.
	// I'll rely on the fact that 'go get' works and use "golang.org/x/image/draw".
	
	destRect := image.Rect(0, 0, 256, 256)
	dstImg := image.NewRGBA(destRect)
	
	// Simple nearest neighbor / subsample approach if we don't want external deps:
	// But let's try to write a simple scaler manually to be safe and dependency-free.
	// Actually, for an icon, "Nearest Neighbor" on a huge image looks bad.
	// "Average" is better.
	
	scaleX := float64(srcImg.Bounds().Dx()) / 256.0
	scaleY := float64(srcImg.Bounds().Dy()) / 256.0
	
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			// Center sampling
			sx := int(float64(x)*scaleX)
			sy := int(float64(y)*scaleY)
			dstImg.Set(x, y, srcImg.At(srcImg.Bounds().Min.X+sx, srcImg.Bounds().Min.Y+sy))
		}
	}

	// 3. Encode resized image to PNG buffer
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, dstImg); err != nil {
		return fmt.Errorf("encode png: %v", err)
	}
	pngBytes := buf.Bytes()
	pngSize := uint32(len(pngBytes))

	// 4. Create ICO Header
	// Header: Reserved(2) | Type(2) | Count(2)
	// Entry: Width(1) | Height(1) | Colors(1) | Reserved(1) | Planes(2) | BPP(2) | Size(4) | Offset(4)
	
	outFile, err := os.Create("cmd/rift/app.ico")
	if err != nil {
		return fmt.Errorf("create output: %v", err)
	}
	defer outFile.Close()

	// Header
	binary.Write(outFile, binary.LittleEndian, uint16(0)) // Reserved
	binary.Write(outFile, binary.LittleEndian, uint16(1)) // Type 1 = Icon
	binary.Write(outFile, binary.LittleEndian, uint16(1)) // Count = 1 image

	// Directory Entry
	outFile.Write([]byte{0, 0}) // Width, Height (0 means 256)
	outFile.Write([]byte{0, 0}) // Colors, Reserved
	binary.Write(outFile, binary.LittleEndian, uint16(1))  // Planes
	binary.Write(outFile, binary.LittleEndian, uint16(32)) // BPP
	binary.Write(outFile, binary.LittleEndian, pngSize)    // Size of data
	binary.Write(outFile, binary.LittleEndian, uint32(6+16)) // Offset (6 header + 16 entry)

	// Image Data
	outFile.Write(pngBytes)

	fmt.Println("Success: Converted to 256x256 PNG-style ICO")
	return nil
}
