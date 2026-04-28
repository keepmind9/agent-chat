$ErrorActionPreference = "Stop"

$Repo = "keepmind9/agent-chat"
$Binary = "agent-chat.exe"
$InstallDir = "$env:USERPROFILE\.local\bin"

# --- Detect arch ---
$Arch = "amd64"

# --- Fetch latest release ---
Write-Host "Fetching latest release from $Repo..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Latest = $Release.tag_name -replace '^v', ''

# --- Check existing install ---
$Existing = Get-Command $Binary -ErrorAction SilentlyContinue
if ($Existing) {
    $Current = & $Binary version 2>$null | Select-String "^Version:" | ForEach-Object { ($_ -split '\s+')[1] }
    if ($Current -eq $Latest) {
        Write-Host "$Binary is already up to date (v$Current)"
        exit 0
    }
}

# --- Find matching asset ---
$Pattern = "agent-chat-$Latest-windows-$Arch"
$Asset = $Release.assets | Where-Object { $_.name -like "$Pattern*.zip" } | Select-Object -First 1
if (-not $Asset) {
    Write-Error "No asset found for windows-$Arch"
    exit 1
}

# --- Download and extract ---
$TmpDir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "agent-chat-install") -Force
$Archive = Join-Path $TmpDir $Asset.name

Write-Host "Downloading $($Asset.name)..."
Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $Archive

Write-Host "Extracting..."
Expand-Archive -Path $Archive -DestinationPath $TmpDir -Force

# --- Install ---
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
$ExePath = Join-Path $InstallDir $Binary
Copy-Item -Path (Join-Path $TmpDir "$Pattern\$Binary") -Destination $ExePath -Force

Write-Host "Installed $Binary v$Latest to $InstallDir"

# --- Update PATH ---
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$InstallPath;$UserPath", "User")
    Write-Host "Added $InstallDir to user PATH"
    Write-Host "Open a new terminal for PATH to take effect"
}

# --- Cleanup ---
Remove-Item -Path $TmpDir -Recurse -Force

Write-Host "Done! Run: agent-chat version"
