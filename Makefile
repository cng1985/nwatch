.PHONY: web build test vet run install-service clean

web:
	cd web && npm install && npm run build

build: web
	go build -o nwatch ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

run: build
	APP_PROFILE=dev ./nwatch

install-service:
	./scripts/install-linux.sh

clean:
	rm -f nwatch
	rm -rf release
