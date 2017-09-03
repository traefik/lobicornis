FROM golang:1-alpine as builder

RUN apk --update upgrade \
&& apk --no-cache --no-progress add git make \
&& rm -rf /var/cache/apk/*

WORKDIR /go/src/github.com/containous/lobicornis
COPY . .

RUN go get -u github.com/golang/dep/cmd/dep
RUN make dependencies
RUN make build

FROM alpine:3.6
RUN apk --update upgrade \
&& apk --no-cache --no-progress add git \
&& rm -rf /var/cache/apk/*
COPY --from=builder /go/src/github.com/containous/lobicornis/lobicornis .
CMD ["./lobicornis", "-h"]