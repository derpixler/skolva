package auth

import "github.com/derpixler/skolva/internal/core/module"

// identityMigrations is the Core-slice schema owned by the identity module:
// the shared base helpers plus the RBAC core (users, roles, permissions and
// their joins) with the seeded roles/permissions.
//
// This is the proof-of-concept for the per-module migration cutover. The
// remaining identity objects (2FA secrets, the users_meta EAV table, audit
// logging, full-text search) are still carried by schema.sql and move here
// when the module is extracted to skolva-core.
var identityMigrations = []module.Migration{
	{Version: 1, Name: "identity_core", SQL: identityCoreSQL},
}

const identityCoreSQL = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION prevent_permanent_delete()
RETURNS TRIGGER AS $$
BEGIN
  IF COALESCE(current_setting('app.allow_delete', true), '') = '1' THEN
    RETURN OLD;
  END IF;
  RAISE EXCEPTION 'Physisches DELETE in % ist untersagt. Nutzen Sie deleted_at (Soft-Delete).', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  first_name TEXT NOT NULL,
  last_name TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  is_protected BOOLEAN NOT NULL DEFAULT FALSE,
  anonymized_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_users_email_active
  ON users (lower(email))
  WHERE deleted_at IS NULL AND anonymized_at IS NULL;
CREATE INDEX ix_users_deleted_at ON users(deleted_at);

CREATE TRIGGER tr_users_updated_at
BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER tr_users_block_delete
BEFORE DELETE ON users FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

