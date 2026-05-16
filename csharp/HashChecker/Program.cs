using System.Security.Cryptography;

namespace HashChecker;

internal sealed record AlgorithmConfig(string Label, int ExpectedLength, Func<HashAlgorithm> CreateHasher);

internal sealed record Options(string Algorithm, string Workspace, string? HashFile, string? TargetFile);

internal sealed class HashCheckerException(string message, int exitCode) : Exception(message)
{
    public int ExitCode { get; } = exitCode;
}

internal static class Program
{
    private static readonly Dictionary<string, AlgorithmConfig> Algorithms = new(StringComparer.OrdinalIgnoreCase)
    {
        ["md5"] = new("MD5", 32, MD5.Create),
        ["sha256"] = new("SHA256", 64, SHA256.Create),
    };

    private static readonly HashSet<string> ExcludedExtensions = new(StringComparer.OrdinalIgnoreCase)
    {
        ".bat",
        ".cmd",
        ".cs",
        ".csproj",
        ".dll",
        ".exe",
        ".go",
        ".log",
        ".pdb",
        ".ps1",
        ".py",
        ".txt",
    };

    private const int ExitMismatch = 1;
    private const int ExitDiscoveryError = 2;
    private const int ExitReadError = 3;
    private const int ExitHashFormatError = 4;
    private const int ExitHashCalculationError = 5;

    private static int Main(string[] args)
    {
        try
        {
            return Run(args);
        }
        catch (HashCheckerException ex)
        {
            Console.WriteLine($"[エラー] {ex.Message}");
            Console.WriteLine();
            Console.WriteLine("処理を終了します。");
            return ex.ExitCode;
        }
        catch (Exception ex)
        {
            Console.WriteLine("[エラー] 想定外のエラーが発生しました。");
            Console.WriteLine();
            Console.WriteLine(ex.Message);
            return ExitHashCalculationError;
        }
    }

    internal static int Run(string[] args)
    {
        var options = ParseArgs(args);
        var config = Algorithms[options.Algorithm];
        var workspace = Path.GetFullPath(options.Workspace);

        PrintHeader(workspace, config.Label);

        if (!Directory.Exists(workspace))
        {
            throw new HashCheckerException($"作業フォルダが見つかりません: {workspace}", ExitDiscoveryError);
        }

        var hashFile = options.HashFile is not null
            ? ResolvePath(options.HashFile, workspace)
            : DiscoverHashFile(workspace);
        var targetFile = options.TargetFile is not null
            ? ResolvePath(options.TargetFile, workspace)
            : DiscoverTargetFile(workspace);

        var vendorHash = ReadVendorHash(hashFile, config);
        var actualHash = CalculateFileHash(targetFile, config);
        var matched = string.Equals(vendorHash, actualHash, StringComparison.Ordinal);

        PrintResult(matched, workspace, config.Label, hashFile, targetFile, vendorHash, actualHash);
        return matched ? 0 : ExitMismatch;
    }

    private static Options ParseArgs(string[] args)
    {
        string? algorithm = null;
        var workspace = ".";
        string? hashFile = null;
        string? targetFile = null;

        for (var i = 0; i < args.Length; i++)
        {
            var arg = args[i];
            switch (arg)
            {
                case "-a":
                case "--algorithm":
                    algorithm = ReadOptionValue(args, ref i, arg);
                    break;
                case "-w":
                case "--workspace":
                    workspace = ReadOptionValue(args, ref i, arg);
                    break;
                case "--hash-file":
                    hashFile = ReadOptionValue(args, ref i, arg);
                    break;
                case "--target-file":
                    targetFile = ReadOptionValue(args, ref i, arg);
                    break;
                default:
                    throw new HashCheckerException($"不明な引数です: {arg}", ExitDiscoveryError);
            }
        }

        if (string.IsNullOrWhiteSpace(algorithm) || !Algorithms.ContainsKey(algorithm))
        {
            throw new HashCheckerException("ハッシュ方式を指定してください。対応方式: md5, sha256", ExitDiscoveryError);
        }

        return new Options(algorithm.ToLowerInvariant(), workspace, hashFile, targetFile);
    }

    private static string ReadOptionValue(string[] args, ref int index, string optionName)
    {
        if (index + 1 >= args.Length || args[index + 1].StartsWith("-", StringComparison.Ordinal))
        {
            throw new HashCheckerException($"{optionName} の値を指定してください。", ExitDiscoveryError);
        }

        index++;
        return args[index];
    }

    private static string ResolvePath(string path, string workspace)
    {
        return Path.GetFullPath(Path.IsPathRooted(path) ? path : Path.Combine(workspace, path));
    }

    private static string DiscoverHashFile(string workspace)
    {
        var hashFiles = Directory
            .EnumerateFiles(workspace, "*.txt", SearchOption.TopDirectoryOnly)
            .OrderBy(path => path, StringComparer.OrdinalIgnoreCase)
            .ToList();

        return hashFiles.Count switch
        {
            0 => throw new HashCheckerException(
                "ハッシュファイルが見つかりません。作業フォルダに txt ファイルを1つ配置してください。",
                ExitDiscoveryError),
            1 => hashFiles[0],
            _ => throw new HashCheckerException(
                $"ハッシュファイル候補が複数見つかりました。\n\n見つかった txt ファイル:\n{ListFiles(hashFiles)}\n\n対応: ハッシュ値を記載した txt ファイルを1つだけ残してください。",
                ExitDiscoveryError),
        };
    }

