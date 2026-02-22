# DB Config
DB_URL=postgres://postgres@localhost:5432/opportunity_radar_dev?sslmode=disable

# Targets
.Phony migrate-up migrate-down migrate-create

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down

migrate-create:
	ifndef name
		@echo "Error: name is required. Usage: make migrate-create name=your_migration_name"
		@exit 1
	endif
		migrate create -ext sql -dir migrations "$(name)"