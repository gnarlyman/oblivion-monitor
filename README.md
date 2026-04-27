# oblivion-monitor

Per-process VRAM/RAM/CPU monitor for Oblivion.exe. Shows an always-on-top
window when Oblivion is running and writes per-session CSV logs.

## Build

```
go install github.com/akavel/rsrc@latest
./build.sh
```

Produces `oblivion-monitor.exe` (~5 MB, GUI subsystem, no console).

## Run

Double-click `oblivion-monitor.exe`. Lives in the system tray; window
appears whenever Oblivion.exe is running.

CSV logs go to `logs/YYYY-MM-DD-HHMMSS.csv`.

## Configuration

`config.json` (auto-created on first window drag) — see source for fields.

## Troubleshooting

If VRAM rows show `--` permanently, run `lodctr /R` from an Admin
PowerShell to repair Windows GPU performance counters.
