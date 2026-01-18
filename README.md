# Go-Instagram-CLI 

<p align="center">
  <img width="514" alt="Hero Logo" src="https://github.com/user-attachments/assets/3dbe41a3-ac6c-41d1-b1c0-68314b2431ec" />
</p>


A high-performance Go CLI for posting Instagram Stories with automatic video segmenting and real-time progress tracking. 
Use instagram fully inside CLI, without unnecesary things like reels, posts etc




```
NAME:
   go-instagram-cli - Instagram CLI tool

USAGE:
   go-instagram-cli [global options] [command [command options]]

VERSION:
   0.0.1-prerelease

COMMANDS:
   login                     Login to your Instagram account
   logout                    Logout from your Instagram account
   status                    Check current login status
   stories                   View your active stories and their stats or post new ones
   messages, dm, inbox, dms  View and manage your Instagram direct messages
   help, h                   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

## Features
- **Auto-Segmenting**: Automatically splits long videos into 15s/60s clips using FFmpeg.
- **Browsing DMs**: Browse DMs in CLI
- **Pro UI**: Real-time multi-part progress bars with ETA and upload speed.
- **Concurrent Processing**: Parallel video encoding for faster preparation.

## Missing Features
- **Responding in DMs**: This feature will be added in next release
 ## 📸 Screenshots

### Story Management
*View your active stories and see engagement stats at a glance.*
<img width="649" alt="Stories Screenshot" src="https://github.com/user-attachments/assets/c53cd447-135f-4795-b78f-84f2b3a83da7" />

### Inbox Browsing
*Stay updated with your Direct Messages without opening a browser.*
<img width="842" alt="DMs Screenshot" src="https://github.com/user-attachments/assets/3c8255b9-0a9a-41da-ab38-d87ee1be09e2" />

### Messaging interface
*Direct communication with followers via a clean CLI interface.*
<img width="763" alt="Messaging Screenshot" src="https://github.com/user-attachments/assets/8f3a11c1-d1ea-4b3c-a852-55fc3589d7dd" />

---

## 🛠️ Installation & Setup

### Prerequisites
> [!IMPORTANT]
> This tool requires **FFmpeg** installed and added to your system `PATH`.
> [Download FFmpeg here](https://ffmpeg.org/download.html).

### Build from Source
```bash
# Clone the repository
git clone git@github.com:PiotrWarzachowski/go-instagram-cli.git
cd go-instagram-cli

# Build the binary
go build -o igcli ./cmd/igcli/main.go

# Start using it
./igcli login
