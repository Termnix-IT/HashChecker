package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type algorithmConfig struct {
	Label          string
	ExpectedLength int
	NewHasher      func() hash.Hash
}

var algorithms = map[string]algorithmConfig{
	"md5": {
		Label:          "MD5",
		ExpectedLength: 32,
		NewHasher:      md5.New,
	},
	"sha256": {
		Label:          "SHA256",
		ExpectedLength: 64,
		NewHasher:      sha256.New,
	},
}

var excludedExtensions = map[string]bool{
	".bat": true,
	".cmd": true,
	".exe": true,
	".go":  true,
	".log": true,
	".ps1": true,
	".py":  true,
	".txt": true,
}

const (
	exitMismatch             = 1
	exitDiscoveryError       = 2
	exitReadError            = 3
	exitHashFormatError      = 4
	exitHashCalculationError = 5
)

type hashCheckerError struct {
	Message  string
	ExitCode int
}

func (e *hashCheckerError) Error() string {
	return e.Message
}

type options struct {
	Algorithm    string
	Workspace    string
	HashFile     string
	HashSet      bool
	TargetFile   string
	TargetSet    bool
	ExpectedHash string
	ExpectedSet  bool
	Manifest     string
	ManifestSet  bool
	Format       string
	Quiet        bool
	Version      bool
	Capabilities bool
}

func parseArgs(args []string) (options, error) {
	var opts options
	requestedFormat := "text"
	// Preserve JSON diagnostics even if flag parsing stops at an invalid input.
	for i, arg := range args {
		if arg == "--format=json" || arg == "-format=json" {
			requestedFormat = "json"
		}
		if arg == "--format=text" || arg == "-format=text" {
			requestedFormat = "text"
		}
		if (arg == "--format" || arg == "-format") && i+1 < len(args) {
			requestedFormat = args[i+1]
		}
	}
	flags := flag.NewFlagSet("hash_checker", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.Algorithm, "algorithm", "", "Hash algorithm to use.")
	flags.StringVar(&opts.Algorithm, "a", "", "Hash algorithm to use.")
	flags.StringVar(&opts.Workspace, "workspace", ".", "Workspace used for automatic file discovery.")
	flags.StringVar(&opts.Workspace, "w", ".", "Workspace used for automatic file discovery.")
	flags.StringVar(&opts.HashFile, "hash-file", "", "Text file containing the vendor-provided hash.")
	flags.StringVar(&opts.TargetFile, "target-file", "", "File to calculate and verify.")
	flags.StringVar(&opts.ExpectedHash, "expected-hash", "", "Expected hash value.")
	flags.StringVar(&opts.Manifest, "manifest", "", "SHA256 checksum manifest relative to current directory.")
	flags.StringVar(&opts.Format, "format", requestedFormat, "Output format: text or json.")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Suppress matching results in text output.")
	flags.BoolVar(&opts.Version, "version", false, "Show version.")
	flags.BoolVar(&opts.Capabilities, "capabilities", false, "Show supported capabilities.")

	if err := flags.Parse(args); err != nil {
		opts.Algorithm = strings.ToLower(opts.Algorithm)
		opts.Format = requestedFormat
		return opts, fmt.Errorf("引数を解釈できません。対応するオプションと値を確認してください。")
	}
	opts.Format = requestedFormat
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "expected-hash" {
			opts.ExpectedSet = true
		}
		if f.Name == "manifest" {
			opts.ManifestSet = true
		}
		if f.Name == "hash-file" {
			opts.HashSet = true
		}
		if f.Name == "target-file" {
			opts.TargetSet = true
		}
	})
	opts.Algorithm = strings.ToLower(opts.Algorithm)
	if flags.NArg() != 0 {
		return opts, fmt.Errorf("位置引数は使用できません。")
	}
	if opts.Format != "text" && opts.Format != "json" {
		return opts, fmt.Errorf("出力形式は text または json を指定してください。")
	}
	if opts.Version || opts.Capabilities {
		return opts, nil
	}
	if opts.ExpectedSet && opts.HashSet {
		return opts, fmt.Errorf("--expected-hash と --hash-file は同時に指定できません。")
	}
	if opts.ManifestSet && (opts.HashSet || opts.TargetSet || opts.ExpectedSet) {
		return opts, fmt.Errorf("一覧検証と単体検証の入力は同時に指定できません。")
	}
	if opts.ManifestSet && opts.Manifest == "" {
		return opts, fmt.Errorf("一覧ファイルを指定してください。")
	}

	if _, ok := algorithms[opts.Algorithm]; !ok {
		return opts, fmt.Errorf("ハッシュ方式を指定してください。対応方式: md5, sha256")
	}
	if opts.ManifestSet && opts.Algorithm != "sha256" {
		return opts, fmt.Errorf("一覧検証には --algorithm sha256 を指定してください。")
	}

	return opts, nil
}

