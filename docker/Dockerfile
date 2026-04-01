FROM alpine
LABEL maintainer "9000000"
COPY start.sh /start.sh
RUN apk add --no-cache wget \
    && chmod +x /start.sh \
    && sed -i 's/\r$//' /start.sh

ENTRYPOINT ["/bin/sh", "/start.sh"]

