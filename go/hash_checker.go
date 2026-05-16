package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	Algorithm  string
	Workspace  string
	HashFile   string
	TargetFile string
}

func parseArgs(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("hash_checker", flag.ContinueOnError)
	flags.StringVar(&opts.Algorithm, "algorithm", "", "Hash algorithm to use.")
	flags.StringVar(&opts.Algorithm, "a", "", "Hash algorithm to use.")
	flags.StringVar(&opts.Workspace, "workspace", ".", "Workspace used for automatic file discovery.")
	flags.StringVar(&opts.Workspace, "w", ".", "Workspace used for automatic file discovery.")
	flags.StringVar(&opts.HashFile, "hash-file", "", "Text file containing the vendor-provided hash.")
	flags.StringVar(&opts.TargetFile, "target-file", "", "File to calculate and verify.")

	if err := flags.Parse(args); err != nil {
		return opts, err
	}

	opts.Algorithm = strings.ToLower(opts.Algorithm)
	if _, ok := algorithms[opts.Algorithm]; !ok {
		return opts, fmt.Errorf("ハッシュ方式を指定してください。対応方式: md5, sha256")
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

func printHeader(workspace string, algorithmLabel string) {
	fmt.Println("========================================")
	fmt.Println(" ハッシュ値確認ツール")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Printf("作業フォルダ : %s\n", workspace)
	fmt.Printf("ハッシュ方式 : %s\n", algorithmLabel)
	fmt.Println()
}

func printResult(matched bool, workspace string, algorithmLabel string, hashFile string, targetFile string, vendorHash string, actualHash string) {
	if matched {
		fmt.Println("[正常] ハッシュ値が一致しました。")
	} else {
		fmt.Println("[警告] ハッシュ値が一致しません。")
	}

	fmt.Println()
	fmt.Printf("作業フォルダ       : %s\n", workspace)
	fmt.Printf("ハッシュ方式       : %s\n", algorithmLabel)
	fmt.Printf("ハッシュファイル   : %s\n", filepath.Base(hashFile))
	fmt.Printf("確認対象ファイル   : %s\n", filepath.Base(targetFile))
	fmt.Println()
	fmt.Printf("ベンダー提供ハッシュ値 : %s\n", vendorHash)
	fmt.Printf("算出ハッシュ値         : %s\n", actualHash)
	fmt.Println()

	if matched {
		fmt.Println("結果 : ファイルはベンダー提供ハッシュ値と一致しています。")
	} else {
		fmt.Println("結果 : ファイルが破損している、または想定と異なる可能性があります。")
		fmt.Println("確認 : ベンダー提供値、確認対象ファイル、ハッシュ方式を確認してください。")
	}
}

func run(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Printf("[エラー] %v\n\n", err)
		fmt.Println("処理を終了します。")
		return exitDiscoveryError
	}

	config := algorithms[opts.Algorithm]
	workspace, err := filepath.Abs(opts.Workspace)
	if err != nil {
		fmt.Printf("[エラー] 作業フォルダのパスを解決できません: %s\n\n", opts.Workspace)
		fmt.Println("処理を終了します。")
		return exitDiscoveryError
	}

	printHeader(workspace, config.Label)

	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		fmt.Printf("[エラー] 作業フォルダが見つかりません: %s\n\n", workspace)
		fmt.Println("処理を終了します。")
		return exitDiscoveryError
	}

	hashFile := ""
	if opts.HashFile != "" {
		hashFile, err = resolvePath(opts.HashFile, workspace)
	} else {
		hashFile, err = discoverHashFile(workspace)
	}
	if err != nil {
		return printError(err)
	}

	targetFile := ""
	if opts.TargetFile != "" {
		targetFile, err = resolvePath(opts.TargetFile, workspace)
	} else {
		targetFile, err = discoverTargetFile(workspace)
	}
	if err != nil {
		return printError(err)
	}

	vendorHash, err := readVendorHash(hashFile, config)
	if err != nil {
		return printError(err)
	}

	actualHash, err := calculateFileHash(targetFile, config)
	if err != nil {
		return printError(err)
	}

	matched := vendorHash == actualHash
	printResult(matched, workspace, config.Label, hashFile, targetFile, vendorHash, actualHash)
	if matched {
		return 0
	}
	return exitMismatch
}

func printError(err error) int {
	var checkerErr *hashCheckerError
	if errors.As(err, &checkerErr) {
		fmt.Printf("[エラー] %s\n\n", checkerErr.Message)
		fmt.Println("処理を終了します。")
		return checkerErr.ExitCode
	}

	fmt.Println("[エラー] 想定外のエラーが発生しました。")
	fmt.Println()
	fmt.Println(err)
	return exitHashCalculationError
}

func main() {
	os.Exit(run(os.Args[1:]))
}