CREATE TABLE roles (
  slug TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  description TEXT,
  is_protected BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_roles_deleted_at ON roles(deleted_at);
CREATE TRIGGER tr_roles_updated_at
BEFORE UPDATE ON roles FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER tr_roles_block_delete
BEFORE DELETE ON roles FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

CREATE TABLE permissions (
  slug TEXT PRIMARY KEY,
  description TEXT NOT NULL,
  is_protected BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE user_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_slug TEXT NOT NULL REFERENCES roles(slug) ON DELETE RESTRICT,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  assigned_by UUID REFERENCES users(id) ON DELETE SET NULL,
  PRIMARY KEY (user_id, role_slug)
);
CREATE INDEX ix_user_roles_user_id ON user_roles(user_id);
CREATE INDEX ix_user_roles_role_slug ON user_roles(role_slug);

CREATE TABLE role_permissions (
  role_slug TEXT NOT NULL REFERENCES roles(slug) ON DELETE CASCADE,
  permission_slug TEXT NOT NULL REFERENCES permissions(slug) ON DELETE CASCADE,
  PRIMARY KEY (role_slug, permission_slug)
);
CREATE INDEX ix_role_permissions_role_slug ON role_permissions(role_slug);
CREATE INDEX ix_role_permissions_permission_slug ON role_permissions(permission_slug);

INSERT INTO roles (slug, display_name, description, is_protected) VALUES
  ('admin',     'Administrator',   'Vollzugriff auf alle Funktionen',              TRUE),
  ('vorstand',  'Vorstand',        'Vereinsvorstand mit erweiterten Rechten',      TRUE),
  ('kassierer', 'Kassierer',       'Finanzverwaltung und Buchhaltung',             TRUE),
  ('mitglied',  'Mitglied',        'Regulaeres Vereinsmitglied',                   FALSE),
  ('pruefer',   'Pruefer',         'Kassenpruefer mit Leserechten auf Finanzen',   FALSE);

INSERT INTO permissions (slug, description, is_protected) VALUES
  ('users.read', 'Benutzer anzeigen', TRUE),
  ('users.write', 'Benutzer anlegen und bearbeiten', TRUE),
  ('users.delete', 'Benutzer loeschen', TRUE),
  ('units.read', 'Pachteinheiten anzeigen', TRUE),
  ('units.write', 'Pachteinheiten anlegen und bearbeiten', TRUE),
  ('units.delete', 'Pachteinheiten loeschen', TRUE),
  ('leases.read', 'Mietvertraege anzeigen', TRUE),
  ('leases.write', 'Mietvertraege anlegen und bearbeiten', TRUE),
  ('leases.delete', 'Mietvertraege loeschen', TRUE),
  ('ownership.read', 'Eigentumsverhaeltnisse anzeigen', TRUE),
  ('ownership.write', 'Eigentumsverhaeltnisse bearbeiten', TRUE),
  ('accounting.read', 'Buchhaltung anzeigen', TRUE),
  ('accounting.write', 'Buchungen erstellen', TRUE),
  ('accounting.lock', 'Journal sperren', TRUE),
  ('billing.read', 'Abrechnungen anzeigen', TRUE),
  ('billing.write', 'Abrechnungen erstellen', TRUE),
  ('billing.approve', 'Abrechnungen freigeben', TRUE),
  ('banking.import', 'Bankdaten importieren', TRUE),
  ('banking.rules', 'Kategorisierungsregeln verwalten', TRUE),
  ('documents.read', 'Dokumente anzeigen', TRUE),
  ('documents.write', 'Dokumente hochladen', TRUE),
  ('documents.delete', 'Dokumente loeschen', TRUE),
  ('audit.read', 'Audit-Log anzeigen', TRUE),
  ('applicants.read', 'Bewerbungen anzeigen', TRUE),
  ('applicants.write', 'Bewerbungen bearbeiten', TRUE),
  ('applicants.assign', 'Pachteinheit zuweisen', TRUE),
  ('groups.read', 'Gruppen anzeigen', TRUE),
  ('groups.write', 'Gruppen verwalten', TRUE),
  ('sharing.read', 'Shared Links anzeigen', TRUE),
  ('sharing.write', 'QR-Codes/Links erstellen', TRUE),
  ('metering.read', 'Zaehlerstaende anzeigen', TRUE),
  ('metering.write', 'Zaehlerstaende erfassen', TRUE),
  ('operations.read', 'Inventar/Kautionen/Spesen anzeigen', TRUE),
  ('operations.write', 'Inventar/Kautionen/Spesen bearbeiten', TRUE),
  ('lending.read', 'Geraeteverleih anzeigen', TRUE),
  ('lending.write', 'Geraeteverleih verwalten', TRUE),
  ('workhours.read', 'Arbeitsstunden anzeigen', TRUE),
  ('workhours.write', 'Arbeitsstunden verwalten', TRUE),
  ('workhours.plan', 'Arbeitseinsaetze planen', TRUE),
  ('compliance.read', 'Verstoesse/Mahnungen anzeigen', TRUE),
  ('compliance.write', 'Verstoesse/Mahnungen erstellen', TRUE),
  ('compliance.approve', 'Mahnungen/Abmahnungen freigeben', TRUE),
  ('webhooks.read', 'Webhooks anzeigen', TRUE),
  ('webhooks.write', 'Webhooks verwalten', TRUE),
  ('admin.jobs', 'Job-Queue verwalten (Retry/Cancel)', TRUE),
  ('meta.read', 'Metadaten lesen', TRUE),
  ('meta.write', 'Metadaten schreiben', TRUE);

INSERT INTO role_permissions (role_slug, permission_slug)
SELECT 'admin', slug FROM permissions;

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('vorstand', 'users.read'), ('vorstand', 'users.write'),
  ('vorstand', 'units.read'), ('vorstand', 'units.write'),
  ('vorstand', 'leases.read'), ('vorstand', 'leases.write'),
  ('vorstand', 'ownership.read'), ('vorstand', 'ownership.write'),
  ('vorstand', 'accounting.read'), ('vorstand', 'billing.read'),
  ('vorstand', 'billing.approve'), ('vorstand', 'documents.read'),
  ('vorstand', 'documents.write'), ('vorstand', 'audit.read'),
  ('vorstand', 'applicants.read'), ('vorstand', 'applicants.write'),
  ('vorstand', 'applicants.assign'), ('vorstand', 'groups.read'),
  ('vorstand', 'groups.write'), ('vorstand', 'sharing.read'),
  ('vorstand', 'sharing.write'), ('vorstand', 'metering.read'),
  ('vorstand', 'operations.read'), ('vorstand', 'lending.read'),
  ('vorstand', 'lending.write'), ('vorstand', 'workhours.read'),
  ('vorstand', 'workhours.write'), ('vorstand', 'workhours.plan'),
  ('vorstand', 'compliance.read'), ('vorstand', 'compliance.write'),
  ('vorstand', 'compliance.approve'), ('vorstand', 'webhooks.read'),
  ('vorstand', 'webhooks.write'), ('vorstand', 'admin.jobs'),
  ('vorstand', 'meta.read');

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('kassierer', 'users.read'), ('kassierer', 'units.read'),
  ('kassierer', 'leases.read'), ('kassierer', 'accounting.read'),
  ('kassierer', 'accounting.write'), ('kassierer', 'accounting.lock'),
  ('kassierer', 'billing.read'), ('kassierer', 'billing.write'),
  ('kassierer', 'billing.approve'), ('kassierer', 'banking.import'),
  ('kassierer', 'banking.rules'), ('kassierer', 'documents.read'),
  ('kassierer', 'documents.write'), ('kassierer', 'operations.read'),
  ('kassierer', 'operations.write'), ('kassierer', 'metering.read'),
  ('kassierer', 'metering.write'), ('kassierer', 'workhours.read'),
  ('kassierer', 'compliance.read'), ('kassierer', 'groups.read');

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('mitglied', 'units.read'), ('mitglied', 'leases.read'),
  ('mitglied', 'documents.read'), ('mitglied', 'metering.read'),
  ('mitglied', 'lending.read'), ('mitglied', 'workhours.read'),
  ('mitglied', 'groups.read');

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('pruefer', 'accounting.read'), ('pruefer', 'billing.read'),
  ('pruefer', 'audit.read'), ('pruefer', 'banking.import'),
  ('pruefer', 'documents.read'), ('pruefer', 'operations.read'),
  ('pruefer', 'workhours.read'), ('pruefer', 'compliance.read'),
  ('pruefer', 'groups.read');
`