func resolvePath(path string, workspace string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Abs(path)
	}
	return filepath.Abs(filepath.Join(workspace, path))
}

func listFiles(paths []string) string {
	lines := make([]string, 0, len(paths))
	for _, path := range paths {
		lines = append(lines, fmt.Sprintf("- %s", filepath.Base(path)))
	}
	return strings.Join(lines, "\n")
}

func discoverHashFile(workspace string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(workspace, "*.txt"))
	if err != nil {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("ハッシュファイルを検索できませんでした。\n%v", err),
			ExitCode: exitDiscoveryError,
		}
	}

	hashFiles := make([]string, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			hashFiles = append(hashFiles, path)
		}
	}
	sort.Strings(hashFiles)

	if len(hashFiles) == 0 {
		return "", &hashCheckerError{
			Message:  "ハッシュファイルが見つかりません。作業フォルダに txt ファイルを1つ配置してください。",
			ExitCode: exitDiscoveryError,
		}
	}
	if len(hashFiles) >= 2 {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("ハッシュファイル候補が複数見つかりました。\n\n見つかった txt ファイル:\n%s\n\n対応: ハッシュ値を記載した txt ファイルを1つだけ残してください。", listFiles(hashFiles)),
			ExitCode: exitDiscoveryError,
		}
	}

	return hashFiles[0], nil
}

func discoverTargetFile(workspace string) (string, error) {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("確認対象ファイルを検索できませんでした。\n%v", err),
			ExitCode: exitDiscoveryError,
		}
	}

	targetFiles := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if excludedExtensions[ext] {
			continue
		}
		targetFiles = append(targetFiles, filepath.Join(workspace, entry.Name()))
	}
	sort.Strings(targetFiles)

	if len(targetFiles) == 0 {
		return "", &hashCheckerError{
			Message:  "確認対象ファイルが見つかりません。作業フォルダに確認したいファイルを1つ配置してください。",
			ExitCode: exitDiscoveryError,
		}
	}
	if len(targetFiles) >= 2 {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("確認対象ファイル候補が複数見つかりました。\n\n見つかったファイル:\n%s\n\n対応: 確認したいファイルを1つだけ残してください。", listFiles(targetFiles)),
			ExitCode: exitDiscoveryError,
		}
	}

	return targetFiles[0], nil
}

func readVendorHash(hashFile string, config algorithmConfig) (string, error) {
	content, err := os.ReadFile(hashFile)
	if err != nil {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("ハッシュファイルを読み取れませんでした。ファイルの権限や状態を確認してください。\n%v", err),
			ExitCode: exitReadError,
		}
	}

	text := strings.TrimPrefix(string(content), "\ufeff")
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return "", &hashCheckerError{
			Message:  "ハッシュファイルにハッシュ値が記載されていません。",
			ExitCode: exitHashFormatError,
		}
	}
	if len(lines) >= 2 {
		return "", &hashCheckerError{
			Message:  "ハッシュファイルに複数行の値が記載されています。\n\n対応: ハッシュ値のみを1行で記載してください。",
			ExitCode: exitHashFormatError,
		}
	}

	vendorHash := lines[0]
	if len(vendorHash) != config.ExpectedLength {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("%s ハッシュ値の文字数が不正です。\n\n期待する文字数 : %d 文字\n実際の文字数   : %d 文字", config.Label, config.ExpectedLength, len(vendorHash)),
			ExitCode: exitHashFormatError,
		}
	}

	if _, err := hex.DecodeString(vendorHash); err != nil {
		return "", &hashCheckerError{
			Message:  "ハッシュ値の形式が不正です。\n\nハッシュ値には 0-9、a-f、A-F のみ使用できます。",
			ExitCode: exitHashFormatError,
		}
	}

	return strings.ToLower(vendorHash), nil
}

