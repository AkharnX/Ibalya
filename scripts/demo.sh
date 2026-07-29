#!/usr/bin/env bash
# Démo complète AgentOps — à lancer avant/pendant la réunion.
# Rejoue tout le parcours J+0 → J+7 du CDC sur le scénario « Menuiserie Dupont » :
# ingestion → pré-filtre → extraction → graphe → 5 détecteurs → miroir → digest → brouillons.
set -euo pipefail
cd "$(dirname "$0")/.."
source .env
API="http://127.0.0.1:9999/api"
H="Authorization: Bearer $ADMIN_TOKEN"

step() { echo; echo "━━━ $1 ━━━"; }

step "0. Fixtures régénérées (dates relatives à aujourd'hui)"
python3 scripts/gen_fixtures.py fixtures/messages.json

step "1. Remise à zéro de la base"
docker exec agentops-db psql -U agentops -d agentops -q -c \
  "TRUNCATE audit_log, drafts, detections, dependency_links, engagement_events, engagements, learned_rules, messages, threads, persons, reports RESTART IDENTITY CASCADE;
   UPDATE capsule SET facts='{}', intentions='{}' WHERE id=1;
   DELETE FROM settings WHERE key IN ('dernier_cycle','dernier_digest');"

step "2. Onboarding J+0 → J+1 : lecture 30 jours, miroir, capsule"
curl -sf -X POST -H "$H" "$API/onboarding/run" > /dev/null
for i in $(seq 1 30); do
  sleep 1
  PENDING=$(curl -sf -H "$H" "$API/status" | python3 -c "import json,sys; print(json.load(sys.stdin)['compteurs']['messages_en_attente'])")
  [ "$PENDING" = "0" ] && break
done
sleep 2

step "3. Le dirigeant confirme LE lien pertinent (vitrage → pose mairie), rejette le reste"
curl -sf -H "$H" "$API/links?statut=candidat" | python3 -c "
import json, sys
for l in json.load(sys.stdin) or []:
    pertinent = 'vitrage' in l['amont_objet'].lower() and 'fen' in l['aval_objet'].lower()
    action = 'confirm' if pertinent else 'reject'
    print(f\"{l['id']} {action}\")" | while read -r ID ACTION; do
  curl -sf -X POST -H "$H" "$API/links/$ID/$ACTION" > /dev/null
  echo "  lien #$ID → $ACTION"
done

step "4. Nouveau cycle : le détecteur de contradiction s'active"
curl -sf -X POST -H "$H" -d '{"since_days":1}' "$API/cycle/run" | python3 -c "
import json,sys; r=json.load(sys.stdin); print('  détections:', r.get('detection',{}).get('detections'))"

step "5. Digest du jour (avec brouillons pré-rédigés)"
curl -sf -X POST -H "$H" -d '{"type":"quotidien"}' "$API/digest/generate" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f\"  {len(d.get('detections') or [])} détections · {len(d.get('brouillons_proposes') or [])} brouillons · {len(d.get('engagements_a_risque') or [])} engagements à risque\")
for x in d.get('detections') or []:
    m = ' [CRITIQUE]' if x['critique'] else ''
    print(f\"  •{m} [{x['type']}] {x['titre']}\")"

step "RÉSUMÉ"
curl -sf -H "$H" "$API/status" | python3 -m json.tool
echo
echo "Tableau de bord : http://$(hostname -I | awk '{print $1}'):9999  (jeton dans .env)"
