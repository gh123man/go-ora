package converters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// GoldenCharset holds encode/decode results for one charset ID.
type GoldenCharset struct {
	LangID        int              `json:"lang_id"`
	ConverterType string           `json:"converter_type"`
	CharWidth     int              `json:"char_width"`
	Tests         []GoldenTestCase `json:"tests"`
}

// GoldenTestCase holds one encode/decode test pair.
// Decoded is stored as []byte to preserve invalid UTF-8 through JSON round-trip.
type GoldenTestCase struct {
	Input        string `json:"input"`
	Encoded      []byte `json:"encoded"`
	EncodedIsNil bool   `json:"encoded_is_nil"`
	DecodeInput  []byte `json:"decode_input"`
	DecodedBytes []byte `json:"decoded_bytes"`
}

// All 236 charset IDs from the original NewStringConverter
var allCharsetIDs = []int{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
	21, 22, 23, 25, 27, 28, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42,
	43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 61, 70, 72, 81, 82, 90, 91, 92,
	93, 94, 95, 96, 97, 98, 99, 100, 101, 110, 113, 114, 140, 150, 152, 153,
	154, 155, 156, 158, 159, 160, 161, 162, 163, 164, 165, 166, 167, 170,
	171, 172, 173, 174, 175, 176, 177, 178, 179, 180, 181, 182, 183, 184,
	185, 186, 187, 188, 189, 190, 191, 192, 193, 194, 195, 196, 197, 198,
	199, 200, 201, 202, 203, 204, 205, 206, 207, 208, 210, 211, 221, 222,
	223, 224, 225, 226, 230, 231, 232, 233, 235, 239, 241, 251, 261, 262,
	263, 264, 265, 266, 267, 277, 278, 279, 301, 311, 312, 314, 315, 316,
	317, 319, 320, 322, 323, 324, 325, 326, 327, 351, 352, 353, 354, 368,
	380, 381, 382, 383, 384, 385, 386, 390, 401, 500, 504, 505, 506, 507,
	508, 509, 511, 514, 554, 555, 556, 557, 558, 559, 560, 561, 563, 565,
	566, 567, 590, 829, 830, 831, 832, 833, 834, 835, 836, 837, 838, 840,
	842, 845, 846, 850, 851, 852, 853, 860, 861, 862, 863, 864, 865, 866,
	867, 868, 992, 1002,
	870, 871, 872, 873, 2000, 2002,
}

var testStrings = []string{
	"",
	"Hello, World!",
	"SELECT * FROM dual",
	"abc123!@#$%^&*()",
	"\x00\x01\x02\x7f",
	"café résumé naïve",
	"Ω≈ç√∫",
	"日本語テスト",
	"한국어테스트",
	"中文测试",
	"繁體中文",
	"Привет мир",
	"مرحبا بالعالم",
	"שלום עולם",
	"สวัสดีชาวโลก",
}

var decodeTestBytes = [][]byte{
	{72, 101, 108, 108, 111},
	{0x41, 0x42, 0x43},
	{0x30, 0x31, 0x32, 0x33},
	{0x20, 0x21, 0x22, 0x23, 0x24},
	{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5},
	{0x80, 0x81, 0x82, 0x83},
	{0xFF},
	{0x00},
}

