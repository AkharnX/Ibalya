#!/usr/bin/env bash
# Déploiement d'Ibalya sur le serveur. Lancé par le CD, ou à la main.
#
# Principe : on note la version en cours AVANT toute modification, et si la
# nouvelle ne répond pas au contrôle de santé, on revient en arrière
# automatiquement. Un déploiement raté ne doit jamais laisser le service à terre.
set -euo pipefail
cd "$(dirname "$0")/.."

BRANCHE="${BRANCHE:-main}"
etape() { echo; echo "━━━ $1 ━━━"; }

precedente=$(git rev-parse HEAD)
etape "Version en cours : ${precedente:0:8}"

etape "Récupération de $BRANCHE"
git fetch --quiet origin "$BRANCHE"
cible=$(git rev-parse "origin/$BRANCHE")
if [ "$cible" = "$precedente" ]; then
  echo "  déjà à jour, rien à faire"
  exit 0
fi
git checkout --quiet "$BRANCHE"
git reset --hard --quiet "$cible"
echo "  → ${cible:0:8}"

construire() {
  (cd backend && go build -o bin/ibalya ./cmd/server)
  (cd frontend && npm ci --silent && npm run build --silent)
}

etape "Construction"
construire
echo "  binaire et interface construits"

etape "Redémarrage"
sudo systemctl restart ibalya-llm ibalya-backend

etape "Contrôle de santé"
sante=1
for i in $(seq 1 15); do
  sleep 2
  code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:9999/api/health || true)
  contenu=$(curl -s -o /dev/null -w '%{content_type}' http://127.0.0.1:9999/api/synthese || true)
  # 401 attendu sans jeton : ce qui compte est que l'API réponde en JSON et
  # non la page du tableau de bord servie par le repli SPA.
  if [ "$code" = "200" ] && [[ "$contenu" == application/json* ]]; then
    sante=0; echo "  ✓ service opérationnel après ${i} tentative(s)"; break
  fi
done

if [ $sante -ne 0 ]; then
  etape "ÉCHEC — retour à ${precedente:0:8}"
  git reset --hard --quiet "$precedente"
  construire
  sudo systemctl restart ibalya-llm ibalya-backend
  sleep 5
  curl -sf http://127.0.0.1:9999/api/health >/dev/null \
    && echo "  ✓ version précédente restaurée et fonctionnelle" \
    || echo "  ✗ ATTENTION : la version précédente ne répond pas non plus"
  exit 1
fi

etape "Déployé : ${cible:0:8}"
git --no-pager log -1 --format='  %s%n  par %an, %ar'
