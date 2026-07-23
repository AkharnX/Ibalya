SHELL := /bin/bash
VENV := /home/akharn/app/.venv

-include .env
export

.PHONY: db build run-backend run-llm run stop logs demo

db: ## démarre PostgreSQL (Docker)
	docker-compose up -d db

build: ## compile le backend Go
	cd backend && go build -o bin/agentops ./cmd/server

run-llm: ## lance le service LLM (Python)
	cd llm-service && $(VENV)/bin/uvicorn app.main:app --host 127.0.0.1 --port 8092

run-backend: build ## lance le backend (port 9999)
	./backend/bin/agentops

demo: build ## chaîne complète sur données de démonstration (sans Gmail ni clé LLM)
	CHANNEL=fixture FIXTURE_PATH=$(PWD)/fixtures/messages.json ./backend/bin/agentops
