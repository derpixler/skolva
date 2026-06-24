# Multi-Stage Build: Go Builder + Alpine Runtime
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/bin/skolva ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates chromium font-noto-cjk tzdata

ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_HEADLESS=true

RUN addgroup -S app && adduser -S app -G app
USER app

WORKDIR /app
COPY --from=builder /app/bin/skolva .
COPY --from=builder /app/schema.sql .

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/health || exit 1

ENTRYPOINT ["./skolva"]
