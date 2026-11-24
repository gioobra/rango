# Variáveis RANGO_* vêm do .env, se existir.
-include .env
export

.PHONY: run build test lint css

run: css
	go run ./cmd/rango serve

build: css
	go build -o bin/rango ./cmd/rango

test:
	go test ./...

lint:
	gofmt -l . | (! grep .) || (echo "rode gofmt -w ." && false)
	golangci-lint run

css:
	tailwindcss -i internal/web/tailwind.css -o internal/web/static/app.css --minify
