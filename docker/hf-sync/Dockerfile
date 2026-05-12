# Đổi sang Ubuntu cho độ tương thích file binary cao nhất
FROM ubuntu:24.04

# Cài đặt wget, ca-certificates, ffmpeg
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    ca-certificates \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Bước tự động nhận diện kiến trúc CPU và tải TorrServer
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

# Thiết lập biến môi trường
ENV TS_PORT=7860
ENV TS_CONF_PATH="/tmp/ts/config"
ENV TS_TORR_DIR="/tmp/ts/torrents"
ENV TS_LOG_PATH="/tmp/ts/torr.log"
ENV GODEBUG=madvdontneed=1

# Cấp quyền ghi toàn thư mục /tmp/ts cho User 1000 của Hugging Face
RUN mkdir -p /tmp/ts/config /tmp/ts/torrents && chmod -R 777 /tmp/ts

EXPOSE 7860

# Lệnh khởi chạy trực tiếp
CMD ["/usr/bin/torrserver", "--port", "7860", "--path", "/tmp/ts/config", "--torrentsdir", "/tmp/ts/torrents", "--logpath", "/tmp/ts/torr.log"]
