all: dist/client.wasm dist/server.wasm dist/wasm_exec.js

dist/wasm_exec.js:
	cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" ./dist/wasm_exec.js

dist/client.wasm:
	env GOOS=js GOARCH=wasm go build -o ./dist/client.wasm ./client

dist/server.wasm:
	env GOOS=js GOARCH=wasm go build -o ./dist/server.wasm ./server

.PHONY: dist/client.wasm dist/server.wasm
