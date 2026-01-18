# Go-Instagram-CLI 

<img width="1029" height="487" alt="image" src="https://github.com/user-attachments/assets/3dbe41a3-ac6c-41d1-b1c0-68314b2431ec" />

A high-performance Go CLI for posting Instagram Stories with automatic video segmenting and real-time progress tracking. 
Use instagram fully inside CLI

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
   stories                   View your active stories and their stats
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
 
## Screenshots
See your stories
<img width="649" height="692" alt="image" src="https://github.com/user-attachments/assets/c53cd447-135f-4795-b78f-84f2b3a83da7" />
Browse DMs
<img width="842" height="1038" alt="image" src="https://github.com/user-attachments/assets/3c8255b9-0a9a-41da-ab38-d87ee1be09e2" />
Message People
<img width="763" height="854" alt="image" src="https://github.com/user-attachments/assets/8f3a11c1-d1ea-4b3c-a852-55fc3589d7dd" />

have fun

## Prerequisites
- [FFmpeg](https://ffmpeg.org/) must be installed in your PATH.

## Installation
```bash
go build -o igcli ./cmd/igcli
