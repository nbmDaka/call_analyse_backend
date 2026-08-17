.PHONY: up down logs migrate-up migrate-down test build vet fmt verify

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate-up:
	docker compose run --rm migrate

migrate-down:
	@echo "Down migrations are not exposed by the current migration binary."

test:
	go test ./...

build:
	go build ./...

vet:
	go vet ./...

fmt:
	powershell -NoProfile -Command "Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $$_.FullName }"

verify: test vet build
	docker compose config
	git diff --check
