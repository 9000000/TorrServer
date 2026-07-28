FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    ca-certificates \
    ffmpeg \
    gstreamer1.0-tools \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-base-apps \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav \
    && rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH}" \
    TS_PORT=7860 \
    TS_CONF_PATH="/data" \
    TS_TORR_DIR="/tmp/ts/torrents" \
    GODEBUG=madvdontneed=1

WORKDIR /app

# Tải TorrServer-LT linux-amd64 bản mới nhất từ Github Releases
RUN wget -qO /usr/bin/torrserver "https://github.com/9000000/TorrServer-LT/releases/latest/download/TorrServer-LT-linux-amd64-gst" \
    && chmod +x /usr/bin/torrserver

RUN mkdir -p /tmp/ts/torrents && \
    chmod -R 777 /tmp/ts

EXPOSE 7860

CMD ["/usr/bin/torrserver", "--port", "7860", "--path", "/data", "--torrentsdir", "/tmp/ts/torrents"]
