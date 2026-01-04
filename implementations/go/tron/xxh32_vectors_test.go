package tron

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type xxh32Vector struct {
	Len    int    `json:"len"`
	Seed   string `json:"seed"`
	Result string `json:"result"`
}

type xxhashVectors struct {
	Arrays struct {
		XXH32 []xxh32Vector `json:"XSUM_XXH32_testdata"`
	} `json:"arrays"`
}

func findXXHashVectorsPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("unable to locate test file path")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "test_vectors", "xxhash_sanity_test_vectors.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("xxhash_sanity_test_vectors.json not found")
}

func parseHexUint32(input string) (uint32, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, errors.New("empty hex string")
	}
	value, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, err
	}
	return uint32(value), nil
}

func generateSanityBuffer(size int) []byte {
	const prime32 uint64 = 2654435761
	const prime64 uint64 = 11400714785074694797
	buffer := make([]byte, size)
	byteGen := prime32
	for i := 0; i < size; i++ {
		buffer[i] = byte(byteGen >> 56)
		byteGen *= prime64
	}
	return buffer
}

func TestXXH32SanityVectors(t *testing.T) {
	path, err := findXXHashVectorsPath()
	if err != nil {
		t.Fatalf("locate vectors: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}

	var vectors xxhashVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}

	if len(vectors.Arrays.XXH32) == 0 {
		t.Fatal("no XXH32 vectors found")
	}

	maxLen := 0
	for _, entry := range vectors.Arrays.XXH32 {
		if entry.Len > maxLen {
			maxLen = entry.Len
		}
	}

	buffer := generateSanityBuffer(maxLen)
	for i, entry := range vectors.Arrays.XXH32 {
		seed, err := parseHexUint32(entry.Seed)
		if err != nil {
			t.Fatalf("parse seed at %d: %v", i, err)
		}
		want, err := parseHexUint32(entry.Result)
		if err != nil {
			t.Fatalf("parse result at %d: %v", i, err)
		}
		got := XXH32(buffer[:entry.Len], seed)
		if got != want {
			t.Fatalf("vector %d len=%d seed=%s got=0x%08X want=%s", i, entry.Len, entry.Seed, got, entry.Result)
		}
	}
}
