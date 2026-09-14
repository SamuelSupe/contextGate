CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sources (id TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS agents (id TEXT PRIMARY KEY, value TEXT NOT NULL, token_hash TEXT UNIQUE);
CREATE TABLE IF NOT EXISTS configuration_agents (id TEXT PRIMARY KEY, value TEXT NOT NULL, token_hash TEXT UNIQUE);
CREATE TABLE IF NOT EXISTS sessions (hash TEXT PRIMARY KEY, csrf TEXT NOT NULL, expires BIGINT NOT NULL);
CREATE TABLE IF NOT EXISTS audit (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at BIGINT NOT NULL, agent_id TEXT NOT NULL, source_id TEXT NOT NULL,
    operation TEXT NOT NULL, fingerprint TEXT NOT NULL,
    elapsed_ms BIGINT NOT NULL, rows BIGINT NOT NULL, error_code TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '', native_code TEXT NOT NULL DEFAULT '',
    preview BOOLEAN NOT NULL DEFAULT FALSE,
    template_id TEXT NOT NULL DEFAULT '', template_version TEXT NOT NULL DEFAULT '',
    ontology_id TEXT NOT NULL DEFAULT '', ontology_version TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS audit_at ON audit(at);
CREATE INDEX IF NOT EXISTS audit_agent ON audit(agent_id,id);
CREATE TABLE IF NOT EXISTS oauth (
    kind TEXT NOT NULL, id TEXT NOT NULL, value TEXT NOT NULL, expires BIGINT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '', active SMALLINT NOT NULL DEFAULT 1,
    PRIMARY KEY(kind,id)
);
CREATE TABLE IF NOT EXISTS semantics_state (
    source_id TEXT PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL, value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS semantics_entries (
    source_id TEXT NOT NULL REFERENCES semantics_state(source_id) ON DELETE CASCADE,
    phase TEXT NOT NULL, id TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(source_id,phase,id)
);
CREATE TABLE IF NOT EXISTS semantics_evidence (
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    definition TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(source_id,definition)
);
CREATE TABLE IF NOT EXISTS ontologies (id TEXT PRIMARY KEY, revision BIGINT NOT NULL, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS ontology_versions (
    ontology_id TEXT NOT NULL REFERENCES ontologies(id) ON DELETE CASCADE,
    version BIGINT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(ontology_id,version)
);
CREATE TABLE IF NOT EXISTS ontology_bindings (
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    phase TEXT NOT NULL, ontology_id TEXT NOT NULL, version BIGINT NOT NULL, PRIMARY KEY(source_id,phase)
);
CREATE INDEX IF NOT EXISTS ontology_binding_reference ON ontology_bindings(ontology_id,version);
CREATE TABLE IF NOT EXISTS evaluation_questions (
    seq BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    id TEXT NOT NULL, revision BIGINT NOT NULL, value TEXT NOT NULL, UNIQUE(source_id,id)
);
CREATE INDEX IF NOT EXISTS evaluation_questions_source ON evaluation_questions(source_id,seq);
CREATE TABLE IF NOT EXISTS evaluations (
    seq BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    id TEXT NOT NULL, revision BIGINT NOT NULL, value TEXT NOT NULL, UNIQUE(source_id,id)
);
CREATE INDEX IF NOT EXISTS evaluations_source ON evaluations(source_id,seq);
CREATE TABLE IF NOT EXISTS semantics_versions (
    source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    version BIGINT NOT NULL, published_at BIGINT NOT NULL, entry_count INTEGER NOT NULL,
    ontology_id TEXT, ontology_version BIGINT, value TEXT NOT NULL,
    PRIMARY KEY(source_id,version),
    FOREIGN KEY(ontology_id,ontology_version) REFERENCES ontology_versions(ontology_id,version)
);
CREATE INDEX IF NOT EXISTS semantics_versions_ontology ON semantics_versions(ontology_id,ontology_version);
CREATE TABLE IF NOT EXISTS source_health (
    source_id TEXT PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
    value TEXT NOT NULL
);
ALTER TABLE audit ADD COLUMN IF NOT EXISTS event_kind TEXT NOT NULL DEFAULT 'query';
ALTER TABLE audit ADD COLUMN IF NOT EXISTS resource_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit ADD COLUMN IF NOT EXISTS revision TEXT NOT NULL DEFAULT '';
ALTER TABLE audit ADD COLUMN IF NOT EXISTS changed_fields TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS audit_request ON audit(request_id,id);
