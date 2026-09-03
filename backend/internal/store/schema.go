package store

// Schéma logique du CDC section 5, appliqué au démarrage (idempotent).
const schema = `
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Comptes nominatifs : l'audit trail doit pouvoir désigner QUI a agi.
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  nom TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  actif BOOLEAN NOT NULL DEFAULT true,
  cree_le TIMESTAMPTZ NOT NULL DEFAULT now(),
  derniere_connexion TIMESTAMPTZ
);

-- Sessions : seul le HACHÉ du jeton est stocké, jamais le jeton lui-même.
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expire_le TIMESTAMPTZ NOT NULL,
  cree_le TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS oauth_tokens (
  provider TEXT PRIMARY KEY,
  token JSONB NOT NULL,
  account_email TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Date du consentement, distincte de updated_at. Cette dernière bouge à chaque
-- rafraîchissement du jeton d'accès ; le compte à rebours des sept jours du
-- mode Test de Google part, lui, du consentement. On le suit à part pour
-- pouvoir prévenir AVANT que la connexion meure.
ALTER TABLE oauth_tokens ADD COLUMN IF NOT EXISTS connecte_le TIMESTAMPTZ;
UPDATE oauth_tokens SET connecte_le=updated_at WHERE connecte_le IS NULL;

CREATE TABLE IF NOT EXISTS persons (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  email TEXT UNIQUE NOT NULL,
  type TEXT NOT NULL DEFAULT 'autre',
  sensitive BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS threads (
  id BIGSERIAL PRIMARY KEY,
  channel TEXT NOT NULL DEFAULT 'gmail',
  external_id TEXT NOT NULL,
  subject TEXT NOT NULL DEFAULT '',
  last_message_at TIMESTAMPTZ,
  response_rhythm_hours DOUBLE PRECISION,
  excluded BOOLEAN NOT NULL DEFAULT false,
  UNIQUE(channel, external_id)
);

CREATE TABLE IF NOT EXISTS messages (
  id BIGSERIAL PRIMARY KEY,
  thread_id BIGINT NOT NULL REFERENCES threads(id),
  external_id TEXT NOT NULL,
  channel TEXT NOT NULL DEFAULT 'gmail',
  sender TEXT NOT NULL,
  recipients TEXT[] NOT NULL DEFAULT '{}',
  sent_at TIMESTAMPTZ NOT NULL,
  subject TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  outbound BOOLEAN NOT NULL DEFAULT false,
  list_unsubscribe BOOLEAN NOT NULL DEFAULT false,
  status TEXT NOT NULL DEFAULT 'pending',
  exclude_reason TEXT,
  UNIQUE(channel, external_id)
);
CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id, sent_at);

CREATE TABLE IF NOT EXISTS engagements (
  id BIGSERIAL PRIMARY KEY,
  emetteur_id BIGINT REFERENCES persons(id),
  destinataire_id BIGINT REFERENCES persons(id),
  objet TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'autre',
  echeance DATE,
  echeance_inferee BOOLEAN NOT NULL DEFAULT false,
  echeance_confirmee BOOLEAN NOT NULL DEFAULT false,
  statut TEXT NOT NULL DEFAULT 'ouvert',
  confiance DOUBLE PRECISION NOT NULL DEFAULT 0,
  priorite TEXT NOT NULL DEFAULT 'normale',
  source_message_id BIGINT REFERENCES messages(id),
  thread_id BIGINT REFERENCES threads(id),
  cree_le TIMESTAMPTZ NOT NULL DEFAULT now(),
  maj_le TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_engagements_statut ON engagements(statut);
-- dédoublonnage : un message retraité ne recrée pas les mêmes engagements
CREATE UNIQUE INDEX IF NOT EXISTS uq_engagements_source
  ON engagements(source_message_id, objet) WHERE source_message_id IS NOT NULL;
-- migration : typage des engagements (devis, relance, rendez_vous, ...)
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'autre';

-- Verdict d'extraction : l'engagement était-il réel ?
--
-- Le statut disait ce qu'est devenu l'engagement, jamais si l'agent avait eu
-- raison de l'extraire. Les deux étaient confondus dans l'événement « corrige »,
-- où « statut: livré » — un succès — se comptait comme une correction. Toute
-- mesure de la qualité d'extraction s'en trouvait faussée.
--
--   juste    : c'était bien un engagement
--   faux     : l'agent a extrait quelque chose qui n'en était pas un
--   imprecis : engagement réel, mais objet ou échéance à côté
--   NULL     : pas encore tranché
ALTER TABLE engagements ADD COLUMN IF NOT EXISTS verdict_extraction TEXT;
CREATE INDEX IF NOT EXISTS idx_engagements_verdict
  ON engagements(verdict_extraction) WHERE verdict_extraction IS NOT NULL;


CREATE TABLE IF NOT EXISTS engagement_events (
  id BIGSERIAL PRIMARY KEY,
  engagement_id BIGINT NOT NULL REFERENCES engagements(id),
  type TEXT NOT NULL,
  horodatage TIMESTAMPTZ NOT NULL DEFAULT now(),
  source_message_id BIGINT REFERENCES messages(id),
  details JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_events_engagement ON engagement_events(engagement_id, horodatage);

-- Reprise du verdict d'extraction. Placée APRÈS engagement_events : la requête
-- la lit, et sur une base neuve la table n'existe pas avant cette ligne. Un
-- engagement livré était forcément réel ; « pas un engagement » est un verdict
-- explicite déjà donné par le dirigeant.
UPDATE engagements SET verdict_extraction='juste'
 WHERE verdict_extraction IS NULL
   AND (statut='livre' OR EXISTS (SELECT 1 FROM engagement_events v
        WHERE v.engagement_id=engagements.id AND v.details->>'statut'='livre'));
UPDATE engagements SET verdict_extraction='faux'
 WHERE EXISTS (SELECT 1 FROM engagement_events v
        WHERE v.engagement_id=engagements.id
          AND v.details->>'action'='pas_un_engagement');

CREATE TABLE IF NOT EXISTS dependency_links (
  id BIGSERIAL PRIMARY KEY,
  amont_id BIGINT NOT NULL REFERENCES engagements(id),
  aval_id BIGINT NOT NULL REFERENCES engagements(id),
  statut TEXT NOT NULL DEFAULT 'candidat',
  raison TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(amont_id, aval_id)
);
-- Score du jugement LLM sur la dépendance (0 = heuristique seule, non jugée).
ALTER TABLE dependency_links ADD COLUMN IF NOT EXISTS score DOUBLE PRECISION NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS capsule (
  id INT PRIMARY KEY DEFAULT 1,
  facts JSONB NOT NULL DEFAULT '{}',
  intentions JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO capsule (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS learned_rules (
  id BIGSERIAL PRIMARY KEY,
  portee_type TEXT NOT NULL,
  portee_cible TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS detections (
  id BIGSERIAL PRIMARY KEY,
  type TEXT NOT NULL,
  engagement_id BIGINT REFERENCES engagements(id),
  thread_id BIGINT REFERENCES threads(id),
  score DOUBLE PRECISION NOT NULL DEFAULT 0.5,
  titre TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  critique BOOLEAN NOT NULL DEFAULT false,
  payload JSONB NOT NULL DEFAULT '{}',
  statut TEXT NOT NULL DEFAULT 'nouvelle',
  dedup_key TEXT UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS drafts (
  id BIGSERIAL PRIMARY KEY,
  detection_id BIGINT REFERENCES detections(id),
  engagement_id BIGINT REFERENCES engagements(id),
  to_email TEXT NOT NULL,
  subject TEXT NOT NULL,
  body TEXT NOT NULL,
  statut TEXT NOT NULL DEFAULT 'propose',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS audit_log (
  id BIGSERIAL PRIMARY KEY,
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor TEXT NOT NULL DEFAULT 'agent',
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS reports (
  id BIGSERIAL PRIMARY KEY,
  type TEXT NOT NULL,
  content JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ════════════════════════════════════════════════════════════════════════
-- Multi-utilisateur, étape 1 : rattachement (additif, ne casse rien).
--
-- Chaque table de données reçoit un user_id. À ce stade il est NULLable et le
-- code ne le filtre pas encore : l'application se comporte exactement comme
-- avant. Les contraintes NOT NULL et d'unicité composite viendront avec le
-- code de cloisonnement (étape 2), une fois toutes les lignes rattachées.
--
-- Le propriétaire de l'existant est dérivé du canal connecté, pas codé en dur :
-- toute la base actuelle appartient au titulaire de la boîte Gmail raccordée.
-- ════════════════════════════════════════════════════════════════════════

ALTER TABLE persons           ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE threads           ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE messages          ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE engagements       ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE engagement_events ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE dependency_links  ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE capsule           ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE learned_rules     ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE detections        ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE drafts            ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE reports           ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE audit_log         ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE oauth_tokens      ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);
ALTER TABLE settings          ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id);

-- Rattachement de l'existant au propriétaire (titulaire du jeton Google).
-- Si aucun jeton (installation neuve), rien à faire : pas de données à migrer.
DO $migr$
DECLARE proprio BIGINT;
BEGIN
  SELECT u.id INTO proprio
    FROM users u
    JOIN oauth_tokens o ON lower(o.account_email) = lower(u.email)
   WHERE o.provider = 'google'
   LIMIT 1;
  IF proprio IS NOT NULL THEN
    UPDATE persons           SET user_id=proprio WHERE user_id IS NULL;
    UPDATE threads           SET user_id=proprio WHERE user_id IS NULL;
    UPDATE messages          SET user_id=proprio WHERE user_id IS NULL;
    UPDATE engagements       SET user_id=proprio WHERE user_id IS NULL;
    UPDATE engagement_events SET user_id=proprio WHERE user_id IS NULL;
    UPDATE dependency_links  SET user_id=proprio WHERE user_id IS NULL;
    UPDATE capsule           SET user_id=proprio WHERE user_id IS NULL;
    UPDATE learned_rules     SET user_id=proprio WHERE user_id IS NULL;
    UPDATE detections        SET user_id=proprio WHERE user_id IS NULL;
    UPDATE drafts            SET user_id=proprio WHERE user_id IS NULL;
    UPDATE reports           SET user_id=proprio WHERE user_id IS NULL;
    UPDATE audit_log         SET user_id=proprio WHERE user_id IS NULL;
    UPDATE oauth_tokens      SET user_id=proprio WHERE user_id IS NULL;
    UPDATE settings          SET user_id=proprio WHERE user_id IS NULL;
  END IF;
END
$migr$;

CREATE INDEX IF NOT EXISTS idx_persons_user     ON persons(user_id);
CREATE INDEX IF NOT EXISTS idx_threads_user     ON threads(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_user    ON messages(user_id);
CREATE INDEX IF NOT EXISTS idx_engagements_user ON engagements(user_id);
CREATE INDEX IF NOT EXISTS idx_events_user      ON engagement_events(user_id);
CREATE INDEX IF NOT EXISTS idx_links_user       ON dependency_links(user_id);
CREATE INDEX IF NOT EXISTS idx_rules_user       ON learned_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_detections_user  ON detections(user_id);
CREATE INDEX IF NOT EXISTS idx_drafts_user      ON drafts(user_id);
CREATE INDEX IF NOT EXISTS idx_reports_user     ON reports(user_id);
`
