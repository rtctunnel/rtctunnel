#!/bin/bash
set -euo pipefail

# use the client config
cp client.yaml $HOME/.config/rtctunnel/rtctunnel.yaml

# add the route
CLIENT_KEY="$(cat client.yaml | yq -r .keypair.public)"
SERVER_KEY="$(cat server.yaml | yq -r .keypair.public)"
rtctunnel add-route \
    --local-peer=$CLIENT_KEY --local-port=6379 \
    --remote-peer=$SERVER_KEY --remote-port=6379 

# show our info
rtctunnel info

# start rtctunnel
rtctunnel run &
wait-for-it localhost:6379

# run the redis commands
echo "running redis commands:"
redis-cli set key value
redis-cli get key