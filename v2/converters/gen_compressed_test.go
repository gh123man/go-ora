package converters

import (
	"bytes"
	"encoding/binary"
	"os"
	"sort"
	"testing"
)

// TestGenerateRawCharsetData extracts all 236 charset converter tables
// from the original NewStringConverter and serializes them into a compact
// binary format. Compression is done externally by tools/generate.sh.
//
// Format (all little-endian):
//
//	Header: uint16(count)
//	Per charset:
//	  uint16(langID), uint8(charWidth), uint16(eReplace), uint16(dReplace)
//	  uint16(len(dBuffer)),  [uint16...]
//	  uint32(len(dBuffer2)), [uint16...]   (uint32 for CJK which can be >65535 entries)
//	  uint16(len(eBuffer)),  [uint16(key), int32(value)...]  (sorted by key; int32 for signed CJK multi-byte sequences)
//	  uint16(len(leading)),  [uint16(key), uint16(value)...]
//
// Run ONLY against the original string_conversion_new.go to generate the blob.
// Use tools/generate.sh for the full reproducible pipeline.
func TestGenerateRawCharsetData(t *testing.T) {
	var buf bytes.Buffer

	binary.Write(&buf, binary.LittleEndian, uint16(len(allCharsetIDs)))

	for _, langID := range allCharsetIDs {
		conv := NewStringConverter(langID)
		sc, ok := conv.(*StringConverter)
		if !ok {
			t.Fatalf("langID %d: expected *StringConverter, got %T", langID, conv)
		}

		binary.Write(&buf, binary.LittleEndian, uint16(sc.LangID))
		binary.Write(&buf, binary.LittleEndian, uint8(sc.CharWidth))
		binary.Write(&buf, binary.LittleEndian, uint16(sc.eReplace))
		binary.Write(&buf, binary.LittleEndian, uint16(sc.dReplace))

		writeU16Slice(&buf, sc.dBuffer)
		writeU16SliceLarge(&buf, sc.dBuffer2)
		writeU16KeyI32ValMap(&buf, sc.eBuffer)
		writeU16Map(&buf, sc.leading)
	}

	os.MkdirAll("testdata", 0755)
	if err := os.WriteFile("testdata/charsets.bin", buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	t.Logf("Generated: %d charsets, %d bytes raw (compress with tools/generate.sh)",
		len(allCharsetIDs), buf.Len())
}

func writeU16Slice(buf *bytes.Buffer, s []int) {
	binary.Write(buf, binary.LittleEndian, uint16(len(s)))
	for _, v := range s {
		binary.Write(buf, binary.LittleEndian, uint16(v))
	}
}

func writeU16SliceLarge(buf *bytes.Buffer, s []int) {
	binary.Write(buf, binary.LittleEndian, uint32(len(s)))
	for _, v := range s {
		binary.Write(buf, binary.LittleEndian, uint16(v))
	}
}

func writeU16KeyI32ValMap(buf *bytes.Buffer, m map[int]int) {
	binary.Write(buf, binary.LittleEndian, uint16(len(m)))
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		binary.Write(buf, binary.LittleEndian, uint16(k))
		binary.Write(buf, binary.LittleEndian, int32(m[k]))
	}
}

func writeU16Map(buf *bytes.Buffer, m map[int]int) {
	binary.Write(buf, binary.LittleEndian, uint16(len(m)))
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		binary.Write(buf, binary.LittleEndian, uint16(k))
		binary.Write(buf, binary.LittleEndian, uint16(m[k]))
	}
}
