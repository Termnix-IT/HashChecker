using System.Security.Cryptography;
using System.Text.Json;
using System.Diagnostics;

namespace HashChecker.Core;

public sealed class VerificationException(string message, int exitCode) : Exception(message)
{
    public int ExitCode { get; } = exitCode;
}

public sealed record VerificationResult(string Path, string Status, string? ExpectedHash,
    string? ActualHash, int? ErrorCode = null);
public sealed record ReportError(int Code, string Message);
public sealed record VerificationReport(int SchemaVersion, string? Algorithm, string Operation,
    IReadOnlyList<VerificationResult> Results, IReadOnlyDictionary<string, int> Summary,
    int ExitCode, ReportError? Error)
{
    public static VerificationReport Single(string algorithm, VerificationResult result) =>
        Create(algorithm, [result], result.Status == "match" ? 0 : 1, null);

    public static VerificationReport Failure(string? algorithm, int code, string message) =>
        Create(algorithm, [], code, new(code, message));

    private static VerificationReport Create(string? algorithm, IReadOnlyList<VerificationResult> results,
        int code, ReportError? error)
    {
        var summary = new Dictionary<string, int>();
        foreach (var state in new[] { "match", "mismatch", "missing", "error", "added", "removed", "changed" })
            summary[state] = results.Count(result => result.Status == state);
        return new(1, algorithm, "verify", results, summary, code, error);
    }

    public string ToJson() => JsonSerializer.Serialize(this, new JsonSerializerOptions
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        WriteIndented = true,
    });
}

public static class Verifier
{
    public static bool IsSupportedAlgorithm(string algorithm) =>
        algorithm.Equals("md5", StringComparison.OrdinalIgnoreCase) ||
        algorithm.Equals("sha256", StringComparison.OrdinalIgnoreCase);

    public static string NormalizeExpectedHash(string value, string algorithm)
    {
        if (!IsSupportedAlgorithm(algorithm))
            throw new VerificationException("対応するハッシュ方式は md5, sha256 です。", 2);
        var normalized = value.Trim();
        var length = algorithm.Equals("md5", StringComparison.OrdinalIgnoreCase) ? 32 : 64;
        if (normalized.Length != length)
            throw new VerificationException($"ハッシュ値は {length} 文字で指定してください。", 4);
        if (!normalized.All(character => character is >= '0' and <= '9' or >= 'a' and <= 'f' or >= 'A' and <= 'F'))
            throw new VerificationException("ハッシュ値には 0-9、a-f、A-F のみ使用できます。", 4);
        return normalized.ToLowerInvariant();
    }

    public static string ReadExpectedHash(string hashFile, string algorithm)
    {
        string text;
        try { text = File.ReadAllText(hashFile); }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        { throw new VerificationException("ハッシュファイルを読み取れません。権限や状態を確認してください。", 3); }
        var lines = text.TrimStart('\uFEFF').Split(['\r', '\n'], StringSplitOptions.RemoveEmptyEntries)
            .Select(line => line.Trim()).Where(line => line.Length > 0).ToArray();
        if (lines.Length != 1)
            throw new VerificationException("ハッシュ値のみを1行で記載してください。", 4);
        return NormalizeExpectedHash(lines[0], algorithm);
    }

    public static async Task<VerificationResult> VerifyAsync(string targetFile, string expectedHash,
        string algorithm, IProgress<double>? progress = null, CancellationToken cancellationToken = default)
    {
        var expected = NormalizeExpectedHash(expectedHash, algorithm);
        cancellationToken.ThrowIfCancellationRequested();
        try
        {
            using var hasher = IncrementalHash.CreateHash(algorithm.Equals("md5", StringComparison.OrdinalIgnoreCase)
                ? HashAlgorithmName.MD5 : HashAlgorithmName.SHA256);
            await using var stream = new FileStream(targetFile, FileMode.Open, FileAccess.Read, FileShare.Read,
                128 * 1024, FileOptions.Asynchronous | FileOptions.SequentialScan);
            var buffer = new byte[128 * 1024];
            var total = stream.Length;
            long completed = 0;
            var progressClock = Stopwatch.StartNew();
            int read;
            while ((read = await stream.ReadAsync(buffer, cancellationToken).ConfigureAwait(false)) > 0)
            {
                cancellationToken.ThrowIfCancellationRequested();
                hasher.AppendData(buffer.AsSpan(0, read));
                completed += read;
                if (completed == read || progressClock.ElapsedMilliseconds >= 100)
                {
                    progress?.Report(total == 0 ? 100 : Math.Min(100, completed * 100.0 / total));
                    progressClock.Restart();
                }
            }
            cancellationToken.ThrowIfCancellationRequested();
            progress?.Report(100);
            var actual = Convert.ToHexStringLower(hasher.GetHashAndReset());
            return new(Path.GetFileName(targetFile), actual == expected ? "match" : "mismatch", expected, actual);
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            // Preserve the existing single-file CLI's calculation-error exit code.
            throw new VerificationException("確認対象ファイルのハッシュ値を算出できません。権限や状態を確認してください。", 5);
        }
        catch (CryptographicException)
        { throw new VerificationException("ハッシュ値の算出に失敗しました。", 5); }
    }
}
