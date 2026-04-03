# TODO: Change the migrate commands to use code in program (i.e., write code for migrations in a ./cmd/migrations/main.go)

# DB Config
include .env
export

# Targets
.PHONY: migrate-up migrate-down migrate-create run

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-force:
ifndef version
	@echo "Error: version is required. Usage: make migrate-force version=XXXX"
	@exit 1
endif	
	migrate -path migrations -database "$(DATABASE_URL)" force "$(version)"

migrate-create:
ifndef name
	@echo "Error: name is required. Usage: make migrate-create name=your_migration_name"
	@exit 1
endif
	migrate create -ext sql -dir migrations "$(name)"

run:
	env DATABASE_URL="$(DATABASE_URL)" ENV="$(ENV)" PORT="$(PORT)" go run ./cmd/app

run-test:
	env DATABASE_URL="$(DATABASE_URL)" ENV="$(ENV)" PORT="$(PORT)" go run ./cmd/test