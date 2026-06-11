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

The server listens on port `45231`. Saves are persisted in `server/data/` on the host. To change the port, edit the `ports` line in `docker-compose.yml`.

---

## Client Setup

> **Prebuilt downloads:** Every [release](https://github.com/JoshuaJVB/cloudsaves/releases) ships ready-to-run binaries — `CloudSave-Setup.exe` (Windows installer) and `cloudsave-linux-amd64` (Linux binary). On Linux: `chmod +x cloudsave-linux-amd64 && ./cloudsave-linux-amd64`. The Linux binary needs the usual desktop GL/X11 runtime libraries (`libgl1`, `libx11-6` and friends — present on any standard desktop). Building from source is only needed if you want to modify the client.

**Prerequisites (building from source):** [Go 1.21+](https://go.dev/dl/)

Fyne (the UI library) requires a C compiler:
- **Windows:** Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or [MSYS2](https://www.msys2.org/)
- **Linux:** `sudo apt install gcc libgl1-mesa-dev xorg-dev` (Debian/Ubuntu) or equivalent

```bash
cd cloudsaves/client
go mod tidy
```

**Build for Windows:**
```bash
# Install the Fyne packaging tool (first time only)
go install fyne.io/tools/cmd/fyne@latest

# Generate the app icon (first time only, must be run from the client/ directory)
go run gen_icon.go

# Package the application
fyne package -os windows -name "CloudSave"
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
   - **Server URL** — e.g. `http://192.168.1.100:45231`
   - **API Key** — must match the key set in `docker-compose.yml`
   - **Machine Name** — a label for this PC (pre-filled with your hostname)
3. Click **Add Game**, enter the game name, and browse to its save folder or file.
4. Use **Push to Server** or **Pull from Server** as needed.

On subsequent runs, just select the game from the dropdown — the path is remembered.

---

## How Push / Pull Works

When you **push**, the client zips your save folder (or file) and uploads it to the server, tagged with your machine name and the save's **modification time** (when you last played — not when you uploaded).

When you **pull**, the server sends the save with the newest modification time, which is extracted back into the original location, overwriting what's there. A confirmation dialog appears before any overwrite.

Newer/older is decided by comparing the save's modification time on each side — so the same save pushed from one machine reads as "in sync" on another, and a genuinely newer local save is never overwritten by an older one just because it was uploaded more recently. The Server card shows both **Saved** (content modification time, used for the comparison) and **Uploaded** (when it reached the server).

The status line tells you which side is newer before you act:

| Status | Meaning |
|---|---|
| Local is newer than server | You have unsaved progress — push it |
| Server is newer than local | Another machine pushed — pull to sync |
| In sync | Timestamps match |
| No saves on server yet | First time for this game — push to start |

### Save history

The server keeps the **last 5 saves** per game. The Server card has a dropdown listing them, each labelled with its **save time** and the **machine** that pushed it. Pulling fetches whichever save is selected — so you can roll back to an older save, not just the latest. The machine name comes from each client's **Settings → Machine Name**.

---

## Updating

**Server** — on the host running Docker:

```bash
cd /mnt/user/appdata/cloudsaves
./update-server.sh
```

It pulls the latest code, rebuilds the image, and restarts the container — preserving your API key and saved data. To override the key: `API_KEY=yourkey ./update-server.sh`.

**Client** — on Windows:

```powershell
powershell -ExecutionPolicy Bypass -File update-client.ps1
```

It downloads the newest installer from the [Releases page](https://github.com/JoshuaJVB/cloudsaves/releases) and installs it silently (closing the app first if it's open).

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
| `POST` | `/games/{id}/saves` | Upload a save (multipart: `file`, `machine_name`, `saved_at`) |
| `GET` | `/games/{id}/saves/latest` | Download the most recent save |
| `GET` | `/games/{id}/saves/{save_id}` | Download a specific save |
