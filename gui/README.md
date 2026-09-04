# Apple Music Downloader GUI

Windows desktop companion for the existing installation at `C:\Apple-Music-Downloader`.

## Security boundaries

- GUI settings contain only non-sensitive output and media preferences.
- Apple ID, password, 2FA codes and tokens are never written to settings or logs.
- Wrapper itself requires `username:password` in its Linux process arguments. The UI shows this risk before login.
- Closing the GUI stops only a Wrapper process started by that GUI session.
- Uninstalling the GUI never removes the downloader project, Wrapper, WSL, configuration or downloaded media.

## Pinned build dependencies

- Go 1.23.1 or newer
- Wails v2.14.0
- Node.js and npm
- TypeScript 5.9.2 and Vite 7.1.7 (pinned in `package-lock.json`)
- Microsoft WebView2 Runtime
- NSIS 3.12 for the machine-wide installer

Before installing build tools, verify their official release source and checksums where available.

## Build

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
cd frontend
npm ci --ignore-scripts --no-audit --no-fund
cd ..
.\build-installer.ps1
```

The installer is written to `build\bin\AppleMusicDownloaderGUI-Setup.exe`.