func calculateFileHash(targetFile string, config algorithmConfig) (string, error) {
	file, err := os.Open(targetFile)
	if err != nil {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("確認対象ファイルのハッシュ値を算出できませんでした。ファイルの権限や状態を確認してください。\n%v", err),
			ExitCode: exitHashCalculationError,
		}
	}
	defer file.Close()

	hasher := config.NewHasher()
	if _, err := io.CopyBuffer(hasher, file, make([]byte, 1024*1024)); err != nil {
		return "", &hashCheckerError{
			Message:  fmt.Sprintf("確認対象ファイルのハッシュ値を算出できませんでした。ファイルの権限や状態を確認してください。\n%v", err),
			ExitCode: exitHashCalculationError,
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

const version = "0.2.0"

type verificationResult struct {
	Path         string  `json:"path"`
	Status       string  `json:"status"`
	ExpectedHash *string `json:"expected_hash"`
	ActualHash   *string `json:"actual_hash"`
	ErrorCode    *int    `json:"error_code"`
}

type reportError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type verificationReport struct {
	SchemaVersion int                  `json:"schema_version"`
	Algorithm     *string              `json:"algorithm"`
	Operation     string               `json:"operation"`
	Results       []verificationResult `json:"results"`
	Summary       map[string]int       `json:"summary"`
	ExitCode      int                  `json:"exit_code"`
	Error         *reportError         `json:"error"`
	HashSource    string               `json:"-"`
}

func newReport(algorithm string) verificationReport {
	var selected *string
	if _, ok := algorithms[algorithm]; ok {
		selected = &algorithm
	}
	return verificationReport{SchemaVersion: 1, Algorithm: selected, Operation: "verify",
		Results: []verificationResult{}, Summary: map[string]int{
			"match": 0, "mismatch": 0, "missing": 0, "error": 0,
			"added": 0, "removed": 0, "changed": 0}}
}

func errorCode(err error) int {
	var e *hashCheckerError
	if errors.As(err, &e) {
		return e.ExitCode
	}
	return exitHashCalculationError
}

func failReport(report verificationReport, err error) verificationReport {
	code := errorCode(err)
	// OS errors can carry absolute paths. The first diagnostic line is sufficient
	// for a portable report and does not include the wrapped OS error.
	message := "想定外のエラーが発生しました。"
	var e *hashCheckerError
	if errors.As(err, &e) {
		message = strings.Split(e.Message, "\n")[0]
	}
	report.ExitCode = code
	report.Error = &reportError{Code: code, Message: message}
	return report
}

func inputError(message string, code int) error {
	return &hashCheckerError{Message: message, ExitCode: code}
}

func validateHash(value string, config algorithmConfig) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) != config.ExpectedLength {
		return "", inputError("ハッシュ値の文字数が不正です。", exitHashFormatError)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", inputError("ハッシュ値の形式が不正です。", exitHashFormatError)
	}
	return strings.ToLower(value), nil
}

type manifestEntry struct{ path, expected string }

func portablePath(path string) bool {
	if path == "" || strings.ContainsAny(path, ":\\") || strings.HasPrefix(path, "/") {
		return false
	}
	for _, c := range path {
		if unicode.IsControl(c) {
			return false
		}
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func asciiFold(value string) string {
	return strings.Map(func(c rune) rune {
		if c >= 'A' && c <= 'Z' {
			return c + ('a' - 'A')
		}
		return c
	}, value)
}

func parseManifest(content []byte) ([]manifestEntry, error) {
	if !utf8.Valid(content) {
		return nil, inputError("一覧はUTF-8で保存してください。", exitHashFormatError)
	}
	text := strings.TrimPrefix(string(content), "\ufeff")
	entries := []manifestEntry{}
	seen := map[string]bool{}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Strip CR only from a CRLF separator. A lone CR at EOF belongs
		// to the filename and must be rejected as a control character.
		if i < len(lines)-1 {
			line = strings.TrimSuffix(line, "\r")
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 67 || line[64] != ' ' || (line[65] != '*' && line[65] != ' ') {
			return nil, inputError("一覧の行形式が不正です。", exitHashFormatError)
		}
		expected, err := validateHash(line[:64], algorithms["sha256"])
		if err != nil {
			return nil, err
		}
		path := line[66:]
		if !portablePath(path) {
			return nil, inputError("一覧に使用できない相対パスがあります。", exitHashFormatError)
		}
		key := asciiFold(path)
		if seen[key] {
			return nil, inputError("一覧に重複パスがあります。", exitHashFormatError)
		}
		seen[key] = true
		entries = append(entries, manifestEntry{path: path, expected: expected})
	}
	if len(entries) == 0 {
		return nil, inputError("一覧が空です。", exitHashFormatError)
	}
	return entries, nil
}

func isLinkOrReparse(info os.FileInfo) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	// Sys is platform-specific. Read Windows' FILE_ATTRIBUTE_REPARSE_POINT
	// without introducing Windows-only imports, so the single-file CLI remains portable.
	if runtime.GOOS == "windows" {
		value := reflect.ValueOf(info.Sys())
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			value = value.Elem()
		}
		if value.Kind() == reflect.Struct {
			attrs := value.FieldByName("FileAttributes")
			if attrs.IsValid() && attrs.Kind() == reflect.Uint32 {
				return attrs.Uint()&0x400 != 0
			}
		}
	}
	return false
}

