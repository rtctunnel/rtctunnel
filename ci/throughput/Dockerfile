ARG VERSION

FROM quay.io/rtctunnel/rtctunnel:$VERSION

FROM golang:1.13.4-alpine3.10

RUN apk add --no-cache --update git

RUN git clone https://github.com/microsoft/ethr.git
RUN cd ethr && go install ./...

FROM alpine:3.10

RUN apk add --no-cache --update \
    bash \
    curl \
    iperf3 \
    jq \
    netcat-openbsd \
    python \
    py-pip

RUN pip install yq

RUN curl -L -o /bin/wait-for https://raw.githubusercontent.com/eficode/wait-for/master/wait-for \
    && chmod +x /bin/wait-for

COPY --from=0 /usr/local/bin/rtctunnel /usr/local/bin/rtctunnel
COPY --from=1 /go/bin/ethr /usr/local/bin/ethr
