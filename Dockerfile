FROM golang:1-alpine as builder

RUN apk --no-cache --no-progress add git make \
&& rm -rf /var/cache/apk/*

WORKDIR /go/lobicornis

ENV GO111MODULE on

# Download go modules
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN make build

FROM alpine:3.20
RUN apk --no-cache --no-progress add ca-certificates git \
    && rm -rf /var/cache/apk/*

COPY --from=builder /go/lobicornis/lobicornis /usr/bin/lobicornis

ENTRYPOINT ["/usr/bin/lobicornis"]
