package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func shaSample() string { return fmt.Sprintf("%x", sha256.Sum256(sampleBytes)) }

func runJSON(t *testing.T, args ...string) (verificationReport, string) {
	t.Helper()
	args = append(args, "--format", "json")
	var output bytes.Buffer
	code := runWithWriter(args, &output)
	var report verificationReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, output.String())
	}
	if report.ExitCode != code {
		t.Fatalf("report exit %d != process exit %d", report.ExitCode, code)
	}
	if report.SchemaVersion != 1 || report.Operation != "verify" {
		t.Fatalf("invalid schema: %+v", report)
	}
	for _, status := range []string{"match", "mismatch", "missing", "error", "added", "removed", "changed"} {
		if _, ok := report.Summary[status]; !ok {
			t.Fatalf("missing summary status %s", status)
		}
	}
	return report, output.String()
}

func TestDirectHashJSONAndNoHashDiscovery(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "ファイル name.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "first.txt"), []byte("invalid"))
	writeTestFile(t, filepath.Join(workspace, "second.txt"), []byte("invalid"))
	report, output := runJSON(t, "--algorithm", "SHA256", "--workspace", workspace,
		"--target-file", "ファイル name.bin", "--expected-hash", "  "+strings.ToUpper(shaSample())+"  ")
	if report.ExitCode != 0 || report.Error != nil || len(report.Results) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	result := report.Results[0]
	if result.Path != "ファイル name.bin" || result.Status != "match" || *result.ExpectedHash != shaSample() || *result.ActualHash != shaSample() || result.ErrorCode != nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if strings.Contains(output, workspace) {
		t.Fatalf("JSON leaked absolute workspace: %s", output)
	}
	if !strings.Contains(output, `"error":null`) || !strings.Contains(output, `"error_code":null`) {
		t.Fatal("nullable fields missing")
	}
}

func TestArgumentErrorsAreJSONWithoutPaths(t *testing.T) {
	workspace := t.TempDir()
	for _, args := range [][]string{
		{"--algorithm", "bogus"}, {},
		{"--algorithm", "sha256", "--expected-hash", shaSample(), "--hash-file", ""},
		{"--algorithm", "md5", "--manifest", "not-there"},
		{"--algorithm", "sha256", "--manifest", "not-there", "--target-file", ""},
		{"--algorithm", "sha256", "--manifest", "not-there", "--expected-hash", shaSample()},
		{"--algorithm", "sha256", "unexpected"},
	} {
		report, output := runJSON(t, append(args, "--workspace", workspace)...)
		if report.ExitCode != 2 || report.Error == nil || len(report.Results) != 0 {
			t.Fatalf("args %v: %+v", args, report)
		}
		if strings.Contains(output, workspace) {
			t.Fatal("path exposed")
		}
	}
}

func TestEmptyDirectHashIsFormatError(t *testing.T) {
	report, _ := runJSON(t, "--algorithm", "sha256", "--expected-hash", "")
	if report.ExitCode != 4 || report.Error == nil {
		t.Fatalf("unexpected report %+v", report)
	}
}

func TestUnknownAlgorithmIsNull(t *testing.T) {
	for _, algorithm := range []string{"", "unknown"} {
		report, output := runJSON(t, "--algorithm", algorithm)
		if report.ExitCode != 2 || report.Algorithm != nil || !strings.Contains(output, `"algorithm":null`) {
			t.Fatalf("unknown algorithm must be null: %s", output)
		}
	}
}

func TestParseFailureKeepsKnownAlgorithm(t *testing.T) {
	report, _ := runJSON(t, "--algorithm", "SHA256", "--unknown")
	if report.ExitCode != 2 || report.Algorithm == nil || *report.Algorithm != "sha256" {
		t.Fatalf("known algorithm lost on parse failure: %+v", report)
	}
}

func TestParseFailureUsesLastRequestedJSONFormat(t *testing.T) {
	for _, invalid := range []string{"--unknown", "unexpected-positional"} {
		var output bytes.Buffer
		code := runWithWriter([]string{"--algorithm", "SHA256", "--format", "text", invalid, "--format", "json"}, &output)
		var report verificationReport
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatalf("JSON diagnostic lost: %v %s", err, output.String())
		}
		if code != 2 || report.Error == nil || report.Algorithm == nil || *report.Algorithm != "sha256" {
			t.Fatalf("invalid diagnostic: %s", output.String())
		}
	}
}

