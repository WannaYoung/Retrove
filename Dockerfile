FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN rm -rf ./internal/ui/dist
COPY --from=web /web/dist ./internal/ui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/retrove ./cmd/retrove

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata su-exec
COPY --from=build /out/retrove /usr/local/bin/retrove
COPY docker/entrypoint.sh /entrypoint.sh
COPY app/config/platforms.yaml /defaults/platforms.yaml
COPY app/config/config.example.yaml /defaults/config.example.yaml
RUN chmod +x /entrypoint.sh
WORKDIR /app
EXPOSE 5180
ENTRYPOINT ["/entrypoint.sh"]
