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

CREATE TABLE IF NOT EXISTS engagement_events (
  id BIGSERIAL PRIMARY KEY,
  engagement_id BIGINT NOT NULL REFERENCES engagements(id),
  type TEXT NOT NULL,
  horodatage TIMESTAMPTZ NOT NULL DEFAULT now(),
  source_message_id BIGINT REFERENCES messages(id),
  details JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_events_engagement ON engagement_events(engagement_id, horodatage);

CREATE TABLE IF NOT EXISTS dependency_links (
  id BIGSERIAL PRIMARY KEY,
  amont_id BIGINT NOT NULL REFERENCES engagements(id),
  aval_id BIGINT NOT NULL REFERENCES engagements(id),
  statut TEXT NOT NULL DEFAULT 'candidat',
  raison TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(amont_id, aval_id)
);

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
`
