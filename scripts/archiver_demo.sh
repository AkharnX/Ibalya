#!/usr/bin/env bash
# Archive puis retire les données de démonstration (connecteur « fixture »),
# en laissant intactes les données réelles issues de Gmail.
#
#   ./scripts/archiver_demo.sh            → archive et supprime
#   ./scripts/archiver_demo.sh --restaurer archives/demo_AAAAMMJJ-HHMM.sql
#
# Note : la démo est de toute façon reproductible par `make demo`, qui la
# régénère avec des dates recalculées. L'archive sert de filet, pas de source.
set -euo pipefail
cd "$(dirname "$0")/.."
BASE="docker exec -i ibalya-db psql -U ibalya -d ibalya"

if [ "${1:-}" = "--restaurer" ]; then
  fichier="${2:?usage : --restaurer archives/demo_....sql}"
  echo "Restauration depuis $fichier"
  $BASE -q < "$fichier"
  echo "✓ données de démonstration réinjectées"
  exit 0
fi

horodatage=$(date +%Y%m%d-%H%M)
archive="archives/demo_$horodatage.sql"
mkdir -p archives

# Périmètre : tout ce qui découle du connecteur fixture.
PERIMETRE="
CREATE TEMP VIEW f_threads AS SELECT * FROM threads WHERE channel='fixture';
CREATE TEMP VIEW f_messages AS SELECT * FROM messages WHERE channel='fixture';
CREATE TEMP VIEW f_engagements AS SELECT * FROM engagements
  WHERE thread_id IN (SELECT id FROM f_threads) OR source_message_id IN (SELECT id FROM f_messages);
"

echo "━━━ Archivage vers $archive ━━━"
{
  echo "-- Données de démonstration Ibalya, archivées le $(date '+%d/%m/%Y à %H:%M')"
  echo "-- Restauration : ./scripts/archiver_demo.sh --restaurer $archive"
  echo "BEGIN;"
  docker exec -i ibalya-db psql -U ibalya -d ibalya -qAt <<SQL
$PERIMETRE
SELECT 'INSERT INTO threads VALUES ('||quote_literal(id::text)||'::bigint,'||quote_literal(channel)||','||quote_literal(external_id)||','||quote_literal(subject)||','||coalesce(quote_literal(last_message_at::text)||'::timestamptz','NULL')||','||coalesce(response_rhythm_hours::text,'NULL')||','||excluded||') ON CONFLICT DO NOTHING;' FROM f_threads;
SELECT 'INSERT INTO messages VALUES ('||id||','||thread_id||','||quote_literal(external_id)||','||quote_literal(channel)||','||quote_literal(sender)||','||quote_literal(recipients::text)||'::text[],'||quote_literal(sent_at::text)||'::timestamptz,'||quote_literal(subject)||','||quote_literal(body)||','||outbound||','||list_unsubscribe||','||quote_literal(status)||','||coalesce(quote_literal(exclude_reason),'NULL')||') ON CONFLICT DO NOTHING;' FROM f_messages;
SELECT 'INSERT INTO engagements (id,emetteur_id,destinataire_id,objet,type,echeance,echeance_inferee,echeance_confirmee,statut,confiance,priorite,source_message_id,thread_id,cree_le,maj_le) VALUES ('||id||','||coalesce(emetteur_id::text,'NULL')||','||coalesce(destinataire_id::text,'NULL')||','||quote_literal(objet)||','||quote_literal(type)||','||coalesce(quote_literal(echeance::text)||'::date','NULL')||','||echeance_inferee||','||echeance_confirmee||','||quote_literal(statut)||','||confiance||','||quote_literal(priorite)||','||coalesce(source_message_id::text,'NULL')||','||coalesce(thread_id::text,'NULL')||','||quote_literal(cree_le::text)||'::timestamptz,'||quote_literal(maj_le::text)||'::timestamptz) ON CONFLICT DO NOTHING;' FROM f_engagements;
SQL
  echo "COMMIT;"
} > "$archive"

lignes=$(grep -c '^INSERT' "$archive" || true)
echo "  $lignes enregistrement(s) archivé(s) — $(du -h "$archive" | cut -f1)"

echo "━━━ Suppression (ordre des dépendances) ━━━"
docker exec -i ibalya-db psql -U ibalya -d ibalya -q <<SQL
BEGIN;
$PERIMETRE
CREATE TEMP VIEW f_eng_ids AS SELECT id FROM f_engagements;

DELETE FROM drafts WHERE engagement_id IN (SELECT id FROM f_eng_ids)
   OR detection_id IN (SELECT id FROM detections WHERE engagement_id IN (SELECT id FROM f_eng_ids)
                                                    OR thread_id IN (SELECT id FROM f_threads));
DELETE FROM detections WHERE engagement_id IN (SELECT id FROM f_eng_ids)
   OR thread_id IN (SELECT id FROM f_threads);
DELETE FROM dependency_links WHERE amont_id IN (SELECT id FROM f_eng_ids)
   OR aval_id IN (SELECT id FROM f_eng_ids);
DELETE FROM engagement_events WHERE engagement_id IN (SELECT id FROM f_eng_ids)
   OR source_message_id IN (SELECT id FROM f_messages);
DELETE FROM engagements WHERE id IN (SELECT id FROM f_eng_ids);
DELETE FROM messages WHERE channel='fixture';
DELETE FROM threads WHERE channel='fixture';
-- interlocuteurs devenus orphelins (plus aucun message ni engagement réel)
DELETE FROM persons p WHERE NOT EXISTS (SELECT 1 FROM messages m WHERE m.sender = p.email OR p.email = ANY(m.recipients))
  AND NOT EXISTS (SELECT 1 FROM engagements e WHERE e.emetteur_id = p.id OR e.destinataire_id = p.id);
COMMIT;
SQL

echo "━━━ Vérification ━━━"
docker exec -i ibalya-db psql -U ibalya -d ibalya -tAc "
SELECT '  messages restants   : '||count(*)||' (tous gmail : '||(count(*) = count(*) FILTER (WHERE channel='gmail'))||')' FROM messages;
SELECT '  engagements         : '||count(*) FROM engagements;
SELECT '  détections          : '||count(*) FROM detections;
SELECT '  brouillons          : '||count(*) FROM drafts;
SELECT '  interlocuteurs      : '||count(*) FROM persons;"
echo
echo "Archive conservée : $archive"
echo "Pour rejouer la démo plus tard : make demo (régénère tout avec des dates fraîches)"
