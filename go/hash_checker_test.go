package main

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var sampleBytes = []byte("hash checker sample\n")

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func TestMatchingMD5ReturnsZero(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "firmware.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "vendor_hash.txt"), []byte(fmt.Sprintf("%x", md5.Sum(sampleBytes))))

	result := run([]string{"--algorithm", "md5", "--workspace", workspace})

	if result != 0 {
		t.Fatalf("expected exit code 0, got %d", result)
	}
}

func TestMatchingSHA256ReturnsZero(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "firmware.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "vendor_hash.txt"), []byte(fmt.Sprintf("%x", sha256.Sum256(sampleBytes))))

	result := run([]string{"--algorithm", "sha256", "--workspace", workspace})

	if result != 0 {
		t.Fatalf("expected exit code 0, got %d", result)
	}
}

func TestMismatchedMD5ReturnsOne(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "firmware.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "vendor_hash.txt"), []byte("00000000000000000000000000000000"))

	result := run([]string{"--algorithm", "md5", "--workspace", workspace})

	if result != exitMismatch {
		t.Fatalf("expected exit code %d, got %d", exitMismatch, result)
	}
}

func TestInvalidHashReturnsFormatError(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "firmware.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "vendor_hash.txt"), []byte("not-a-hash"))

	result := run([]string{"--algorithm", "md5", "--workspace", workspace})

	if result != exitHashFormatError {
		t.Fatalf("expected exit code %d, got %d", exitHashFormatError, result)
	}
}

func TestMultipleTargetsReturnDiscoveryError(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "firmware-a.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "firmware-b.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "vendor_hash.txt"), []byte(fmt.Sprintf("%x", md5.Sum(sampleBytes))))

	result := run([]string{"--algorithm", "md5", "--workspace", workspace})

	if result != exitDiscoveryError {
		t.Fatalf("expected exit code %d, got %d", exitDiscoveryError, result)
	}
}
