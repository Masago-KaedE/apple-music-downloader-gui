$ErrorActionPreference = 'Stop'

$wails = Get-Command wails.exe -ErrorAction SilentlyContinue
if (-not $wails) {
    $goPath = & 'C:\Go\bin\go.exe' env GOPATH
    $wailsPath = Join-Path $goPath 'bin\wails.exe'
    if (-not (Test-Path -LiteralPath $wailsPath)) { throw 'Wails v2 CLI was not found.' }
} else {
    $wailsPath = $wails.Source
}

$makensis = Get-Command makensis.exe -ErrorAction SilentlyContinue
if ($makensis) {
    $makensisPath = $makensis.Source
} elseif (Test-Path -LiteralPath 'C:\Program Files (x86)\NSIS\makensis.exe') {
    $makensisPath = 'C:\Program Files (x86)\NSIS\makensis.exe'
} elseif (Test-Path -LiteralPath 'C:\Program Files\NSIS\makensis.exe') {
    $makensisPath = 'C:\Program Files\NSIS\makensis.exe'
} else {
    throw 'NSIS makensis.exe was not found.'
}

& $wailsPath build -clean
if ($LASTEXITCODE -ne 0) { throw "Wails build failed with exit code $LASTEXITCODE" }

& $makensisPath 'installer\AppleMusicDownloaderGUI.nsi'
if ($LASTEXITCODE -ne 0) { throw "NSIS build failed with exit code $LASTEXITCODE" }

Write-Host 'Installer: build\bin\AppleMusicDownloaderGUI-Setup.exe'
