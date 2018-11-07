#!/bin/bash
set -euxo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"

(cd "$DIR/.." && docker build -f dist/build.dockerfile -t gcr.io/doxsey-1/rtctunnel:latest .)
mkdir -p "$DIR/../bin/linux-amd64"
docker run -v "$DIR/..":/app -it gcr.io/doxsey-1/rtctunnel:latest /bin/sh -c 'cp -f /bin/rtctunnel /app/bin/linux-amd64/ && tar -czf /app/bin/rtctunnel_linux_amd64.tar.gz -C /app/bin/linux-amd64 rtctunnel'