func TestSingleMissingTargetRetainsCalculationExitCode(t *testing.T) {
	workspace := t.TempDir()
	report, output := runJSON(t, "--algorithm", "sha256", "--workspace", workspace,
		"--expected-hash", shaSample(), "--target-file", "missing.bin")
	if report.ExitCode != 5 || report.Error == nil || strings.Contains(output, workspace) {
		t.Fatalf("unexpected report: %s", output)
	}
}

func TestManifestParsingBOMCRLFAndSpaces(t *testing.T) {
	data := "\ufeff\r\n" + strings.ToUpper(shaSample()) + " *sub/a file.bin\r\n" + shaSample() + "  leading space.bin\n\n"
	entries, err := parseManifest([]byte(data))
	if err != nil || len(entries) != 2 {
		t.Fatalf("parse: %v %+v", err, entries)
	}
	if entries[0].expected != shaSample() || entries[0].path != "sub/a file.bin" || entries[1].path != "leading space.bin" {
		t.Fatalf("bad entries %+v", entries)
	}
	unicodeEntries := shaSample() + " *Ä.bin\n" + shaSample() + " *ä.bin\n"
	if _, err := parseManifest([]byte(unicodeEntries)); err != nil {
		t.Fatalf("only ASCII case must be folded: %v", err)
	}
}

func TestManifestLoneCarriageReturnAtEOFRejected(t *testing.T) {
	content := shaSample() + " *data.bin\r"
	if _, err := parseManifest([]byte(content)); err == nil || errorCode(err) != 4 {
		t.Fatalf("lone CR filename accepted: %v", err)
	}
	if _, err := parseManifest([]byte(content + "\n")); err != nil {
		t.Fatalf("CRLF rejected: %v", err)
	}
}

func TestMalformedManifestsRejected(t *testing.T) {
	invalid := []string{"", "\n \n", "wrong", "\\" + shaSample() + " *name.bin",
		shaSample() + "\t*name.bin", shaSample() + " xname.bin", strings.Repeat("g", 64) + " *name.bin",
		shaSample() + " *one.bin\n" + shaSample() + " *ONE.bin"}
	for _, path := range []string{"", "/abs.bin", "../outside", "a/../b", "./file", "a//b", "a/", "a\\b", "C:/a", "a:b", "a\tb", "a\x00b", "a/./b"} {
		invalid = append(invalid, shaSample()+" *"+path)
	}
	for _, content := range invalid {
		_, err := parseManifest([]byte(content))
		if err == nil || errorCode(err) != 4 {
			t.Fatalf("expected format error for %q, got %v", content, err)
		}
	}
	if _, err := parseManifest([]byte{0xff, 0xfe}); err == nil || errorCode(err) != 4 {
		t.Fatalf("invalid UTF8 accepted: %v", err)
	}
}

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "checksums.txt")
	writeTestFile(t, path, []byte(content))
	return path
}

