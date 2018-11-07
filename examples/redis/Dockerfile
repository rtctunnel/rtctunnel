FROM alpine:latest 

RUN apk add bash curl redis python py-pip jq
RUN pip install yq

RUN curl -L https://github.com/rtctunnel/rtctunnel/releases/download/v0.1.1/rtctunnel_linux_amd64.tar.gz \
    | tar -C /bin -xz
RUN curl -L -o /bin/wait-for-it https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh \
    && chmod +x /bin/wait-for-it

# Create the initial profiles so we can share the public keys easily
RUN rtctunnel init --config-file=server.yaml
RUN rtctunnel init --config-file=client.yaml

COPY client.sh /bin/client.sh 
COPY server.sh /bin/server.sh