// TestGenerateGoldenData generates golden reference data from the ORIGINAL implementation.
// Run this ONLY against the original string_conversion_new.go, then commit the golden file.
// To regenerate: git stash the new changes, run this test, git stash pop.
func TestGenerateGoldenData(t *testing.T) {
	t.Skip("Golden data already generated. Remove this skip to regenerate.")

	var results []GoldenCharset
	for _, langID := range allCharsetIDs {
		conv := NewStringConverter(langID)
		gc := GoldenCharset{
			LangID:    langID,
			CharWidth: conv.GetLangID(),
		}
		switch conv.(type) {
		case *StringConverter:
			gc.ConverterType = "StringConverter"
		default:
			gc.ConverterType = fmt.Sprintf("%T", conv)
		}
		for _, s := range testStrings {
			encoded := conv.Encode(s)
			gc.Tests = append(gc.Tests, GoldenTestCase{
				Input: s, Encoded: encoded, EncodedIsNil: encoded == nil,
			})
		}
		for _, b := range decodeTestBytes {
			gc.Tests = append(gc.Tests, GoldenTestCase{
				DecodeInput: b, DecodedBytes: []byte(conv.Decode(b)),
			})
		}
		for _, s := range []string{"Hello", "ABC123", "test"} {
			encoded := conv.Encode(s)
			if encoded != nil {
				gc.Tests = append(gc.Tests, GoldenTestCase{
					Input: s, Encoded: encoded, DecodeInput: encoded, DecodedBytes: []byte(conv.Decode(encoded)),
				})
			}
		}
		results = append(results, gc)
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	os.MkdirAll("testdata", 0755)
	if err := os.WriteFile("testdata/golden_charsets.json", data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Logf("Generated %d charsets", len(results))
}

// TestCharsetRegressionVsGolden validates the current NewStringConverter implementation
// against golden data captured from the original 78K-line implementation.
// This ensures the x/text adapter produces identical results.
func TestCharsetRegressionVsGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/golden_charsets.json")
	if err != nil {
		t.Fatalf("Failed to read golden data (run TestGenerateGoldenData against original first): %v", err)
	}

	var golden []GoldenCharset
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("Failed to parse golden data: %v", err)
	}

	totalTests := 0
	totalPass := 0
	totalFail := 0
	failedCharsets := map[int]int{}

	for _, gc := range golden {
		conv := NewStringConverter(gc.LangID)
		if conv == nil {
			t.Errorf("langID %d: NewStringConverter returned nil", gc.LangID)
			continue
		}
		if conv.GetLangID() != gc.LangID {
			t.Errorf("langID %d: GetLangID() = %d", gc.LangID, conv.GetLangID())
		}

		for i, tc := range gc.Tests {
			totalTests++

			// Encode test
			if tc.Input != "" || tc.EncodedIsNil || tc.Encoded != nil {
				if tc.Encoded != nil || tc.EncodedIsNil {
					encoded := conv.Encode(tc.Input)
					if tc.EncodedIsNil && encoded != nil {
						t.Errorf("langID %d test %d: Encode(%q) expected nil, got %v",
							gc.LangID, i, tc.Input, encoded)
						failedCharsets[gc.LangID]++
						totalFail++
						continue
					}
					if !tc.EncodedIsNil && encoded == nil {
						t.Errorf("langID %d test %d: Encode(%q) expected %v, got nil",
							gc.LangID, i, tc.Input, tc.Encoded)
						failedCharsets[gc.LangID]++
						totalFail++
						continue
					}
					if !bytes.Equal(encoded, tc.Encoded) {
						t.Errorf("langID %d test %d: Encode(%q)\n  golden: %v\n  actual: %v",
							gc.LangID, i, truncate(tc.Input, 30), tc.Encoded, encoded)
						failedCharsets[gc.LangID]++
						totalFail++
						continue
					}
				}
			}

			// Decode test
			if tc.DecodeInput != nil {
				decoded := conv.Decode(tc.DecodeInput)
				decodedBytes := []byte(decoded)
				if !bytes.Equal(decodedBytes, tc.DecodedBytes) {
					t.Errorf("langID %d test %d: Decode(%v)\n  golden: %v\n  actual: %v",
						gc.LangID, i, tc.DecodeInput, tc.DecodedBytes, decodedBytes)
					failedCharsets[gc.LangID]++
					totalFail++
					continue
				}
			}

			totalPass++
		}
	}

	t.Logf("Results: %d/%d tests passed, %d failed across %d charsets (%d charsets with failures)",
		totalPass, totalTests, totalFail, len(golden), len(failedCharsets))

	if totalFail > 0 {
		t.Logf("Failed charsets: %v", failedCharsets)
	}
}

// TestUTF8PassThrough verifies UTF-8 charsets are unmodified pass-through.
func TestUTF8PassThrough(t *testing.T) {
	for _, langID := range []int{870, 871, 872, 873} {
		conv := NewStringConverter(langID)
		for _, s := range testStrings {
			encoded := conv.Encode(s)
			if s == "" {
				if encoded != nil {
					t.Errorf("langID %d: Encode(\"\") should be nil, got %v", langID, encoded)
				}
				continue
			}
			if !bytes.Equal(encoded, []byte(s)) {
				t.Errorf("langID %d: UTF-8 Encode(%q) should be pass-through", langID, s)
			}
		}
		for _, b := range decodeTestBytes {
			decoded := conv.Decode(b)
			if decoded != string(b) {
				t.Errorf("langID %d: UTF-8 Decode(%v) should be pass-through, got %q", langID, b, decoded)
			}
		}
	}
}

// TestASCIIRoundTrip verifies that pure ASCII round-trips for all charsets.
func TestASCIIRoundTrip(t *testing.T) {
	// Use uppercase to avoid case-folding charsets like IW7IS960 (langID 23)
	ascii := "HELLO WORLD 0123456789"
	for _, langID := range allCharsetIDs {
		conv := NewStringConverter(langID)
		encoded := conv.Encode(ascii)
		if encoded == nil {
			continue
		}
		decoded := conv.Decode(encoded)
		if decoded != ascii {
			t.Errorf("langID %d: ASCII round-trip failed: %q -> %v -> %q", langID, ascii, encoded, decoded)
		}
	}
}

// TestClone verifies Clone produces an independent converter.
func TestClone(t *testing.T) {
	for _, langID := range []int{1, 31, 178, 832, 846, 852, 865, 870, 2000} {
		conv := NewStringConverter(langID)
		clone := conv.Clone()
		if clone.GetLangID() != conv.GetLangID() {
			t.Errorf("langID %d: Clone LangID mismatch", langID)
		}
		s := "test123"
		if !bytes.Equal(conv.Encode(s), clone.Encode(s)) {
			t.Errorf("langID %d: Clone Encode differs", langID)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
