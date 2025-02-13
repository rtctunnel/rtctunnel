FROM golang:1.19-alpine as builder
RUN apk --no-cache add git

ENV GO111MODULE=on
RUN mkdir -p /go/src/github.com/rtctunnel/rtctunnel
WORKDIR /go/src/github.com/rtctunnel/rtctunnel

COPY go.mod .
COPY go.sum . 
RUN go mod download

COPY channels channels
COPY cmd cmd 
COPY crypt crypt 
COPY peer peer 
COPY signal signal 

RUN go build -v -o /bin/rtctunnel ./main.go