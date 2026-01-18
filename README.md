# Go-Instagram-CLI 🚀

A high-performance Go CLI for posting Instagram Stories with automatic video segmenting and real-time progress tracking.

## Features
- **Auto-Segmenting**: Automatically splits long videos into 15s/60s clips using FFmpeg.
- **Pro UI**: Real-time multi-part progress bars with ETA and upload speed.
- **Concurrent Processing**: Parallel video encoding for faster preparation.

## Prerequisites
- [FFmpeg](https://ffmpeg.org/) must be installed in your PATH.

## Installation
```bash
go build -o igcli ./cmd/igcli