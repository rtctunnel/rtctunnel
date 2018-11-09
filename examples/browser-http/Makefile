all: dist/client.js dist/server.js

dist/client.js:
	gopherjs build -v -o dist/client.js ./client

dist/server.js:
	gopherjs build -v -o dist/server.js ./server

.PHONY: dist/client.js dist/server.js
