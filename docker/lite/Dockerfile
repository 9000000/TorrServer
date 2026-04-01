### FRONT BUILD START ###
FROM --platform=$BUILDPLATFORM node:20-alpine AS front

WORKDIR /app

COPY ./web/package.json ./web/yarn.lock ./
RUN yarn install

COPY ./web .
RUN yarn run build
### FRONT BUILD END ###


### BUILD TORRSERVER MULTIARCH START ###
FROM --platform=$BUILDPLATFORM golang:1.26.0-alpine AS builder

COPY . /opt/src
COPY --from=front /app/build /opt/src/web/build

WORKDIR /opt/src

ARG TARGETARCH
ENV GOARCH=$TARGETARCH

RUN apk add --update g++ \
    && go run gen_web.go \
    && cd server \
    && go mod tidy \
    && go clean -i -r -cache \
    && go build -ldflags '-w -s' --o "torrserver" ./cmd
### BUILD TORRSERVER MULTIARCH END ###


### UPX COMPRESSING START ###
FROM ubuntu AS compressed

COPY --from=builder /opt/src/server/torrserver ./torrserver

RUN apt update && apt install -y upx-ucl && upx --best --lzma ./torrserver
### UPX COMPRESSING END ###


### BUILD MAIN IMAGE START ###
FROM scratch

COPY --from=compressed ./torrserver /TorrServer

ENTRYPOINT ["/TorrServer"]
### BUILD MAIN IMAGE end ###
