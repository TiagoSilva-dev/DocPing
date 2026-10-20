# Uso exclusivamente local. Produção deve escolher S3 mantido com criptografia.
# Release oficial fixa inclui a correção de segurança de outubro de 2025.
FROM golang:1.26-alpine AS build
RUN apk add --no-cache git
ENV CGO_ENABLED=0
RUN go install github.com/minio/minio@RELEASE.2025-10-15T17-29-55Z

FROM alpine:3.23
RUN apk add --no-cache ca-certificates curl \
    && addgroup -S minio && adduser -S -G minio minio \
    && mkdir /data && chown minio:minio /data
COPY --from=build /go/bin/minio /usr/local/bin/minio
USER minio
EXPOSE 9000 9001
ENTRYPOINT ["minio"]
