# DB Config
include .env
export

# Targets
.PHONY: migrate-up migrate-down migrate-create

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down

migrate-force:
ifndef version
	@echo "Error: version is required. Usage: make migrate-force version=XXXX"
	@exit 1
endif	
	migrate -path migrations -database "$(DB_URL)" force "$(version)"

migrate-create:
ifndef name
	@echo "Error: name is required. Usage: make migrate-create name=your_migration_name"
	@exit 1
endif
	migrate create -ext sql -dir migrations "$(name)"