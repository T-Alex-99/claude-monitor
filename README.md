# Claude Monitor

Lightweight web dashboard to monitor Claude CLI processes.

## Features

- **Process List** - View all running Claude CLI instances with CPU%, RAM, and working directory
- **Smart Naming** - Processes named after their working folder (e.g., "my-project", "my-project (2nd)")
- **Kill Button** - Terminate runaway processes with one click
- **Temperature** - Real-time CPU temperature display
- **History Graphs** - 30-minute CPU and temperature charts
- **Browser Alerts** - Notifications when thresholds are exceeded
- **Configurable** - Adjustable CPU and temperature thresholds

## Requirements

- Linux (uses `/proc` filesystem)
- Go 1.21+
- `lm-sensors` for temperature readings (optional)

## Installation

```bash
git clone https://github.com/T-Alex-99/claude-monitor.git
cd claude-monitor
go build -o claude-monitor .
```

## Usage

```bash
./claude-monitor              # Start on default port 8080
./claude-monitor -port 3000   # Start on custom port
```

Then open http://localhost:8080

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Web dashboard |
| GET | `/api/processes` | List Claude processes |
| GET | `/api/temperature` | Temperature readings |
| GET | `/api/history` | Historical data (30 min) |
| POST | `/api/kill/{pid}` | Kill process (SIGTERM) |
| GET | `/api/settings` | Get alert settings |
| POST | `/api/settings` | Update alert settings |

## Configuration

Settings are stored in `~/.config/claude-monitor/settings.json`:

```json
{
  "cpuThreshold": 90,
  "tempThreshold": 85,
  "alertsEnabled": true
}
```

## License

MIT
