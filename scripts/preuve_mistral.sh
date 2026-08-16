#!/usr/bin/env bash
# Preuve en direct que l'extraction passe par l'API Mistral.
cd "$(dirname "$0")/.."
source .env
echo "════════════════════════════════════════════════════════"
echo " Modèle configuré : $MISTRAL_MODEL"
echo " Clé API          : ${MISTRAL_API_KEY:0:6}…${MISTRAL_API_KEY: -4}"
echo "════════════════════════════════════════════════════════"
echo "▶ Réponse de Mistral :"
curl -s -X POST http://127.0.0.1:8092/extract -H 'Content-Type: application/json' -d '{
  "messages":[{"id":1,"sender":"artisan@demo.fr","to":"client@demo.fr","subject":"Point dossier",
  "body":"Je ne serai pas disponible la semaine prochaine donc je vous renseigne dès mon retour. Mardi prochain","sent_at":"'"$(date -Iseconds)"'"}],
  "open_engagements":[],"capsule":{},"account_email":"artisan@demo.fr"}' \
  | python3 -c "
import json,sys
e = json.load(sys.stdin)['results'][0]['engagements'][0]
print(f\"    Engagement  : {e['objet']}\")
print(f\"    Type        : {e['type']}\")
print(f\"    Échéance    : {e['echeance']} \")
print(f\"    Signalée    : {'déduite, à confirmer' if e['echeance_inferee'] else 'explicite'}\")
print(f\"    Confiance   : {e['confiance']*100:.0f} %\")"
echo
echo "▶ Trafic HTTP sortant vers api.mistral.ai (journal du service) :"
grep -c 'HTTP/1.1 200 OK' /tmp/claude-1000/-home-akharn-agnet-ia/e7091be9-1105-47fb-8929-193067aad6f7/scratchpad/llm.log 2>/dev/null | sed 's/^/    appels traités : /'
echo "════════════════════════════════════════════════════════"
