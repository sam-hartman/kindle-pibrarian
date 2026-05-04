# syntax=docker/dockerfile:1.7
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/annas-mcp ./cmd/annas-mcp

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/annas-mcp /annas-mcp
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/annas-mcp"]
CMD ["http", "--port", "8080"]
