# CLI Install

## From release

```bash
# Linux / macOS
curl -fsSL https://github.com/eyangisgod-cell/ai-kbs/releases/latest/download/ai-kbs-$(uname -s | tr A-Z a-z)-$(uname -m) -o ai-kbs
chmod +x ai-kbs
sudo mv ai-kbs /usr/local/bin/
```

```powershell
# Windows (PowerShell)
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Invoke-WebRequest -Uri "https://github.com/eyangisgod-cell/ai-kbs/releases/latest/download/ai-kbs-windows-$arch.exe" -OutFile ai-kbs.exe
```

## From source

```bash
git clone https://github.com/eyangisgod-cell/ai-kbs
cd ai-kbs
make cli-build    # outputs to ./bin/
```

## Verify

```bash
ai-kbs --version
# ai-kbs version 1.0.0 (commit: ..., built: 2026-10-09)
```

## First-time setup

```bash
mkdir my-kb && cd my-kb
ai-kbs init
ai-kbs login --url https://kbs.example.com
ai-kbs bind dev:/api/
```

## Upgrade

```bash
ai-kbs upgrade --channel stable
```

The CLI checks for updates on every invocation and emits a non-blocking notice if a newer version is available.
