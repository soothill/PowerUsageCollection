.PHONY: init run

init:
@echo "Fetching Go module dependencies"
go mod tidy
go mod download

run:
go run ./...
