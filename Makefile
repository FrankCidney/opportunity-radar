# TODO: Change the migrate commands to use code in program (i.e., write code for migrations in a ./cmd/migrations/main.go)

# DB Config
include .env
export

SCHEDULER_ENABLED ?= true
SCHEDULER_INTERVAL ?= 24h
SCHEDULER_RUN_ON_START ?= true
SCHEDULER_RUN_TIMEOUT ?= 30m
DIGEST_ENABLED ?= false
DIGEST_TO_EMAIL ?=
DIGEST_TOP_N ?= 10
DIGEST_LOOKBACK ?= 24h

# Targets
.PHONY: migrate-up migrate-down migrate-force migrate-create run run-once run-scheduler-smoke run-test

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
	env DATABASE_URL="$(DATABASE_URL)" ENV="$(ENV)" PORT="$(PORT)" \
		SCHEDULER_ENABLED="$(SCHEDULER_ENABLED)" \
		SCHEDULER_INTERVAL="$(SCHEDULER_INTERVAL)" \
		SCHEDULER_RUN_ON_START="$(SCHEDULER_RUN_ON_START)" \
		SCHEDULER_RUN_TIMEOUT="$(SCHEDULER_RUN_TIMEOUT)" \
		DIGEST_ENABLED="$(DIGEST_ENABLED)" \
		DIGEST_TO_EMAIL="$(DIGEST_TO_EMAIL)" \
		DIGEST_TOP_N="$(DIGEST_TOP_N)" \
		DIGEST_LOOKBACK="$(DIGEST_LOOKBACK)" \
		go run ./cmd/app

run-once:
	env DATABASE_URL="$(DATABASE_URL)" ENV="$(ENV)" PORT="$(PORT)" \
		SCHEDULER_ENABLED="false" \
		DIGEST_ENABLED="$(DIGEST_ENABLED)" \
		DIGEST_TO_EMAIL="$(DIGEST_TO_EMAIL)" \
		DIGEST_TOP_N="$(DIGEST_TOP_N)" \
		DIGEST_LOOKBACK="$(DIGEST_LOOKBACK)" \
		go run ./cmd/app

run-scheduler-smoke:
	env DATABASE_URL="$(DATABASE_URL)" ENV="$(ENV)" PORT="$(PORT)" \
		SCHEDULER_ENABLED="true" \
		SCHEDULER_INTERVAL="5s" \
		SCHEDULER_RUN_ON_START="true" \
		SCHEDULER_RUN_TIMEOUT="20s" \
		DIGEST_ENABLED="$(DIGEST_ENABLED)" \
		DIGEST_TO_EMAIL="$(DIGEST_TO_EMAIL)" \
		DIGEST_TOP_N="$(DIGEST_TOP_N)" \
		DIGEST_LOOKBACK="$(DIGEST_LOOKBACK)" \
		go run ./cmd/app

run-test:
	env DATABASE_URL="$(DATABASE_URL)" ENV="$(ENV)" PORT="$(PORT)" go run ./cmd/test
