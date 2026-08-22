#!/usr/bin/env bash
# Déploiement d'Ibalya sur le serveur. Lancé par le CD, ou à la main.
#
# Deux garanties :
#   1. la version précédente est restaurée si N'IMPORTE QUELLE étape échoue ;
#   2. la construction utilise le Node de nvm, pas celui du système.
set -euo pipefail
cd "$(dirname "$0")/.."

BRANCHE="${BRANCHE:-main}"
# Commit effectivement construit, redémarré et déclaré sain.
EMPREINTE=".deploy-stamp"
etape() { echo; echo "━━━ $1 ━━━"; }

# Le déploiement s'exécute dans un shell non interactif : nvm n'est pas chargé
# et `node` retombe sur la version système (18), que Vite refuse.
if [ -s "$HOME/.nvm/nvm.sh" ]; then
  # shellcheck disable=SC1091
  . "$HOME/.nvm/nvm.sh" >/dev/null 2>&1 || true
fi
version_node="$(node --version 2>/dev/null | sed 's/^v//;s/\..*//')"
if [ -z "$version_node" ] || [ "$version_node" -lt 20 ]; then
  echo "Node 20 ou plus est requis pour construire le frontend."
  echo "Version trouvée : $(node --version 2>/dev/null || echo aucune)"
  exit 1
fi
echo "Node $(node --version) utilisé pour la construction."

precedente="$(git rev-parse HEAD)"
branche_precedente="$(git rev-parse --abbrev-ref HEAD)"
etape "Version en cours : ${precedente:0:8} ($branche_precedente)"

construire() {
  (cd backend && go build -o bin/ibalya ./cmd/server)
  (cd frontend && npm ci --silent && npm run build --silent)
}

# Toute sortie en erreur restaure la version précédente : build cassé,
# redémarrage impossible, ou contrôle de santé négatif. Sans ce filet, un build
# raté laisse le dépôt sur la nouvelle version, les fichiers servis
# statiquement disparaissent et le site tombe en 404.
restaurer() {
  code=$?
  [ "$code" -eq 0 ] && exit 0
  etape "ÉCHEC (code $code) — retour à ${precedente:0:8}"
  git checkout --quiet "$branche_precedente" 2>/dev/null || true
  git reset --hard --quiet "$precedente"
  construire || echo "  ✗ la reconstruction de la version précédente a échoué"
  sudo systemctl restart ibalya-llm ibalya-backend || true
  sleep 5
  if curl -sf http://127.0.0.1:9999/api/health >/dev/null; then
    echo "  ✓ version précédente restaurée et fonctionnelle"
  else
    echo "  ✗ ATTENTION : la version précédente ne répond pas non plus"
  fi
  exit "$code"
}
trap restaurer EXIT

etape "Récupération de $BRANCHE"
git fetch --quiet origin "$BRANCHE"
cible="$(git rev-parse "origin/$BRANCHE")"
# On compare au commit réellement construit et mis en service (empreinte écrite
# après le contrôle de santé), et non au HEAD du dépôt. Un arbre déplacé à la
# main pointait déjà sur la cible : le déploiement concluait « rien à faire »,
# sautait la construction et laissait tourner l'ancien binaire en annonçant un
# succès. C'est ainsi qu'une route livrée est restée absente en production.
deploye=""
[ -f "$EMPREINTE" ] && deploye="$(cat "$EMPREINTE")"
if [ "$cible" = "$deploye" ]; then
  echo "  ${cible:0:8} déjà construit et en service, rien à faire"
  trap - EXIT
  exit 0
fi
if [ "$cible" = "$precedente" ] && [ -z "$deploye" ]; then
  echo "  arbre déjà sur ${cible:0:8}, mais rien ne prouve que les binaires en"
  echo "  correspondent : on reconstruit."
fi
git checkout --quiet "$BRANCHE"
git reset --hard --quiet "$cible"
echo "  → ${cible:0:8}"

etape "Construction"
construire
echo "  binaire et interface construits"

etape "Redémarrage"
sudo systemctl restart ibalya-llm ibalya-backend

etape "Contrôle de santé"
sante=1
for i in $(seq 1 15); do
  sleep 2
  code_http="$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:9999/api/health || true)"
  type_contenu="$(curl -s -o /dev/null -w '%{content_type}' http://127.0.0.1:9999/api/synthese || true)"
  # 401 attendu sans jeton : ce qui compte est que l'API réponde en JSON et non
  # la page du tableau de bord servie par le repli SPA.
  if [ "$code_http" = "200" ] && [[ "$type_contenu" == application/json* ]]; then
    sante=0; echo "  ✓ service opérationnel après $i tentative(s)"; break
  fi
done
[ $sante -eq 0 ] || exit 1

trap - EXIT
echo "$cible" > "$EMPREINTE"
etape "Déployé : ${cible:0:8}"
git --no-pager log -1 --format='  %s%n  par %an, %ar'
