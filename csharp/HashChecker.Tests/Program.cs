using System.Text.Json;
using HashChecker.Core;

var folder = Path.Combine(Path.GetTempPath(), "hashchecker-tests-" + Guid.NewGuid().ToString("N"));
Directory.CreateDirectory(folder);
var checks = 0;
void Check(bool condition, string name)
{
    if (!condition) throw new InvalidOperationException(name);
    checks++;
}
void ExpectError(Action action, int code)
{
    try { action(); throw new InvalidOperationException("Expected validation failure"); }
    catch (VerificationException ex) { Check(ex.ExitCode == code, "Exit code"); }
}
try
{
    var file = Path.Combine(folder, "sample.bin");
    await File.WriteAllBytesAsync(file, "abc"u8.ToArray());
    const string sha = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad";
    const string md5 = "900150983cd24fb0d6963f7d28e17f72";
    var result = await Verifier.VerifyAsync(file, " " + sha.ToUpperInvariant() + " ", "sha256");
    Check(result.Status == "match" && result.ActualHash == sha && result.Path == "sample.bin", "SHA256 published vector");
    result = await Verifier.VerifyAsync(file, md5, "MD5");
    Check(result.Status == "match" && result.ActualHash == md5, "MD5 published vector");
    result = await Verifier.VerifyAsync(file, new string('0', 32), "md5");
    Check(result.Status == "mismatch", "Mismatch");
    ExpectError(() => Verifier.NormalizeExpectedHash("abc", "sha256"), 4);
    ExpectError(() => Verifier.NormalizeExpectedHash(new string('ａ', 32), "md5"), 4);
    ExpectError(() => Verifier.NormalizeExpectedHash(md5, "sha1"), 2);
    var hashFile = Path.Combine(folder, "vendor.txt");
    await File.WriteAllTextAsync(hashFile, "\uFEFF\n " + sha + " \r\n\n");
    Check(Verifier.ReadExpectedHash(hashFile, "sha256") == sha, "BOM and blank lines");
    await File.WriteAllTextAsync(hashFile, sha + "\n" + sha);
    ExpectError(() => Verifier.ReadExpectedHash(hashFile, "sha256"), 4);
    ExpectError(() => Verifier.ReadExpectedHash(Path.Combine(folder, "missing"), "sha256"), 3);
    try { await Verifier.VerifyAsync(Path.Combine(folder, "missing"), sha, "sha256"); throw new InvalidOperationException("Expected missing error"); }
    catch (VerificationException ex) { Check(ex.ExitCode == 5 && !ex.Message.Contains(folder), "Safe calculation error"); }
    await File.WriteAllBytesAsync(file, []);
    result = await Verifier.VerifyAsync(file, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "sha256");
    Check(result.Status == "match", "Empty file");
    await File.WriteAllBytesAsync(file, new byte[1024 * 1024]);
    using var source = new CancellationTokenSource();
    var progress = new InlineProgress(value => { Check(value > 0 && value <= 100, "Progress range"); source.Cancel(); });
    try { await Verifier.VerifyAsync(file, sha, "sha256", progress, source.Token); throw new InvalidOperationException("Expected cancel"); }
    catch (OperationCanceledException) { Check(true, "Cancellation during chunks"); }
    // Disposal after cancellation allows exclusive access and a subsequent verification.
    using (File.Open(file, FileMode.Open, FileAccess.ReadWrite, FileShare.None)) { Check(true, "Cancelled stream disposed"); }
    using var document = JsonDocument.Parse(VerificationReport.Single("sha256", result).ToJson());
    var root = document.RootElement;
    Check(root.GetProperty("schema_version").GetInt32() == 1 && root.GetProperty("results")[0].GetProperty("error_code").ValueKind == JsonValueKind.Null, "JSON contract");
    Check(root.GetProperty("summary").EnumerateObject().Count() == 7 && root.GetProperty("summary").GetProperty("match").GetInt32() == 1, "All summary states");
    using var errorDocument = JsonDocument.Parse(VerificationReport.Failure("sha256", 3, "読み取れません。").ToJson());
    Check(errorDocument.RootElement.GetProperty("results").GetArrayLength() == 0 && errorDocument.RootElement.GetProperty("exit_code").GetInt32() == 3, "Error contract");
    using var unknownDocument = JsonDocument.Parse(VerificationReport.Failure(null, 2, "方式を指定してください。").ToJson());
    Check(unknownDocument.RootElement.GetProperty("algorithm").ValueKind == JsonValueKind.Null, "Unknown algorithm is null");
    Console.WriteLine($"C# Core: {checks} checks passed.");
}
finally { Directory.Delete(folder, true); }

sealed class InlineProgress(Action<double> report) : IProgress<double>
{
    public void Report(double value) => report(value);
}
