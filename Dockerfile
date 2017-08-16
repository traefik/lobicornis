FROM golang:1-alpine

RUN apk --update upgrade \
&& apk --no-cache --no-progress add git make \
&& rm -rf /var/cache/apk/*

WORKDIR /go/src/github.com/containous/lobicornis
COPY . .

RUN go get -u github.com/golang/dep/cmd/dep
RUN make dependencies
RUN make build

CMD ["./lobicornis", "-h"]