func safeManifestTarget(workspace, relative string) (string, error) {
	// Check every directory component, including parents of the workspace.
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return "", inputError("作業フォルダのパスを解決できません。", exitReadError)
	}
	parents := []string{}
	for current := absolute; ; current = filepath.Dir(current) {
		parents = append(parents, current)
		if filepath.Dir(current) == current {
			break
		}
	}
	for i := len(parents) - 1; i >= 0; i-- {
		info, err := os.Lstat(parents[i])
		if err != nil {
			return "", err
		}
		if isLinkOrReparse(info) {
			return "", inputError("リンクまたはreparse pointは一覧検証できません。", exitReadError)
		}
	}
	current := absolute
	parts := strings.Split(relative, "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if isLinkOrReparse(info) {
			return "", inputError("リンクまたはreparse pointは一覧検証できません。", exitReadError)
		}
		if (i < len(parts)-1 && !info.IsDir()) || (i == len(parts)-1 && !info.Mode().IsRegular()) {
			return "", inputError("対象は通常ファイルではありません。", exitReadError)
		}
	}
	return current, nil
}

func hashManifestReader(reader io.Reader, hasher hash.Hash) (string, int) {
	buffer := make([]byte, 1024*1024)
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			written, hashErr := hasher.Write(buffer[:n])
			if hashErr != nil || written != n {
				return "", exitHashCalculationError
			}
		}
		if readErr == io.EOF {
			return hex.EncodeToString(hasher.Sum(nil)), 0
		}
		if readErr != nil {
			return "", exitReadError
		}
	}
}

func verifyManifest(opts options, workspace string, report verificationReport) verificationReport {
	manifest, err := filepath.Abs(opts.Manifest)
	if err != nil {
		return failReport(report, inputError("一覧のパスを解決できません。", exitDiscoveryError))
	}
	manifest, err = safeManifestTarget(filepath.Dir(manifest), filepath.Base(manifest))
	if err != nil {
		return failReport(report, inputError("一覧または親フォルダを読み取れません。リンク・reparse pointは使用できません。", exitReadError))
	}
	content, err := os.ReadFile(manifest)
	if err != nil {
		return failReport(report, inputError("一覧ファイルを読み取れません。", exitReadError))
	}
	entries, err := parseManifest(content)
	if err != nil {
		return failReport(report, err)
	}
	for _, entry := range entries {
		expected := entry.expected
		result := verificationResult{Path: entry.path, Status: "match", ExpectedHash: &expected}
		target, err := safeManifestTarget(workspace, entry.path)
		code := 0
		if err != nil {
			code = exitReadError
			result.Status = "error"
			if errors.Is(err, os.ErrNotExist) {
				result.Status = "missing"
			}
		} else {
			// Read and hash errors have distinct aggregate exit codes.
			file, openErr := os.Open(target)
			if openErr != nil {
				code = exitReadError
				result.Status = "error"
			} else {
				actual, hashCode := hashManifestReader(file, algorithms["sha256"].NewHasher())
				closeErr := file.Close()
				if hashCode != 0 || closeErr != nil {
					code = hashCode
					if code == 0 {
						code = exitReadError
					}
					result.Status = "error"
				} else {
					result.ActualHash = &actual
					if actual != expected {
						result.Status = "mismatch"
						code = exitMismatch
					}
				}
			}
		}
		if result.Status == "error" || result.Status == "missing" {
			result.ErrorCode = &code
		}
		report.Results = append(report.Results, result)
		report.Summary[result.Status]++
		if code > report.ExitCode {
			report.ExitCode = code
		}
	}
	return report
}

