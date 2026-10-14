using System.Text.Json;
using HashChecker.Core;

namespace HashChecker;

internal sealed record Options(string Algorithm, string Workspace, string? HashFile,
    string? TargetFile, string? ExpectedHash, string Format, bool Version, bool Capabilities);

internal static class Program
{
    private static readonly HashSet<string> ExcludedExtensions = new(StringComparer.OrdinalIgnoreCase)
    { ".bat", ".cmd", ".cs", ".csproj", ".dll", ".exe", ".go", ".log", ".pdb", ".ps1", ".py", ".txt" };

    private static async Task<int> Main(string[] args)
    {
        // Retain JSON output even when parsing another argument fails.
        var json = args.Zip(args.Skip(1)).Any(pair => pair.First == "--format" && pair.Second == "json");
        string? algorithm = args.Zip(args.Skip(1))
            .Where(pair => pair.First is "--algorithm" or "-a")
            .Select(pair => pair.Second.ToLowerInvariant()).LastOrDefault();
        if (algorithm is null || !Verifier.IsSupportedAlgorithm(algorithm)) algorithm = null;
        try
        {
            var options = ParseArgs(args);
            json = options.Format == "json";
            algorithm = options.Algorithm;
            if (options.Version) { Console.WriteLine("0.2.0"); return 0; }
            if (options.Capabilities)
            {
                Console.WriteLine(JsonSerializer.Serialize(new { schema_version = 1,
                    capabilities = new[] { "single-verify", "expected-hash", "json-report" } }));
                return 0;
            }
            var workspace = Path.GetFullPath(options.Workspace);
            if (!Directory.Exists(workspace)) throw new VerificationException("作業フォルダが見つかりません。", 2);
            var hashFile = options.ExpectedHash is not null ? null : options.HashFile is not null
                ? ResolvePath(options.HashFile, workspace) : DiscoverFile(workspace, true);
            var targetFile = options.TargetFile is not null
                ? ResolvePath(options.TargetFile, workspace) : DiscoverFile(workspace, false);
            var expected = options.ExpectedHash is not null
                ? Verifier.NormalizeExpectedHash(options.ExpectedHash, algorithm)
                : Verifier.ReadExpectedHash(hashFile!, algorithm);
            var result = await Verifier.VerifyAsync(targetFile, expected, algorithm);
            var report = VerificationReport.Single(algorithm, result);
            if (json) Console.WriteLine(report.ToJson());
            else
            {
                Console.WriteLine("ハッシュ値確認ツール");
                Console.WriteLine(result.Status == "match" ? "[正常] ハッシュ値が一致しました。" : "[警告] ハッシュ値が一致しません。");
                Console.WriteLine($"作業フォルダ : {workspace}");
                Console.WriteLine($"ハッシュ方式 : {algorithm.ToUpperInvariant()}");
                Console.WriteLine($"ハッシュファイル : {(hashFile is null ? "直接入力" : Path.GetFileName(hashFile))}");
                Console.WriteLine($"確認対象ファイル : {result.Path}");
                Console.WriteLine($"ベンダー提供ハッシュ値 : {result.ExpectedHash}");
                Console.WriteLine($"算出ハッシュ値 : {result.ActualHash}");
            }
            return report.ExitCode;
        }
        catch (VerificationException ex) { return PrintError(algorithm, json, ex.ExitCode, ex.Message); }
        catch (Exception) { return PrintError(algorithm, json, 5, "想定外のエラーが発生しました。"); }
    }

    private static int PrintError(string? algorithm, bool json, int code, string message)
    {
        Console.WriteLine(json ? VerificationReport.Failure(algorithm, code, message).ToJson() : $"[エラー] {message}");
        return code;
    }

    private static Options ParseArgs(string[] args)
    {
        var algorithm = "";
        var workspace = ".";
        string? hashFile = null, targetFile = null, expected = null;
        var format = "text";
        var version = false;
        var capabilities = false;
        for (var i = 0; i < args.Length; i++)
        {
            switch (args[i])
            {
                case "--version": version = true; break;
                case "--capabilities": capabilities = true; break;
                case "-a": case "--algorithm": algorithm = ReadValue(args, ref i).ToLowerInvariant(); break;
                case "-w": case "--workspace": workspace = ReadValue(args, ref i); break;
                case "--hash-file": hashFile = ReadValue(args, ref i); break;
                case "--target-file": targetFile = ReadValue(args, ref i); break;
                case "--expected-hash": expected = ReadValue(args, ref i); break;
                case "--format": format = ReadValue(args, ref i); break;
                default: throw new VerificationException("不明な引数です。--capabilities で対応機能を確認してください。", 2);
            }
        }
        if (format is not ("text" or "json")) throw new VerificationException("--format は text または json を指定してください。", 2);
        if (hashFile is not null && expected is not null) throw new VerificationException("--hash-file と --expected-hash は同時指定できません。", 2);
        if (!version && !capabilities && !Verifier.IsSupportedAlgorithm(algorithm))
            throw new VerificationException("ハッシュ方式を指定してください。対応方式: md5, sha256", 2);
        return new(algorithm, workspace, hashFile, targetFile, expected, format, version, capabilities);
    }

    private static string ReadValue(string[] args, ref int index)
    {
        if (index + 1 >= args.Length || args[index + 1].StartsWith('-'))
            throw new VerificationException("引数の値が指定されていません。", 2);
        return args[++index];
    }

    private static string ResolvePath(string path, string workspace) =>
        Path.GetFullPath(Path.IsPathRooted(path) ? path : Path.Combine(workspace, path));

    private static string DiscoverFile(string workspace, bool hashFile)
    {
        List<string> files;
        try
        {
            files = Directory.EnumerateFiles(workspace).Where(path => hashFile
                ? Path.GetExtension(path).Equals(".txt", StringComparison.OrdinalIgnoreCase)
                : !ExcludedExtensions.Contains(Path.GetExtension(path))
                    && !path.EndsWith(".deps.json", StringComparison.OrdinalIgnoreCase)
                    && !path.EndsWith(".runtimeconfig.json", StringComparison.OrdinalIgnoreCase)).ToList();
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        { throw new VerificationException("作業フォルダを読み取れません。", 2); }
        if (files.Count != 1)
            throw new VerificationException(hashFile
                ? "ハッシュファイル候補は1つだけ配置するか --hash-file を指定してください。"
                : "確認対象ファイル候補は1つだけ配置するか --target-file を指定してください。", 2);
        return files[0];
    }
}
