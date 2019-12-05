FROM golang:1.13.4-alpine3.10 as builder
RUN apk --no-cache add git gcc musl-dev

ENV GO111MODULE=on
ENV CGO_ENABLED=0

RUN mkdir -p /go/src/github.com/rtctunnel/rtctunnel
WORKDIR /go/src/github.com/rtctunnel/rtctunnel

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -v -ldflags '-extldflags "-static"' \
    -o /usr/local/bin/rtctunnel ./cmd/rtctunnel

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root
COPY --from=0 /usr/local/bin/rtctunnel /usr/local/bin/rtctunnel
CMD ["/usr/local/bin/rtctunnel"]
