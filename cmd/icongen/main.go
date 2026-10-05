// Command icongen packs PNG images into a Windows .ico file.
//
//	go run ./cmd/icongen -o cmd/rift/rift.ico brand/png/icon-16.png brand/png/icon-32.png ...
//
// Each PNG is stored as-is (PNG-compressed ICO entries are supported since
// Windows Vista), so the icon keeps the exact pixels brand/render.mjs drew.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image/png"
	"os"
	"sort"
)

func main() {
	out := flag.String("o", "", "output .ico path")
	flag.Parse()
	if *out == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: icongen -o out.ico in1.png [in2.png ...]")
		os.Exit(2)
	}
	if err := run(*out, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "icongen: %v\n", err)
		os.Exit(1)
	}
}

type entry struct {
	size int
	data []byte
}

func run(out string, inputs []string) error {
	var entries []entry
	for _, path := range inputs {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if cfg.Width != cfg.Height || cfg.Width > 256 {
			return fmt.Errorf("%s: icons must be square and at most 256px (got %dx%d)", path, cfg.Width, cfg.Height)
		}
		entries = append(entries, entry{cfg.Width, data})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].size < entries[j].size })

	var buf bytes.Buffer
	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, [3]uint16{0, 1, uint16(len(entries))})
	offset := 6 + 16*len(entries)
	for _, e := range entries {
		dim := byte(e.size)
		if e.size == 256 {
			dim = 0 // 0 means 256 in ICONDIRENTRY
		}
		// ICONDIRENTRY: width, height, colors, reserved, planes, bpp, size, offset
		buf.Write([]byte{dim, dim, 0, 0})
		binary.Write(&buf, binary.LittleEndian, uint16(1))
		binary.Write(&buf, binary.LittleEndian, uint16(32))
		binary.Write(&buf, binary.LittleEndian, uint32(len(e.data)))
		binary.Write(&buf, binary.LittleEndian, uint32(offset))
		offset += len(e.data)
	}
	for _, e := range entries {
		buf.Write(e.data)
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}
