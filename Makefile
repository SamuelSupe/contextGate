.PHONY: ui build test docker dist
ui:
	cd web && npm ci --no-audit --no-fund && npm run build
build: ui
	CGO_ENABLED=1 go build -trimpath -o bin/contextgate ./cmd/mcpdbhub
	ln -sf contextgate bin/mcpdbhub
test:
	go test ./...
docker:
	docker build -t contextgate:local .
dist:
	python3 scripts/dist.py --version "$(VERSION)"