func TestManifestMixedStatusesAndQuiet(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "match.bin"), sampleBytes)
	writeTestFile(t, filepath.Join(workspace, "mismatch.bin"), []byte("changed"))
	if err := os.Mkdir(filepath.Join(workspace, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := writeManifest(t, shaSample()+" *match.bin\n"+shaSample()+" *mismatch.bin\n"+shaSample()+" *missing.bin\n"+shaSample()+" *directory\n")
	args := []string{"--algorithm", "sha256", "--workspace", workspace, "--manifest", manifest}
	report, output := runJSON(t, args...)
	if report.ExitCode != 3 || report.Error != nil || len(report.Results) != 4 {
		t.Fatalf("unexpected report: %s", output)
	}
	for _, status := range []string{"match", "mismatch", "missing", "error"} {
		if report.Summary[status] != 1 {
			t.Fatalf("unexpected summary: %+v", report.Summary)
		}
	}
	if report.Results[2].ActualHash != nil || *report.Results[2].ErrorCode != 3 || *report.Results[3].ErrorCode != 3 {
		t.Fatalf("bad error values: %s", output)
	}
	var text bytes.Buffer
	if code := runWithWriter(append(args, "--quiet"), &text); code != 3 {
		t.Fatalf("quiet exit %d", code)
	}
	if strings.HasPrefix(text.String(), "match.bin:") || strings.Contains(text.String(), "\nmatch.bin:") || !strings.Contains(text.String(), "mismatch.bin:") || !strings.Contains(text.String(), "missing.bin:") {
		t.Fatalf("quiet output: %s", text.String())
	}
	// Quiet has no effect on machine-readable JSON.
	quietReport, _ := runJSON(t, append(args, "--quiet")...)
	if quietReport.Summary["match"] != 1 {
		t.Fatal("quiet dropped JSON result")
	}
}

func TestManifestAllMatchQuietAndMismatchExit(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "good.bin"), sampleBytes)
	manifest := writeManifest(t, shaSample()+" *good.bin\n")
	args := []string{"--algorithm", "sha256", "--workspace", workspace, "--manifest", manifest, "--quiet"}
	var output bytes.Buffer
	if code := runWithWriter(args, &output); code != 0 || output.Len() != 0 {
		t.Fatalf("quiet success: %d %s", code, output.String())
	}
	writeTestFile(t, filepath.Join(workspace, "good.bin"), []byte("bad"))
	report, _ := runJSON(t, args...)
	if report.ExitCode != 1 || report.Summary["mismatch"] != 1 {
		t.Fatalf("bad mismatch report %+v", report)
	}
}

func TestManifestErrorsHaveEmptyResults(t *testing.T) {
	workspace := t.TempDir()
	for _, content := range []string{"", shaSample() + " *../outside.bin", shaSample() + " *a.bin\n" + shaSample() + " *A.bin"} {
		report, _ := runJSON(t, "--algorithm", "sha256", "--workspace", workspace, "--manifest", writeManifest(t, content))
		if report.ExitCode != 4 || report.Error == nil || len(report.Results) != 0 {
			t.Fatalf("bad report %+v", report)
		}
	}
	report, _ := runJSON(t, "--algorithm", "sha256", "--workspace", workspace, "--manifest", filepath.Join(workspace, "absent.txt"))
	if report.ExitCode != 3 || report.Error == nil {
		t.Fatalf("bad missing-manifest report %+v", report)
	}
}

func TestManifestPathRelativeToCurrentDirectory(t *testing.T) {
	// Avoid changing the process cwd; put the manifest in cwd and refer to it by basename.
	file, err := os.CreateTemp(".", "manifest-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(shaSample() + " *target.bin\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "target.bin"), sampleBytes)
	report, _ := runJSON(t, "--algorithm", "sha256", "--workspace", workspace, "--manifest", filepath.Base(path))
	if report.ExitCode != 0 {
		t.Fatalf("manifest should be cwd-relative: %+v", report)
	}
}

func TestLinksAreRejected(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "target.bin"), sampleBytes)
	for _, directory := range []bool{false, true} {
		target := filepath.Join(outside, "target.bin")
		link := filepath.Join(workspace, "file-link")
		relative := "file-link"
		if directory {
			target = outside
			link = filepath.Join(workspace, "dir-link")
			relative = "dir-link/target.bin"
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		report, _ := runJSON(t, "--algorithm", "sha256", "--workspace", workspace, "--manifest", writeManifest(t, shaSample()+" *"+relative))
		if report.ExitCode != 3 || report.Summary["error"] != 1 || report.Results[0].ActualHash != nil {
			t.Fatalf("link accepted: %+v", report)
		}
	}
}

func TestWorkspaceLinkAncestorIsRejected(t *testing.T) {
	root := t.TempDir()
	realRoot := t.TempDir()
	workspace := filepath.Join(realRoot, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, "target.bin"), sampleBytes)
	link := filepath.Join(root, "alias")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	report, _ := runJSON(t, "--algorithm", "sha256", "--workspace", filepath.Join(link, "workspace"),
		"--manifest", writeManifest(t, shaSample()+" *target.bin\n"))
	if report.ExitCode != 3 || report.Summary["error"] != 1 || report.Results[0].ActualHash != nil {
		t.Fatalf("workspace ancestor link accepted: %+v", report)
	}
}

