FROM alpine
LABEL maintainer "9000000"
RUN apk add --no-cache wget ca-certificates
COPY start.sh /start.sh
RUN chmod +x /start.sh
EXPOSE 8080
ENTRYPOINT ["/bin/sh", "/start.sh"]
