# ============================================
# Stage 1: Build frontend
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --prefer-offline 2>/dev/null || npm install
COPY frontend/ ./
RUN npm run build

# ============================================
# Stage 2: Build Go backend
# ============================================
FROM golang:1.22-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app/backend
COPY backend/ ./
RUN go mod tidy && go mod download

# Build with CGO for SQLite, static link, strip debug info
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -tags 'sqlite_omit_load_extension netgo' \
    -o /app/server .

# ============================================
# Stage 3: Final minimal image
# ============================================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -h /app appuser

WORKDIR /app

COPY --from=backend-builder /app/server ./server
COPY --from=frontend-builder /app/frontend/dist ./static

RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENV GIN_MODE=release \
    PORT=8080 \
    DB_PATH=/app/data/oauth.db \
    STATIC_DIR=/app/static

VOLUME ["/app/data"]

ENTRYPOINT ["./server"]
