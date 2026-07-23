FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /rootguard-core ./cmd/rootguard

FROM docker:29-cli

RUN apk add --no-cache docker-cli-compose \
    && mkdir -p /var/lib/rootguard/unbound

COPY --from=builder /rootguard-core /usr/local/bin/rootguard-core

EXPOSE 8081
ENTRYPOINT ["rootguard-core"]
