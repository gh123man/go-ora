package converters

// This file replaces the original string_conversion_new.go (78K lines, ~11MiB)
// with a compressed binary blob loader.
//
// The original file contained hardcoded charset conversion tables for all 236
// Oracle character sets as Go source-code integer array/map literals. This
// replacement loads the same tables from an embedded zstd-compressed binary
// blob (~770KB), saving ~10MiB of binary space with 100% behavioral compatibility.
//
// To regenerate the blob, run: tools/generate.sh

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"sync"

	"github.com/DataDog/zstd"
)

//go:embed testdata/charsets.bin.zst
var charsetDataZst []byte

var (
	charsetMap  map[int]*StringConverter
	charsetOnce sync.Once
)

func loadCharsets() {
	data, err := zstd.Decompress(nil, charsetDataZst)
	if err != nil {
		panic("go-ora: failed to decompress charset data: " + err.Error())
	}

	r := bytes.NewReader(data)
	var count uint16
	binary.Read(r, binary.LittleEndian, &count)

	charsetMap = make(map[int]*StringConverter, count)

	for i := 0; i < int(count); i++ {
		var langID uint16
		var charWidth uint8
		var eReplace, dReplace uint16

		binary.Read(r, binary.LittleEndian, &langID)
		binary.Read(r, binary.LittleEndian, &charWidth)
		binary.Read(r, binary.LittleEndian, &eReplace)
		binary.Read(r, binary.LittleEndian, &dReplace)

		dBuffer := readU16Slice(r)
		dBuffer2 := readU16SliceLarge(r)
		eBuffer := readU16KeyI32ValMap(r)
		leading := readU16Map(r)

		charsetMap[int(langID)] = &StringConverter{
			LangID:    int(langID),
			CharWidth: int(charWidth),
			eReplace:  int(eReplace),
			dReplace:  int(dReplace),
			dBuffer:   dBuffer,
			dBuffer2:  dBuffer2,
			eBuffer:   eBuffer,
			leading:   leading,
		}
	}
}

func readU16Slice(r *bytes.Reader) []int {
	var length uint16
	binary.Read(r, binary.LittleEndian, &length)
	if length == 0 {
		return nil
	}
	s := make([]int, length)
	for i := range s {
		var v uint16
		binary.Read(r, binary.LittleEndian, &v)
		s[i] = int(v)
	}
	return s
}

func readU16SliceLarge(r *bytes.Reader) []int {
	var length uint32
	binary.Read(r, binary.LittleEndian, &length)
	if length == 0 {
		return nil
	}
	s := make([]int, length)
	for i := range s {
		var v uint16
		binary.Read(r, binary.LittleEndian, &v)
		s[i] = int(v)
	}
	return s
}

func readU16KeyI32ValMap(r *bytes.Reader) map[int]int {
	var length uint16
	binary.Read(r, binary.LittleEndian, &length)
	if length == 0 {
		return nil
	}
	m := make(map[int]int, length)
	for i := 0; i < int(length); i++ {
		var k uint16
		var v int32
		binary.Read(r, binary.LittleEndian, &k)
		binary.Read(r, binary.LittleEndian, &v)
		m[int(k)] = int(v)
	}
	return m
}

func readU16Map(r *bytes.Reader) map[int]int {
	var length uint16
	binary.Read(r, binary.LittleEndian, &length)
	if length == 0 {
		return nil
	}
	m := make(map[int]int, length)
	for i := 0; i < int(length); i++ {
		var k, v uint16
		binary.Read(r, binary.LittleEndian, &k)
		binary.Read(r, binary.LittleEndian, &v)
		m[int(k)] = int(v)
	}
	return m
}

// NewStringConverter returns an IStringConverter for the given Oracle charset ID.
// Charset tables are loaded from an embedded zstd-compressed binary blob on first call.
func NewStringConverter(langID int) IStringConverter {
	// UTF-8 and UTF-16 use simple pass-through — no tables needed
	switch {
	case langID >= 870 && langID <= 873:
		return &StringConverter{LangID: langID, CharWidth: 1}
	case langID == 2000 || langID == 2002:
		return &StringConverter{LangID: langID, CharWidth: 2}
	}

	charsetOnce.Do(loadCharsets)

	if sc, ok := charsetMap[langID]; ok {
		return sc.Clone()
	}

	// Unknown charset: fallback to UTF-8 pass-through
	return &StringConverter{LangID: langID, CharWidth: 1}
}
