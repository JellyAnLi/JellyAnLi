# --- Stage 1: Build Frontend ---
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# --- Stage 2: Build Backend ---
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go/src/app
COPY go.mod ./
RUN go mod download
COPY . .
# Копируем собранный фронтенд из предыдущего шага
COPY --from=frontend-builder /app/dist ./frontend/dist
# Компилируем оптимизированный бинарник без отладочной информации под целевую платформу
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o jelly-an-li .

# --- Stage 3: Final Image ---
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata su-exec

# Путь для конфигов и кэша, дефолтные PUID/PGID
ENV CONFIG_DIR=/config
ENV PORT=37773
ENV PUID=1000
ENV PGID=1000
RUN mkdir /config

COPY --from=backend-builder /go/src/app/jelly-an-li /usr/local/bin/jelly-an-li
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

VOLUME /config

# Порт веб-интерфейса по умолчанию
EXPOSE 37773

# Запуск сервера из директории с конфигами
WORKDIR /config
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["jelly-an-li", "serve"]
