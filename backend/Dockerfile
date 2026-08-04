# syntax=docker/dockerfile:1

# ─────────────────────────────── сборка ────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Зависимости кешируются отдельным слоем: правка кода не приводит к
# повторному скачиванию модулей.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev

# CGO выключен — бинарник статический и работает в минимальном образе.
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build \
        -trimpath -ldflags "-s -w" \
        -o /out/seeder ./cmd/seed

# ───────────────────────────── исполнение ──────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 10001 app

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/seeder /app/seeder

# Контент лежит рядом с бинарником: загрузчику не нужен смонтированный том.
COPY seed /app/seed

USER app

EXPOSE 8080

# Проба готовности учитывает соединение с БД, а не только живость процесса.
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 \
    CMD wget -qO- http://localhost:8080/readyz >/dev/null || exit 1

CMD ["/app/api"]