func verifySingle(opts options, workspace string, report verificationReport) verificationReport {
	config := algorithms[opts.Algorithm]
	expected := ""
	var err error
	if opts.ExpectedSet {
		report.HashSource = "直接入力"
		expected, err = validateHash(opts.ExpectedHash, config)
	} else {
		hashFile := ""
		if opts.HashFile != "" {
			hashFile, err = resolvePath(opts.HashFile, workspace)
		} else {
			hashFile, err = discoverHashFile(workspace)
		}
		if err != nil {
			return failReport(report, err)
		}
		report.HashSource = filepath.Base(hashFile)
		expected, err = readVendorHash(hashFile, config)
	}
	if err != nil {
		return failReport(report, err)
	}
	target := ""
	if opts.TargetFile != "" {
		target, err = resolvePath(opts.TargetFile, workspace)
	} else {
		target, err = discoverTargetFile(workspace)
	}
	if err != nil {
		return failReport(report, err)
	}
	actual, err := calculateFileHash(target, config)
	if err != nil {
		return failReport(report, err)
	}
	result := verificationResult{Path: filepath.Base(target), Status: "match", ExpectedHash: &expected, ActualHash: &actual}
	if expected != actual {
		result.Status = "mismatch"
		report.ExitCode = exitMismatch
	}
	report.Results = append(report.Results, result)
	report.Summary[result.Status]++
	return report
}

func emitReport(writer io.Writer, opts options, report verificationReport) int {
	if opts.Format == "json" {
		if err := json.NewEncoder(writer).Encode(report); err != nil {
			return exitHashCalculationError
		}
	} else if report.Error != nil {
		fmt.Fprintf(writer, "[エラー] %s\n", report.Error.Message)
	} else {
		if !opts.Quiet {
			fmt.Fprintf(writer, "ハッシュ値確認ツール\n作業フォルダ: %s\nハッシュ方式: %s\n", opts.Workspace, strings.ToUpper(opts.Algorithm))
			if opts.ManifestSet {
				fmt.Fprintf(writer, "一覧ファイル: %s\n", filepath.Base(opts.Manifest))
			} else {
				fmt.Fprintf(writer, "ハッシュファイル: %s\n", report.HashSource)
			}
		}
		for _, result := range report.Results {
			if opts.Quiet && result.Status == "match" {
				continue
			}
			fmt.Fprintf(writer, "%s: %s\n", result.Path, result.Status)
			if result.ActualHash != nil {
				fmt.Fprintf(writer, "期待値: %s\n実測値: %s\n", *result.ExpectedHash, *result.ActualHash)
			}
		}
	}
	return report.ExitCode
}

func runWithWriter(args []string, writer io.Writer) int {
	opts, err := parseArgs(args)
	report := newReport(opts.Algorithm)
	if err != nil {
		return emitReport(writer, opts, failReport(report, inputError(err.Error(), exitDiscoveryError)))
	}
	if opts.Version {
		fmt.Fprintln(writer, version)
		return 0
	}
	if opts.Capabilities {
		err := json.NewEncoder(writer).Encode(struct {
			SchemaVersion int      `json:"schema_version"`
			Capabilities  []string `json:"capabilities"`
		}{1, []string{"single-verify", "expected-hash", "json-report", "manifest-verify"}})
		if err != nil {
			return exitHashCalculationError
		}
		return 0
	}
	workspace, err := filepath.Abs(opts.Workspace)
	if err != nil {
		return emitReport(writer, opts, failReport(report, inputError("作業フォルダのパスを解決できません。", exitDiscoveryError)))
	}
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		return emitReport(writer, opts, failReport(report, inputError("作業フォルダが見つかりません。", exitDiscoveryError)))
	}
	if opts.ManifestSet {
		report = verifyManifest(opts, workspace, report)
	} else {
		report = verifySingle(opts, workspace, report)
	}
	return emitReport(writer, opts, report)
}

func run(args []string) int { return runWithWriter(args, os.Stdout) }

func main() { os.Exit(run(os.Args[1:])) }
