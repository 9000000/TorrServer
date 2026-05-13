
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    ca-certificates \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app


RUN architecture=$(uname -m) && \
    case $architecture in \
        x86_64) arch="amd64" ;; \
        aarch64) arch="arm64" ;; \
        i386|i686) arch="386" ;; \
        armv7|armv7l) arch="arm7" ;; \
        *) echo "Unsupported arch: $architecture"; exit 1 ;; \
    esac && \
    echo "Tải TorrServer bản: $arch" && \
    wget -qO /usr/bin/torrserver "https://github.com/9000000/TorrServer/releases/latest/download/TorrServer-linux-$arch" && \
    chmod +x /usr/bin/torrserver

ENV TS_PORT=7860
ENV TS_CONF_PATH="/data"
ENV TS_TORR_DIR="/tmp/ts/torrents"
ENV TS_LOG_PATH="/tmp/ts/torr.log"
ENV GODEBUG=madvdontneed=1

RUN mkdir -p /tmp/ts/torrents && \
    chmod -R 777 /tmp/ts

EXPOSE 7860

CMD ["sh", "-c", "[ ! -f /data/tracker.txt ] && touch /data/tracker.txt; exec /usr/bin/torrserver --port 7860 --path /data --torrentsdir /tmp/ts/torrents --logpath /tmp/ts/torr.log"]


