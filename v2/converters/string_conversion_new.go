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
	"fmt"
	"sync"

	"github.com/DataDog/zstd"
)

//go:embed charsetdata/charsets.bin.zst
var charsetDataZst []byte

var (
	charsetMap  map[int]*StringConverter
	charsetOnce sync.Once
)

// binaryReader wraps a bytes.Reader and accumulates the first read error,
// allowing callers to check once at the end rather than after every read.
type binaryReader struct {
	r   *bytes.Reader
	err error
}

func (br *binaryReader) read(v any) {
	if br.err == nil {
		br.err = binary.Read(br.r, binary.LittleEndian, v)
	}
}

func loadCharsets() {
	data, err := zstd.Decompress(nil, charsetDataZst)
	if err != nil {
		panic("go-ora: failed to decompress charset data: " + err.Error())
	}

	br := &binaryReader{r: bytes.NewReader(data)}

	var count uint16
	br.read(&count)

	charsetMap = make(map[int]*StringConverter, count)

	for i := 0; i < int(count); i++ {
		var langID uint16
		var charWidth uint8
		var eReplace, dReplace uint16

		br.read(&langID)
		br.read(&charWidth)
		br.read(&eReplace)
		br.read(&dReplace)

		dBuffer := readU16Slice(br)
		dBuffer2 := readU16SliceLarge(br)
		eBuffer := readU16KeyI32ValMap(br)
		leading := readU16Map(br)

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

	if br.err != nil {
		panic(fmt.Sprintf("go-ora: failed to deserialize charset data: %v", br.err))
	}
}

func readU16Slice(br *binaryReader) []int {
	var length uint16
	br.read(&length)
	if length == 0 {
		return nil
	}
	s := make([]int, length)
	for i := range s {
		var v uint16
		br.read(&v)
		s[i] = int(v)
	}
	return s
}

func readU16SliceLarge(br *binaryReader) []int {
	var length uint32
	br.read(&length)
	if length == 0 {
		return nil
	}
	s := make([]int, length)
	for i := range s {
		var v uint16
		br.read(&v)
		s[i] = int(v)
	}
	return s
}

func readU16KeyI32ValMap(br *binaryReader) map[int]int {
	var length uint16
	br.read(&length)
	if length == 0 {
		return nil
	}
	m := make(map[int]int, length)
	for i := 0; i < int(length); i++ {
		var k uint16
		var v int32
		br.read(&k)
		br.read(&v)
		m[int(k)] = int(v)
	}
	return m
}

func readU16Map(br *binaryReader) map[int]int {
	var length uint16
	br.read(&length)
	if length == 0 {
		return nil
	}
	m := make(map[int]int, length)
	for i := 0; i < int(length); i++ {
		var k, v uint16
		br.read(&k)
		br.read(&v)
		m[int(k)] = int(v)
	}
	return m
}

// NewStringConverter returns an IStringConverter for the given Oracle charset ID,
// or nil if the charset ID is not recognized (matching the original library behavior).
// Charset tables are loaded from an embedded zstd-compressed binary blob on first call.
func NewStringConverter(langID int) IStringConverter {
	charsetOnce.Do(loadCharsets)
	if sc, ok := charsetMap[langID]; ok {
		return sc.Clone()
	}
	return nil
}
