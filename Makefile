test:
	go test ./...

run:
	go run ./cmd/deplens-platform

migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up
