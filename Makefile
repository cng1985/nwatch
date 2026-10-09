.PHONY: web build test run

web:
	cd web && npm install && npm run build

build: web
	go build -o nmonitor ./cmd/server

test:
	go test ./...

run: build
	./nmonitor
