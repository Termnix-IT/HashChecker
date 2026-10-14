[CmdletBinding()]
param(
    [string]$Workspace = $PSScriptRoot,
    [string]$HashFile,
    [string]$TargetFile,
    [AllowEmptyString()][string]$ExpectedHash,
    [string]$CsvPath,
    [switch]$PassThru
)
. (Join-Path $PSScriptRoot 'HashChecker.Common.ps1')
$verification = @{ Algorithm = 'SHA256'; Workspace = $Workspace }
foreach ($name in @('HashFile', 'TargetFile', 'ExpectedHash')) {
    if ($PSBoundParameters.ContainsKey($name)) { $verification[$name] = $PSBoundParameters[$name] }
}
$result = Invoke-HashVerification @verification
if ($CsvPath) {
    try { Export-HashVerificationResult $result $CsvPath }
    catch {
        $result.ExitCode = 3
        $result.Status = 'error'
        $result.ErrorMessage = 'CSV保存に失敗しました。保存先の権限と既存ファイルを確認してください。'
    }
}
if ($PassThru) { $result } else { Write-HashVerificationResult $result }
exit $result.ExitCode