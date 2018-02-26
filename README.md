# RTCTunnel

The RTCTunnel is a suite of applications used to build network tunnels over Web RTC.

## Flow

Imagine we have two peers: Bob and Alice. Bob wants to connect to a redis server run by Alice over an RTCTunnel. From the user's perspective the process is as follows:

1. Alice starts the redis server listening on `127.0.0.1:7777`

1. Alice runs the RTCTunnel Local Client and creates a local endpoint for `redis-alice.rtctunnel.com` which points to `127.0.0.1:7777`.

1. Bob runs the RTCTunnel Local Client and creates a remote endpoint for `redis-alice.rtctunnel.com` on `127.0.0.1:7777`.

1. Bob opens up a terminal and runs `telnet localhost 7777`

## Operator

The operator coordinates sessions between peers.
