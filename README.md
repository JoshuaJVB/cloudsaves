# CloudSaves

A self-hosted cloud save system for your game library. A lightweight Docker server stores your saves, and a cross-platform desktop client (Windows/Linux) lets you push and pull saves from any machine on your network.

## Features

- Register any game by pointing to its save file or folder
- Push your local save to the server or pull the latest down
- Timestamp comparison shows which side is newer before you act
- Keeps the last 5 saves per game automatically
- Shared API key authentication
- Single-binary client — no runtime or installer needed

---

## Server Setup

The server runs in Docker and stores all saves in a local `data/` directory.

**Prerequisites:** Docker and Docker Compose

```bash
git clone https://github.com/JoshuaJVB/cloudsaves.git
cd cloudsaves/server
```

Open `docker-compose.yml` and set a strong API key:

```yaml
environment:
  - API_KEY=your-secret-key-here
```

Start the server:

```bash
docker compose up -d
```

The server listens on port `8080`. Saves are persisted in `server/data/` on the host. To change the port, edit the `ports` line in `docker-compose.yml`.

---

## Client Setup

**Prerequisites:** [Go 1.21+](https://go.dev/dl/)

Fyne (the UI library) requires a C compiler:
- **Windows:** Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or [MSYS2](https://www.msys2.org/)
- **Linux:** `sudo apt install gcc libgl1-mesa-dev xorg-dev` (Debian/Ubuntu) or equivalent

```bash
cd cloudsaves/client
go mod tidy
```

**Build for Windows:**
```bash
go build -o cloudsave.exe .
```

**Build for Linux:**
```bash
go build -o cloudsave .
```

> **Cross-compilation note:** Fyne uses CGO, so `GOOS=linux go build` from Windows won't work out of the box. Build natively on each platform, or use [`fyne-cross`](https://github.com/fyne-io/fyne-cross) for cross-compilation.

---

## First Run

1. Launch the client (`cloudsave.exe` on Windows, `./cloudsave` on Linux).
2. Click the **gear icon** (Settings) and fill in:
   - **Server URL** — e.g. `http://192.168.1.100:8080`
   - **API Key** — must match the key set in `docker-compose.yml`
   - **Machine Name** — a label for this PC (pre-filled with your hostname)
3. Click **Add Game**, enter the game name, and browse to its save folder or file.
4. Use **Push to Server** or **Pull from Server** as needed.

On subsequent runs, just select the game from the dropdown — the path is remembered.

---

## How Push / Pull Works

When you **push**, the client zips your save folder (or file) and uploads it to the server, tagged with your machine name and the current timestamp.

When you **pull**, the server sends the most recent zip, which is extracted back into the original location, overwriting what's there. A confirmation dialog appears before any overwrite.

The status line tells you which side is newer before you act:

| Status | Meaning |
|---|---|
| Local is newer than server | You have unsaved progress — push it |
| Server is newer than local | Another machine pushed — pull to sync |
| In sync | Timestamps match |
| No saves on server yet | First time for this game — push to start |

---

## Project Structure

```
cloudsaves/
├── server/
│   ├── main.py            # FastAPI app — all routes
│   ├── database.py        # SQLAlchemy models (Game, Save)
│   ├── requirements.txt
│   ├── Dockerfile
│   └── docker-compose.yml
└── client/
    ├── main.go
    ├── go.mod
    ├── api/
    │   └── client.go      # HTTP client for the server API
    ├── archive/
    │   └── archive.go     # Zip / unzip with zip-slip protection
    ├── config/
    │   └── config.go      # Config file (JSON, platform-appropriate path)
    └── ui/
        └── ui.go          # Fyne GUI
```

**Config file location:**
- Windows: `%APPDATA%\cloudsave\config.json`
- Linux: `~/.config/cloudsave/config.json`

---

## Server API

All routes require the header `X-API-Key: <your-key>`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/games` | List all registered games |
| `POST` | `/games` | Register a game `{id, name}` — idempotent |
| `GET` | `/games/{id}/saves` | List saves (newest first) |
| `POST` | `/games/{id}/saves` | Upload a save (multipart: `file`, `machine_name`) |
| `GET` | `/games/{id}/saves/latest` | Download the most recent save |
| `GET` | `/games/{id}/saves/{save_id}` | Download a specific save |
