FROM golang:1.11-alpine as builder
RUN apk --no-cache add git gcc musl-dev libcrypto1.0 openssl-dev zlib-dev

ENV GO111MODULE=on
RUN mkdir -p /go/src/github.com/rtctunnel/rtctunnel
WORKDIR /go/src/github.com/rtctunnel/rtctunnel

COPY go.mod .
COPY go.sum . 
RUN go mod download

COPY cmd cmd 
COPY crypt crypt 
COPY peer peer 
COPY signal signal 

RUN go build -v -ldflags '-extldflags "-static -lz"' \
    -o /bin/rtctunnel ./cmd/rtctunnel