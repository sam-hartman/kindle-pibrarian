# syntax=docker/dockerfile:1.7
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/annas-mcp ./cmd/annas-mcp

FROM alpine:3.20
# Install tailscale via apk directly (the install.sh script runs `rc-update`
# which requires OpenRC; we don't have/need it in this image).
RUN apk add --no-cache ca-certificates iptables ip6tables tailscale
COPY --from=build /out/annas-mcp /usr/local/bin/annas-mcp
COPY <<'EOF' /usr/local/bin/start.sh
#!/bin/sh
set -e
mkdir -p /var/run/tailscale /var/cache/tailscale /var/lib/tailscale
tailscaled \
  --tun=userspace-networking \
  --socks5-server=localhost:1055 \
  --outbound-http-proxy-listen=localhost:1055 \
  --state=/var/lib/tailscale/tailscaled.state &
sleep 2
tailscale up \
  --authkey="${TAILSCALE_AUTH_KEY}" \
  --hostname=fly-kindle-pibrarian \
  --exit-node="${TAILSCALE_EXIT_NODE}" \
  --accept-routes
export HTTPS_PROXY=http://localhost:1055
export HTTP_PROXY=http://localhost:1055
export NO_PROXY=localhost,127.0.0.1,fly.dev,fly.io,.internal
exec /usr/local/bin/annas-mcp http --port 8080
EOF
RUN chmod +x /usr/local/bin/start.sh
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/start.sh"]
