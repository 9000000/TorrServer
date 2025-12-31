<p align="center" style="text-align: center">
  <img src="https://github.com/9000000/TorrServer/assets/144587546/53f7175a-cda4-4a06-86b6-2ac07582dcf1" width="33%"><br/>
</p>

<p align="center">
  Simple and powerful tool for streaming torrents.
  <br/>
  <br/>
  <a href="https://github.com/9000000/TorrServer/blob/master/LICENSE">
    <img alt="GitHub" src="https://img.shields.io/github/license/9000000/TorrServer"/>
  </a>
  <a href="https://goreportcard.com/report/github.com/9000000/TorrServer">
    <img src="https://goreportcard.com/badge/github.com/9000000/TorrServer" />
  </a>
  <a href="https://pkg.go.dev/github.com/9000000/TorrServer">
    <img src="https://pkg.go.dev/badge/github.com/9000000/TorrServer.svg" alt="Go Reference"/>
  </a>
  <a href="https://github.com/9000000/TorrServer/issues">
    <img src="https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat" alt="CodeFactor" />
  </a>
  <a href="https://github.com/9000000/TorrServer/actions/workflows/docker_image.yml" rel="nofollow">
    <img src="https://img.shields.io/github/actions/workflow/status/9000000/TorrServer/docker_image.yml?logo=Github" alt="Build" />
  </a>
  <a href="https://github.com/9000000/TorrServer/releases" rel="nofollow">
    <img alt="GitHub release (latest SemVer)" src="https://img.shields.io/github/v/release/9000000/TorrServer?label=version"/>
  </a>
  <a href="https://github.com/9000000/TorrServer/tags" rel="nofollow">
    <img alt="GitHub tag (latest SemVer pre-release)" src="https://img.shields.io/github/v/tag/9000000/TorrServer?include_prereleases&label=pre-release"/>
  </a>
</p>

## Introduction

TorrServer is a program that allows users to view torrents online without the need for preliminary file downloading.
The core functionality of TorrServer includes caching torrents and subsequent data transfer via the HTTP protocol,
allowing the cache size to be adjusted according to the system parameters and the user's internet connection speed.

## AI Documentation

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/9000000/TorrServer)

## Features

- Caching
- Streaming
- Local and Remote Server
- Viewing torrents on various devices
- Integration with other apps through API
- Cross-browser modern web interface
- Optional DLNA server

## Getting Started

### Installation

Download the application for the required platform in the [releases](https://github.com/9000000/TorrServer/releases) page. After installation, open the link <http://127.0.0.1:8090> in the browser.

#### Linux

Run in console

```bash
curl -s https://raw.githubusercontent.com/9000000/TorrServer/master/installTorrServerLinux.sh | sudo bash
```

The script supports interactive and non-interactive installation, configuration, updates, and removal. When running the script interactively, you can:

- **Install/Update**: Choose to install or update TorrServer
- **Reconfigure**: If TorrServer is already installed, you'll be prompted to reconfigure settings (port, auth, read-only mode, logging, BBR)
- **Uninstall**: Type `Delete` (or `Удалить` in Russian) to uninstall TorrServer

**Download first and set execute permissions:**

```bash
curl -s https://raw.githubusercontent.com/9000000/TorrServer/master/installTorrServerLinux.sh -o installTorrServerLinux.sh && chmod 755 installTorrServerLinux.sh
```

**Command-line examples:**

- Install a specific version:

  ```bash
  sudo bash ./installTorrServerLinux.sh --install 135 --silent
  ```

- Update to latest version:

  ```bash
  sudo bash ./installTorrServerLinux.sh --update --silent
  ```

- Reconfigure settings interactively:

  ```bash
  sudo bash ./installTorrServerLinux.sh --reconfigure
  ```

- Check for updates:

  ```bash
  sudo bash ./installTorrServerLinux.sh --check
  ```

- Downgrade to a specific version:

  ```bash
  sudo bash ./installTorrServerLinux.sh --down 135
  ```

- Remove/uninstall:

  ```bash
  sudo bash ./installTorrServerLinux.sh --remove --silent
  ```

- Change the systemd service user:

  ```bash
  sudo bash ./installTorrServerLinux.sh --change-user root --silent
  ```

**All available commands:**

- `--install [VERSION]` - Install latest or specific version
- `--update` - Update to latest version
- `--reconfigure` - Reconfigure TorrServer settings (port, auth, read-only mode, logging, BBR)
- `--check` - Check for updates (version info only)
- `--down VERSION` - Downgrade to specific version
- `--remove` - Uninstall TorrServer
- `--change-user USER` - Change service user (root|torrserver)
- `--root` - Run service as root user
- `--silent` - Non-interactive mode with defaults
- `--help` - Show help message

### Running in Docker & Docker Compose

Run in console

```bash
docker run --rm -d --name torrserver -p 8090:8090 ghcr.io/9000000/torrserver:latest
```

For running in persistence mode, just mount volume to container by adding `-v ~/ts:/opt/ts`, where `~/ts` folder path is just example, but you could use it anyway... Result example command:

```bash
docker run --rm -d --name torrserver -v ~/ts:/opt/ts -p 8090:8090 ghcr.io/9000000/torrserver:latest
```

#### Docker Compose

```yml
# docker-compose.yml

version: '3.3'
services:
    torrserver:
        image: ghcr.io/9000000/torrserver
        container_name: torrserver
        network_mode: host
        environment:
            - TS_PORT=5665
            - TS_DONTKILL=1
            - TS_HTTPAUTH=0
            - TS_CONF_PATH=/opt/ts/config
            - TS_TORR_DIR=/opt/ts/torrents
        volumes:
            - './CACHE:/opt/ts/torrents'
            - './CONFIG:/opt/ts/config'
        ports:
            - '5665:5665'
        restart: unless-stopped
```

## Development

### Build

#### Server

- Install [Golang](https://golang.org/doc/install) 1.20+
- Go to the TorrServer source directory
- Run build script under linux `build-all.sh`

#### Web

- Install **npm** and **yarn**
- Go to the web directory
- Run `NODE_OPTIONS=--openssl-legacy-provider yarn build`

More info at <https://github.com/9000000/TorrServer/tree/master/web#readme>

## API

API documentation is hosted as Swagger format available at path `/swagger/index.html`.

## Authentication

The users data file should be located near to the settings. Basic auth.

`accs.db` in JSON format:

```json
{
    "User1": "Pass1",
    "User2": "Pass2"
}
```

Note: You should enable authentication with -a (--httpauth) TorrServer startup option.
