FROM golang:1-alpine as builder

RUN apk --update upgrade \
&& apk --no-cache --no-progress add git make \
&& rm -rf /var/cache/apk/*

WORKDIR /go/src/github.com/containous/lobicornis
COPY . .

RUN go mod download
RUN make build

FROM alpine:3.10
RUN apk --update upgrade \
    && apk --no-cache --no-progress add ca-certificates git \
    && rm -rf /var/cache/apk/*

COPY --from=builder /go/src/github.com/containous/lobicornis/lobicornis /usr/bin/lobicornis

ENTRYPOINT ["/usr/bin/lobicornis"]
