FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/retrove ./cmd/retrove

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/retrove /usr/local/bin/retrove
EXPOSE 5080
ENTRYPOINT ["retrove"]
