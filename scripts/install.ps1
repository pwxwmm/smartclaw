$ErrorActionPreference = "Stop"
$Repo = "instructkr/smartclaw"
$Binary = "smartclaw.exe"

$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "arm64" }

$Latest = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
if (-not $Latest) { Write-Error "Could not determine latest version"; exit 1 }

$Url = "https://github.com/$Repo/releases/download/$Latest/smartclaw_windows_${Arch}.zip"
$InstallDir = "$env:USERPROFILE\.local\bin"
$TmpFile = "$env:TEMP\smartclaw.zip"

Write-Host "Installing SmartClaw $Latest for windows/$Arch..."
Invoke-WebRequest -Uri $Url -OutFile $TmpFile
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Expand-Archive -Path $TmpFile -DestinationPath $InstallDir -Force
Remove-Item $TmpFile

$BinPath = Join-Path $InstallDir $Binary
if (-not ($env:PATH -like "*$InstallDir*")) {
    [Environment]::SetEnvironmentVariable("PATH", "$InstallDir;$env:PATH", "User")
    Write-Host "Added $InstallDir to PATH. Restart your terminal."
}

Write-Host "SmartClaw installed to $BinPath"
& $BinPath version
