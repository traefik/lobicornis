# syntax=docker/dockerfile:1.2
FROM golang:1-alpine as builder

RUN apk --no-cache --no-progress add git ca-certificates tzdata make \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

# syntax=docker/dockerfile:1.2
# Create a minimal container to run a Golang static binary
FROM alpine:3.16

RUN apk --no-cache --no-progress add ca-certificates git \
    && rm -rf /var/cache/apk/*

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY lobicornis /

ENTRYPOINT ["/lobicornis"]
EXPOSE 80
