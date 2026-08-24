#!/usr/bin/env bash
# Sauvegarde de la base Ibalya.
#
# Une sauvegarde qu'on n'a jamais restaurée n'est pas une sauvegarde : le mode
# --verifier restaure le dernier fichier dans une base jetable et compare les
# volumes, ce qui est la seule preuve qu'il est exploitable.
#
#   ./scripts/sauvegarde.sh              sauvegarde et rotation
#   ./scripts/sauvegarde.sh --verifier   restaure la dernière dans une base jetable
#   ./scripts/sauvegarde.sh --restaurer <fichier>   écrase la base courante
set -euo pipefail
cd "$(dirname "$0")/.."

CONTENEUR="${CONTENEUR:-ibalya-db}"
BASE="${BASE:-ibalya}"
UTILISATEUR="${UTILISATEUR:-ibalya}"
DOSSIER="${DOSSIER:-$HOME/sauvegardes/ibalya}"
RETENTION_JOURS="${RETENTION_JOURS:-30}"

# Les tables dont le contenu se reconstruit par une nouvelle lecture de la
# boîte ne sont pas exclues : leur volume est modeste et une restauration
# partielle laisserait l'agent dans un état incohérent.

sauvegarder() {
  mkdir -p "$DOSSIER"
  local horodatage fichier
  horodatage=$(date +%Y%m%d-%H%M%S)
  fichier="$DOSSIER/ibalya-$horodatage.sql.gz"

  docker exec "$CONTENEUR" pg_dump -U "$UTILISATEUR" -d "$BASE" --clean --if-exists \
    | gzip -9 > "$fichier"

  # Un fichier vide ou tronqué passerait inaperçu jusqu'au jour où on en a besoin.
  if [ ! -s "$fichier" ] || ! gzip -t "$fichier" 2>/dev/null; then
    echo "ÉCHEC : l'archive $fichier est vide ou corrompue" >&2
    rm -f "$fichier"
    exit 1
  fi
  sha256sum "$fichier" > "$fichier.sha256"

  local taille lignes
  taille=$(du -h "$fichier" | cut -f1)
  lignes=$(gzip -dc "$fichier" | grep -c '^INSERT\|^COPY' || true)
  echo "✓ $fichier ($taille, $lignes instruction(s) de données)"

  # Rotation : on garde les archives récentes et la plus ancienne du mois.
  find "$DOSSIER" -name 'ibalya-*.sql.gz' -mtime "+$RETENTION_JOURS" -print -delete \
    | sed 's/^/  purgée : /'
}

verifier() {
  local dernier
  dernier=$(ls -t "$DOSSIER"/ibalya-*.sql.gz 2>/dev/null | head -1)
  [ -n "$dernier" ] || { echo "aucune sauvegarde dans $DOSSIER" >&2; exit 1; }
  echo "━━━ Vérification de $(basename "$dernier") ━━━"

  sha256sum -c "$dernier.sha256" >/dev/null && echo "  empreinte conforme"

  local jetable="verif_$(date +%s)"
  docker exec "$CONTENEUR" createdb -U "$UTILISATEUR" "$jetable"
  # shellcheck disable=SC2064
  trap "docker exec $CONTENEUR dropdb -U $UTILISATEUR --if-exists $jetable >/dev/null 2>&1 || true" EXIT

  gzip -dc "$dernier" | docker exec -i "$CONTENEUR" psql -U "$UTILISATEUR" -d "$jetable" -q >/dev/null 2>&1

  local ecarts=0
  for t in messages threads engagements engagement_events detections drafts persons \
           capsule learned_rules audit_log reports users settings oauth_tokens; do
    local vif restaure
    vif=$(docker exec "$CONTENEUR" psql -U "$UTILISATEUR" -d "$BASE" -tAc "select count(*) from $t" 2>/dev/null || echo "?")
    restaure=$(docker exec "$CONTENEUR" psql -U "$UTILISATEUR" -d "$jetable" -tAc "select count(*) from $t" 2>/dev/null || echo "?")
    if [ "$vif" = "$restaure" ]; then
      printf "  %-20s %6s ✓\n" "$t" "$restaure"
    else
      printf "  %-20s vif %s ≠ restauré %s ✗\n" "$t" "$vif" "$restaure"
      ecarts=$((ecarts + 1))
    fi
  done
  [ "$ecarts" -eq 0 ] && echo "  sauvegarde exploitable" || { echo "  $ecarts écart(s)" >&2; exit 1; }
}

restaurer() {
  local fichier="${1:?usage : --restaurer <fichier.sql.gz>}"
  [ -f "$fichier" ] || { echo "fichier introuvable : $fichier" >&2; exit 1; }
  echo "Cette opération écrase la base $BASE."
  read -r -p "Taper le nom de la base pour confirmer : " reponse
  [ "$reponse" = "$BASE" ] || { echo "annulé"; exit 1; }
  gzip -dc "$fichier" | docker exec -i "$CONTENEUR" psql -U "$UTILISATEUR" -d "$BASE" -q
  echo "✓ base restaurée depuis $(basename "$fichier")"
}

case "${1:-}" in
  --verifier)  verifier ;;
  --restaurer) restaurer "${2:-}" ;;
  "")          sauvegarder ;;
  *)           echo "usage : $0 [--verifier | --restaurer <fichier>]" >&2; exit 1 ;;
esac