func TestManifestLinkAndAncestorAreRejected(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "target.bin"), sampleBytes)
	manifest := writeManifest(t, shaSample()+" *target.bin\n")
	links := t.TempDir()
	leaf := filepath.Join(links, "leaf.txt")
	if err := os.Symlink(manifest, leaf); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	ancestor := filepath.Join(links, "ancestor")
	if err := os.Symlink(filepath.Dir(manifest), ancestor); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{leaf, filepath.Join(ancestor, filepath.Base(manifest))} {
		report, _ := runJSON(t, "--algorithm", "sha256", "--workspace", workspace, "--manifest", path)
		if report.ExitCode != 3 || report.Error == nil || len(report.Results) != 0 {
			t.Fatalf("linked manifest accepted: %+v", report)
		}
	}
}

type testAttributes struct{ FileAttributes uint32 }
type testFileInfo struct {
	mode  os.FileMode
	attrs testAttributes
}

func (f testFileInfo) Name() string       { return "file" }
func (f testFileInfo) Size() int64        { return 0 }
func (f testFileInfo) Mode() os.FileMode  { return f.mode }
func (f testFileInfo) ModTime() time.Time { return time.Time{} }
func (f testFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f testFileInfo) Sys() any           { return &f.attrs }

func TestWindowsReparseAttribute(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific metadata")
	}
	if !isLinkOrReparse(testFileInfo{attrs: testAttributes{FileAttributes: 0x400}}) {
		t.Fatal("reparse attribute ignored")
	}
	if isLinkOrReparse(testFileInfo{}) {
		t.Fatal("regular file rejected")
	}
}

func TestVersionAndCapabilities(t *testing.T) {
	var output bytes.Buffer
	if code := runWithWriter([]string{"--version"}, &output); code != 0 || strings.TrimSpace(output.String()) != "0.2.0" {
		t.Fatalf("version: %d %s", code, output.String())
	}
	output.Reset()
	if code := runWithWriter([]string{"--capabilities"}, &output); code != 0 {
		t.Fatalf("capabilities exit %d", code)
	}
	var caps struct {
		SchemaVersion int      `json:"schema_version"`
		Capabilities  []string `json:"capabilities"`
	}
	if err := json.Unmarshal(output.Bytes(), &caps); err != nil || caps.SchemaVersion != 1 || len(caps.Capabilities) != 4 {
		t.Fatalf("capabilities: %v %s", err, output.String())
	}
	found := false
	for _, cap := range caps.Capabilities {
		if cap == "manifest-verify" {
			found = true
		}
	}
	if !found {
		t.Fatal("manifest capability missing")
	}
}

type failingRead struct{}

func (failingRead) Read(buffer []byte) (int, error) {
	copy(buffer, "bad")
	return 3, io.ErrUnexpectedEOF
}

type failingHash struct{ hash.Hash }

func (failingHash) Write(data []byte) (int, error) { return 0, io.ErrShortWrite }

func TestManifestReadAndCalculationFailuresAreDistinct(t *testing.T) {
	if actual, code := hashManifestReader(failingRead{}, sha256.New()); code != 3 || actual != "" {
		t.Fatalf("read failure: %d %q", code, actual)
	}
	if actual, code := hashManifestReader(bytes.NewReader(sampleBytes), failingHash{sha256.New()}); code != 5 || actual != "" {
		t.Fatalf("calculation failure: %d %q", code, actual)
	}

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "present.bin"), sampleBytes)
	manifest := writeManifest(t, shaSample()+" *missing.bin\n"+shaSample()+" *present.bin\n")
	original := algorithms["sha256"]
	defer func() { algorithms["sha256"] = original }()
	injected := original
	injected.NewHasher = func() hash.Hash { return failingHash{sha256.New()} }
	algorithms["sha256"] = injected
	report, _ := runJSON(t, "--algorithm", "sha256", "--workspace", workspace, "--manifest", manifest)
	if report.ExitCode != 5 || report.Summary["missing"] != 1 || report.Summary["error"] != 1 {
		t.Fatalf("calculation failure should outrank missing: %+v", report)
	}
}
