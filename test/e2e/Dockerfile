FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -trimpath -ldflags "-s -w -X main.version=v0.0.0-e2e" \
      -o /tmp/heft ./cmd/heft \
    && mkdir /release \
    && tar -C /tmp -czf /release/heft_v0.0.0-e2e_linux_amd64.tar.gz heft \
    && cd /release \
    && sha256sum ./*.tar.gz > SHA256SUMS

FROM node:26-bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates coreutils curl git tar \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL --retry 3 \
      https://github.com/herdrdev/herdr/releases/download/v0.9.0/herdr-linux-x86_64 \
      -o /usr/local/bin/herdr \
    && echo "4fa1a01158dd8043da92d31b270780b0dcc10603038d9b61cac4d81ab63fb71f  /usr/local/bin/herdr" | sha256sum -c - \
    && chmod 755 /usr/local/bin/herdr

COPY --from=build /release /release
COPY install.sh /test/install.sh
COPY test/e2e/run.sh /test/run.sh
COPY skills/heft /test/repo/skills/heft

RUN /test/run.sh
