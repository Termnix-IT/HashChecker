@echo off
cd /d "%~dp0"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Verify-MD5Hash.ps1"

set RESULT=%ERRORLEVEL%

echo.
pause

exit /b %RESULT%