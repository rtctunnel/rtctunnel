#!/bin/bash
set -euo pipefail

: "${VERSION?"VERSION is required"}"

docker build --tag=rtctunnel-ci-throughput:latest --build-arg="VERSION=$VERSION" .

mkdir -p /tmp/rtctunnel-ci-throughput
rm /tmp/rtctunnel-ci-throughput/* || true

docker run -v /tmp/rtctunnel-ci-throughput:/config rtctunnel-ci-throughput:latest rtctunnel init --config-file=/config/server.yaml
docker run -v /tmp/rtctunnel-ci-throughput:/config rtctunnel-ci-throughput:latest rtctunnel init --config-file=/config/client.yaml

docker run -t -v /tmp/rtctunnel-ci-throughput:/config rtctunnel-ci-throughput:latest bash -c '
set -euo pipefail

cd /config

CLIENT_KEY="$(cat client.yaml | yq -r .keypair.public)"
SERVER_KEY="$(cat server.yaml | yq -r .keypair.public)"
for port in 8888 9999 9999 9899 9799; do
  rtctunnel --config-file=/config/server.yaml \
    add-route \
      --local-peer=$CLIENT_KEY --local-port=$port \
      --remote-peer=$SERVER_KEY --remote-port=$port
done

echo "running server"
ethr -s &

echo "waiting for server to come up"
wait-for localhost:9999


rtctunnel --config-file=/config/server.yaml \
  run
' &
docker run -t -v /tmp/rtctunnel-ci-throughput:/config rtctunnel-ci-throughput:latest bash -c '
set -euo pipefail

cd /config

CLIENT_KEY="$(cat client.yaml | yq -r .keypair.public)"
SERVER_KEY="$(cat server.yaml | yq -r .keypair.public)"
for port in 8888 9999 9999 9899 9799; do
  rtctunnel --config-file=/config/client.yaml \
    add-route \
      --local-peer=$CLIENT_KEY --local-port=$port \
      --remote-peer=$SERVER_KEY --remote-port=$port
done

rtctunnel --config-file=/config/client.yaml \
  run &

echo "waiting for server to come up"
wait-for localhost:9999

echo "running test"
ethr -c localhost -n 8
'
kill %1
