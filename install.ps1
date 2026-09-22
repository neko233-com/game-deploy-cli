param(
    [string]$Version = $env:GAME_DEPLOY_VERSION,
    [string]$InstallDir = $env:GAME_DEPLOY_INSTALL_DIR
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($Version)) { $Version = 'latest' }
if ([string]::IsNullOrWhiteSpace($InstallDir)) { $InstallDir = Join-Path $env:LOCALAPPDATA 'game-deploy\bin' }

$repo = if ($env:GAME_DEPLOY_REPO) { $env:GAME_DEPLOY_REPO } else { 'neko233-com/game-deploy-cli' }
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
if ($Version -eq 'latest') {
    $baseUrl = "https://github.com/$repo/releases/latest/download"
} else {
    $baseUrl = "https://github.com/$repo/releases/download/v$($Version.TrimStart('v'))"
}
$asset = "game-deploy-windows-$arch.zip"
$temp = Join-Path ([System.IO.Path]::GetTempPath()) ("game-deploy-" + [guid]::NewGuid().ToString('N'))
$zip = Join-Path $temp $asset
$extract = Join-Path $temp 'extract'

try {
    New-Item -ItemType Directory -Force -Path $temp | Out-Null
    New-Item -ItemType Directory -Force -Path $extract | Out-Null
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $zip
    Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -LiteralPath (Join-Path $extract 'game-deploy.exe') -Destination (Join-Path $InstallDir 'game-deploy.exe') -Force

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @($userPath -split ';' | Where-Object { $_ -and $_.Trim() })
    if ($entries -notcontains $InstallDir) {
        [Environment]::SetEnvironmentVariable('Path', (($entries + $InstallDir) -join ';'), 'User')
    }
    $env:Path = "$InstallDir;$env:Path"
    Write-Output "game-deploy installed to $(Join-Path $InstallDir 'game-deploy.exe')"
    Write-Output 'Open a new PowerShell window to use the updated PATH.'
} finally {
    if (Test-Path -LiteralPath $temp) { Remove-Item -LiteralPath $temp -Recurse -Force }
}
