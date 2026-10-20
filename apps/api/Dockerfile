FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /docping ./cmd/server
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -S docping && adduser -S -G docping docping && mkdir /data && chown docping:docping /data
WORKDIR /app
COPY --from=build /docping /app/docping
USER docping
ENV PORT=8080 DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/app/docping"]
