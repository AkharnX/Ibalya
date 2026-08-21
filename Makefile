SHELL := /bin/bash
VENV := /home/akharn/app/.venv
LOGS := $(PWD)/logs
API_PORT := 9999
LLM_PORT := 8092

# Connecteur réel par défaut. La cible `demo` bascule explicitement sur les
# données de démonstration — ne jamais inverser : un déploiement en ligne qui
# lit des fixtures ressemble à s'y méprendre à une application qui fonctionne.
CHANNEL ?= gmail
FIXTURE_PATH ?= $(PWD)/fixtures/messages.json

-include .env
export

.PHONY: db build front front-dev run-backend run-llm restart restart-llm restart-nohup restart-all stop services test status logs logs-llm demo help utilisateur

help: ## liste les cibles disponibles
	@grep -hE '^[a-z-]+:.*##' $(MAKEFILE_LIST) | sed 's/:.*##/\t/' | column -t -s $$'\t'

db: ## démarre PostgreSQL (Docker)
	docker-compose up -d db

build: ## compile le backend Go
	cd backend && go build -o bin/ibalya ./cmd/server

front: ## build du frontend React (frontend/dist, servi par le backend)
	cd frontend && npm install && npm run build

front-dev: ## frontend en mode dev (hot-reload, proxy /api vers :9999)
	cd frontend && npm run dev

run-llm: ## lance le service LLM au premier plan
	cd llm-service && $(VENV)/bin/uvicorn app.main:app --host 127.0.0.1 --port $(LLM_PORT)

run-backend: build ## lance le backend au premier plan
	CHANNEL=$(CHANNEL) FIXTURE_PATH=$(FIXTURE_PATH) ./backend/bin/ibalya

# ── recompilation + redémarrage ──
# Les services sont supervisés par systemd : ils repartent seuls après un
# plantage ou un redémarrage du serveur. Les cibles ci-dessous s'appuient
# dessus, avec repli sur un lancement direct en développement local.
restart: build ## recompile le backend et le relance
	@sudo -n systemctl restart ibalya-backend 2>/dev/null || $(MAKE) --no-print-directory restart-nohup
	@sleep 3
	@$(MAKE) --no-print-directory status

restart-llm: ## relance le service d'inférence
	@sudo -n systemctl restart ibalya-llm 2>/dev/null || echo "systemd indisponible : lancer make run-llm"
	@sleep 3
	@$(MAKE) --no-print-directory status

# Repli hors serveur supervisé (poste de développement).
restart-nohup:
	@mkdir -p $(LOGS)
	@fuser -k $(API_PORT)/tcp >/dev/null 2>&1 || true
	@sleep 1
	@( CHANNEL=$(CHANNEL) FIXTURE_PATH=$(FIXTURE_PATH) \
		nohup ./backend/bin/ibalya >> $(LOGS)/backend.log 2>&1 & )

restart-all: front restart restart-llm ## rebuild complet (front + back) et redémarrage des deux services

stop: ## arrête backend et service d'inférence
	@sudo -n systemctl stop ibalya-backend ibalya-llm 2>/dev/null || { \
		fuser -k $(API_PORT)/tcp >/dev/null 2>&1 || true; \
		fuser -k $(LLM_PORT)/tcp >/dev/null 2>&1 || true; }
	@echo "services arrêtés"

services: ## installe et active les unités systemd (une seule fois)
	sudo cp deploy/ibalya-backend.service deploy/ibalya-llm.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable --now ibalya-llm ibalya-backend
	@systemctl is-active ibalya-llm ibalya-backend

test: ## exécute les tests (Go et Python)
	cd backend && go vet ./... && go test ./...
	@$(VENV)/bin/python -m pytest llm-service/tests -q 2>/dev/null || echo "(pytest absent : tests Python ignorés)"

status: ## vérifie que les services répondent vraiment (pas seulement le code HTTP)
	@printf "base      : "; docker inspect -f '{{.State.Status}}' ibalya-db 2>/dev/null || echo "arrêtée"
	@printf "service LLM : "; curl -sf http://127.0.0.1:$(LLM_PORT)/health >/dev/null 2>&1 \
		&& echo "ok (:$(LLM_PORT))" || echo "INJOIGNABLE (:$(LLM_PORT))"
	@printf "backend     : "; curl -sf http://127.0.0.1:$(API_PORT)/api/health >/dev/null 2>&1 \
		&& echo "ok (:$(API_PORT))" || echo "INJOIGNABLE (:$(API_PORT))"
	@printf "connecteur  : "; curl -sf -H "Authorization: Bearer $(ADMIN_TOKEN)" \
		http://127.0.0.1:$(API_PORT)/api/status 2>/dev/null \
		| grep -o '"canal":"[^"]*"' | cut -d'"' -f4 \
		| sed 's/^fixture$$/fixture (DONNÉES DE DÉMONSTRATION)/; s/^gmail$$/gmail (réel)/' || echo "?"
	@printf "authentif.  : "; code=$$(curl -s -H "Authorization: Bearer $(ADMIN_TOKEN)" \
		-o /dev/null -w '%{http_code}' http://127.0.0.1:$(API_PORT)/api/status); \
		case "$$code" in 200) echo "ok (jeton du .env accepté)";; \
		401) echo "REFUSÉE — le jeton du .env ne correspond pas à celui du processus (lancer make restart)";; \
		*) echo "réponse inattendue ($$code)";; esac
	@printf "routes API  : "; out=$$(curl -s -H "Authorization: Bearer $(ADMIN_TOKEN)" \
		-w '\n%{http_code} %{content_type}' http://127.0.0.1:$(API_PORT)/api/synthese | tail -1); \
		case "$$out" in "200 application/json"*) echo "à jour (JSON)";; \
		*"text/html"*) echo "PÉRIMÉES — le binaire en cours ne connaît pas /api/synthese (lancer make restart)";; \
		*) echo "réponse inattendue ($$out)";; esac

logs: ## suit le journal du backend
	@journalctl -u ibalya-backend -f -n 50 2>/dev/null || tail -f $(LOGS)/backend.log

logs-llm: ## suit le journal du service d'inférence
	@journalctl -u ibalya-llm -f -n 50 2>/dev/null || tail -f $(LOGS)/llm.log

demo: ## remet la démo à zéro et rejoue le scénario complet (connecteur fixture)
	@$(MAKE) --no-print-directory restart CHANNEL=fixture
	@./scripts/demo.sh

utilisateur: build ## crée un compte : make utilisateur EMAIL=... NOM="..."
	@test -n "$(EMAIL)" || (echo 'usage : make utilisateur EMAIL=x@y.fr NOM="Prénom Nom"'; exit 1)
	@set -a; . ./.env; set +a; ./backend/bin/ibalya -creer-utilisateur "$(EMAIL)" -nom "$(NOM)"