    private static string DiscoverTargetFile(string workspace)
    {
        var targetFiles = Directory
            .EnumerateFiles(workspace, "*", SearchOption.TopDirectoryOnly)
            .Where(path => !ShouldExcludeTargetFile(path))
            .OrderBy(path => path, StringComparer.OrdinalIgnoreCase)
            .ToList();

        return targetFiles.Count switch
        {
            0 => throw new HashCheckerException(
                "確認対象ファイルが見つかりません。作業フォルダに確認したいファイルを1つ配置してください。",
                ExitDiscoveryError),
            1 => targetFiles[0],
            _ => throw new HashCheckerException(
                $"確認対象ファイル候補が複数見つかりました。\n\n見つかったファイル:\n{ListFiles(targetFiles)}\n\n対応: 確認したいファイルを1つだけ残してください。",
                ExitDiscoveryError),
        };
    }

    private static bool ShouldExcludeTargetFile(string path)
    {
        var fileName = Path.GetFileName(path);
        if (fileName.EndsWith(".deps.json", StringComparison.OrdinalIgnoreCase)
            || fileName.EndsWith(".runtimeconfig.json", StringComparison.OrdinalIgnoreCase))
        {
            return true;
        }

        return ExcludedExtensions.Contains(Path.GetExtension(path));
    }

    private static string ListFiles(IEnumerable<string> files)
    {
        return string.Join(Environment.NewLine, files.Select(path => $"- {Path.GetFileName(path)}"));
    }

    private static string ReadVendorHash(string hashFile, AlgorithmConfig config)
    {
        string rawText;
        try
        {
            rawText = File.ReadAllText(hashFile);
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            throw new HashCheckerException(
                $"ハッシュファイルを読み取れませんでした。ファイルの権限や状態を確認してください。\n{ex.Message}",
                ExitReadError);
        }

        var lines = rawText
            .TrimStart('\uFEFF')
            .Split(["\r\n", "\n", "\r"], StringSplitOptions.None)
            .Select(line => line.Trim())
            .Where(line => line.Length > 0)
            .ToList();

        if (lines.Count == 0)
        {
            throw new HashCheckerException("ハッシュファイルにハッシュ値が記載されていません。", ExitHashFormatError);
        }

        if (lines.Count >= 2)
        {
            throw new HashCheckerException(
                "ハッシュファイルに複数行の値が記載されています。\n\n対応: ハッシュ値のみを1行で記載してください。",
                ExitHashFormatError);
        }

        var vendorHash = lines[0];
        if (vendorHash.Length != config.ExpectedLength)
        {
            throw new HashCheckerException(
                $"{config.Label} ハッシュ値の文字数が不正です。\n\n期待する文字数 : {config.ExpectedLength} 文字\n実際の文字数   : {vendorHash.Length} 文字",
                ExitHashFormatError);
        }

        if (!vendorHash.All(Uri.IsHexDigit))
        {
            throw new HashCheckerException(
                "ハッシュ値の形式が不正です。\n\nハッシュ値には 0-9、a-f、A-F のみ使用できます。",
                ExitHashFormatError);
        }

        return vendorHash.ToLowerInvariant();
    }

    private static string CalculateFileHash(string targetFile, AlgorithmConfig config)
    {
        try
        {
            using var hasher = config.CreateHasher();
            using var stream = File.OpenRead(targetFile);
            var hashBytes = hasher.ComputeHash(stream);
            return Convert.ToHexString(hashBytes).ToLowerInvariant();
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            throw new HashCheckerException(
                $"確認対象ファイルのハッシュ値を算出できませんでした。ファイルの権限や状態を確認してください。\n{ex.Message}",
                ExitHashCalculationError);
        }
    }

    private static void PrintHeader(string workspace, string algorithmLabel)
    {
        Console.WriteLine("========================================");
        Console.WriteLine(" ハッシュ値確認ツール");
        Console.WriteLine("========================================");
        Console.WriteLine();
        Console.WriteLine($"作業フォルダ : {workspace}");
        Console.WriteLine($"ハッシュ方式 : {algorithmLabel}");
        Console.WriteLine();
    }

    private static void PrintResult(
        bool matched,
        string workspace,
        string algorithmLabel,
        string hashFile,
        string targetFile,
        string vendorHash,
        string actualHash)
    {
        Console.WriteLine(matched ? "[正常] ハッシュ値が一致しました。" : "[警告] ハッシュ値が一致しません。");
        Console.WriteLine();
        Console.WriteLine($"作業フォルダ       : {workspace}");
        Console.WriteLine($"ハッシュ方式       : {algorithmLabel}");
        Console.WriteLine($"ハッシュファイル   : {Path.GetFileName(hashFile)}");
        Console.WriteLine($"確認対象ファイル   : {Path.GetFileName(targetFile)}");
        Console.WriteLine();
        Console.WriteLine($"ベンダー提供ハッシュ値 : {vendorHash}");
        Console.WriteLine($"算出ハッシュ値         : {actualHash}");
        Console.WriteLine();

        if (matched)
        {
            Console.WriteLine("結果 : ファイルはベンダー提供ハッシュ値と一致しています。");
            return;
        }

        Console.WriteLine("結果 : ファイルが破損している、または想定と異なる可能性があります。");
        Console.WriteLine("確認 : ベンダー提供値、確認対象ファイル、ハッシュ方式を確認してください。");
    }
}
