BEGIN;

-- =============================================================================
-- 0) EXTENSIONS
-- =============================================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- =============================================================================
-- 1) GLOBAL HELPERS
-- =============================================================================

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

CREATE OR REPLACE FUNCTION jsonb_diff(old_json JSONB, new_json JSONB)
RETURNS JSONB AS $$
WITH keys AS (
  SELECT key FROM jsonb_object_keys(COALESCE(old_json,'{}'::jsonb)) key
  UNION
  SELECT key FROM jsonb_object_keys(COALESCE(new_json,'{}'::jsonb)) key
)
SELECT COALESCE(
  jsonb_object_agg(
    k.key,
    jsonb_build_object('old', old_json -> k.key, 'new', new_json -> k.key)
  )
  FILTER (WHERE (old_json -> k.key) IS DISTINCT FROM (new_json -> k.key)),
  '{}'::jsonb
)
FROM keys k;
$$ LANGUAGE sql;

-- =============================================================================
-- 2) IDENTITY & RBAC
-- =============================================================================

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

-- ZFA (Zwei-Faktor-Authentifizierung)
CREATE TABLE user_totp_secrets (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  secret_encrypted TEXT NOT NULL,
  is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  verified_at TIMESTAMPTZ,
  recovery_codes_hash TEXT[],
  last_used_at TIMESTAMPTZ,
  failed_attempts INT NOT NULL DEFAULT 0,
  locked_until TIMESTAMPTZ,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER tr_user_totp_updated_at
BEFORE UPDATE ON user_totp_secrets FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- =============================================================================
-- 2b) CRM
-- =============================================================================

CREATE TABLE user_address (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

  company TEXT,
  care_of TEXT,

  street1 TEXT NOT NULL,
  street2 TEXT,
  postal_code TEXT NOT NULL,
  city TEXT NOT NULL,
  state TEXT,
  country_code CHAR(2) NOT NULL CHECK (country_code ~ '^[A-Z]{2}$'),

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE TRIGGER tr_user_address_updated_at
BEFORE UPDATE ON user_address
FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_user_address_block_delete
BEFORE DELETE ON user_address
FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE user_contact_points (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  contact_type TEXT NOT NULL
    CHECK (contact_type IN ('email','phone','mobile','fax','website','other')),

  label TEXT,
  value TEXT NOT NULL,

  is_primary BOOLEAN NOT NULL DEFAULT FALSE,
  is_preferred BOOLEAN NOT NULL DEFAULT FALSE,
  allow_contact BOOLEAN NOT NULL DEFAULT TRUE,

  preferred_time_window TEXT,
  verified_at TIMESTAMPTZ,
  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_user_contact_points_user_id ON user_contact_points(user_id);
CREATE INDEX ix_user_contact_points_type ON user_contact_points(contact_type);
CREATE INDEX ix_user_contact_points_deleted_at ON user_contact_points(deleted_at);

CREATE UNIQUE INDEX ux_user_contact_points_primary_per_type_active
  ON user_contact_points(user_id, contact_type)
  WHERE deleted_at IS NULL AND is_primary = TRUE;

CREATE TRIGGER tr_user_contact_points_updated_at
BEFORE UPDATE ON user_contact_points
FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_user_contact_points_block_delete
BEFORE DELETE ON user_contact_points
FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE user_preferences (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

  preferred_contact_type TEXT
    CHECK (preferred_contact_type IN ('email','phone','mobile','postal','other')),

  note TEXT,

  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE TRIGGER tr_user_preferences_updated_at
BEFORE UPDATE ON user_preferences
FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- =============================================================================
-- 2c) GROUPS (generische Gruppen: Mannschaften, Abteilungen, AGs)
-- =============================================================================

CREATE TABLE groups (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  name TEXT NOT NULL,
  description TEXT,
  group_type TEXT NOT NULL DEFAULT 'sonstige'
    CHECK (group_type IN ('mannschaft','abteilung','arbeitsgruppe','vorstand','ausschuss','sonstige')),

  is_active BOOLEAN NOT NULL DEFAULT TRUE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_groups_type ON groups(group_type);
CREATE INDEX ix_groups_deleted_at ON groups(deleted_at);

CREATE TRIGGER tr_groups_updated_at
BEFORE UPDATE ON groups FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_groups_block_delete
BEFORE DELETE ON groups FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE group_members (
  group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

  role_in_group TEXT NOT NULL DEFAULT 'mitglied'
    CHECK (role_in_group IN ('leiter','stellvertreter','trainer','mitglied')),

  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  left_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,

  PRIMARY KEY (group_id, user_id)
);

CREATE INDEX ix_group_members_user ON group_members(user_id);
CREATE INDEX ix_group_members_group ON group_members(group_id);

-- =============================================================================
-- 3) ABSTRACT UNITS
-- =============================================================================

CREATE TABLE units (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  unit_type TEXT NOT NULL
    CHECK (unit_type IN ('garage','parzelle','stellplatz','other')),

  label TEXT NOT NULL,
  note TEXT,

  is_protected BOOLEAN NOT NULL DEFAULT FALSE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_units_unit_type ON units(unit_type);
CREATE INDEX ix_units_deleted_at ON units(deleted_at);

CREATE TRIGGER tr_units_updated_at
BEFORE UPDATE ON units FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_units_block_delete
BEFORE DELETE ON units FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


-- Typ-Tabelle: GARAGE
CREATE TABLE garages (
  unit_id UUID PRIMARY KEY REFERENCES units(id) ON DELETE RESTRICT,

  nr TEXT NOT NULL,
  area_sqm NUMERIC(10,2) NOT NULL CHECK (area_sqm > 0),
  meter_id TEXT,

  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX ux_garages_nr_active
  ON garages (nr)
  WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_garages_meter_id_active
  ON garages (meter_id)
  WHERE deleted_at IS NULL AND meter_id IS NOT NULL;

CREATE INDEX ix_garages_deleted_at ON garages(deleted_at);


-- Typ-Tabelle: PARZELLE (Kleingarten)
CREATE TABLE parcels (
  unit_id UUID PRIMARY KEY REFERENCES units(id) ON DELETE RESTRICT,

  nr TEXT NOT NULL,
  garden_area_sqm NUMERIC(10,2) CHECK (garden_area_sqm >= 0),
  building_area_sqm NUMERIC(10,2) CHECK (building_area_sqm >= 0),
  community_area_sqm NUMERIC(10,2) CHECK (community_area_sqm >= 0),
  total_area_sqm NUMERIC(10,2) GENERATED ALWAYS AS (
    COALESCE(garden_area_sqm, 0) + COALESCE(building_area_sqm, 0) + COALESCE(community_area_sqm, 0)
  ) STORED,

  building_value NUMERIC(12,2) CHECK (building_value >= 0),
  has_building_permit BOOLEAN NOT NULL DEFAULT FALSE,

  meter_electricity_id TEXT,
  meter_water_id TEXT,

  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX ux_parcels_nr_active
  ON parcels (nr)
  WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_parcels_meter_electricity_active
  ON parcels (meter_electricity_id)
  WHERE deleted_at IS NULL AND meter_electricity_id IS NOT NULL;

CREATE UNIQUE INDEX ux_parcels_meter_water_active
  ON parcels (meter_water_id)
  WHERE deleted_at IS NULL AND meter_water_id IS NOT NULL;

CREATE INDEX ix_parcels_deleted_at ON parcels(deleted_at);

-- =============================================================================
-- 4) EIGENTUM & VERMIETUNG
-- =============================================================================

CREATE TABLE unit_ownerships (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  unit_id UUID NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
  owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  start_date DATE NOT NULL,
  end_date DATE,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,

  CHECK (end_date IS NULL OR end_date >= start_date)
);

CREATE INDEX ix_unit_ownerships_unit_id ON unit_ownerships(unit_id);
CREATE INDEX ix_unit_ownerships_owner_id ON unit_ownerships(owner_user_id);
CREATE INDEX ix_unit_ownerships_deleted_at ON unit_ownerships(deleted_at);

CREATE TRIGGER tr_unit_ownerships_updated_at
BEFORE UPDATE ON unit_ownerships FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_unit_ownerships_block_delete
BEFORE DELETE ON unit_ownerships FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

ALTER TABLE unit_ownerships
ADD CONSTRAINT ex_unit_ownerships_no_overlap
EXCLUDE USING gist (
  unit_id WITH =,
  daterange(start_date, COALESCE(end_date, 'infinity'::date), '[]') WITH &&
)
WHERE (deleted_at IS NULL);


CREATE TABLE leases (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  unit_id UUID NOT NULL REFERENCES units(id) ON DELETE RESTRICT,

  tenant_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  landlord_user_id UUID REFERENCES users(id) ON DELETE SET NULL,

  start_date DATE NOT NULL,
  end_date DATE,

  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('planned','active','ended')),

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,

  CHECK (end_date IS NULL OR end_date >= start_date)
);

CREATE INDEX ix_leases_unit_id ON leases(unit_id);
CREATE INDEX ix_leases_tenant_user_id ON leases(tenant_user_id);
CREATE INDEX ix_leases_landlord_user_id ON leases(landlord_user_id);
CREATE INDEX ix_leases_deleted_at ON leases(deleted_at);

CREATE TRIGGER tr_leases_updated_at
BEFORE UPDATE ON leases FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_leases_block_delete
BEFORE DELETE ON leases FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

ALTER TABLE leases
ADD CONSTRAINT ex_leases_no_overlap
EXCLUDE USING gist (
  unit_id WITH =,
  daterange(start_date, COALESCE(end_date, 'infinity'::date), '[]') WITH &&
)
WHERE (deleted_at IS NULL);

-- =============================================================================
-- 4b) BEWERBERMANAGER
-- =============================================================================

CREATE TABLE applicants (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  applied_at DATE NOT NULL DEFAULT CURRENT_DATE,
  preferred_unit_type TEXT
    CHECK (preferred_unit_type IS NULL OR preferred_unit_type IN ('garage','parzelle','stellplatz','other')),
  preferred_area_min NUMERIC(10,2) CHECK (preferred_area_min IS NULL OR preferred_area_min >= 0),

  status TEXT NOT NULL DEFAULT 'waiting'
    CHECK (status IN ('waiting','offered','accepted','rejected','withdrawn')),

  waitlist_position INT,
  assigned_unit_id UUID REFERENCES units(id) ON DELETE SET NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_applicants_user_id ON applicants(user_id);
CREATE INDEX ix_applicants_status ON applicants(status);
CREATE INDEX ix_applicants_deleted_at ON applicants(deleted_at);

CREATE TRIGGER tr_applicants_updated_at
BEFORE UPDATE ON applicants FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_applicants_block_delete
BEFORE DELETE ON applicants FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

-- =============================================================================
-- 5) DOCUMENTS
-- =============================================================================

CREATE TABLE documents (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  title TEXT NOT NULL,
  file_path TEXT NOT NULL,
  mime_type TEXT,
  file_size_bytes BIGINT CHECK (file_size_bytes IS NULL OR file_size_bytes >= 0),
  checksum_sha256 TEXT,

  is_public BOOLEAN NOT NULL DEFAULT FALSE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_documents_deleted_at ON documents(deleted_at);

CREATE TRIGGER tr_documents_updated_at
BEFORE UPDATE ON documents FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_documents_block_delete
BEFORE DELETE ON documents FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE document_categories (
  slug TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  description TEXT,
  sort_order INT NOT NULL DEFAULT 0,
  is_protected BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE document_relations (
  document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  category_slug TEXT NOT NULL REFERENCES document_categories(slug) ON DELETE RESTRICT,

  target_type TEXT NOT NULL
    CHECK (target_type IN (
      'user','unit','lease','ownership','journal','expense_claim',
      'deposit','meter_reading','inventory_item',
      'applicant','billing_period','bank_transaction','shared_link',
      'lendable_item','lending_record',
      'incident','warning','dunning_notice','termination',
      'work_event','legal_provision'
    )),

  target_pk TEXT NOT NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,

  PRIMARY KEY (document_id, category_slug, target_type, target_pk)
);

CREATE INDEX ix_document_relations_target ON document_relations(target_type, target_pk);
CREATE INDEX ix_document_relations_category ON document_relations(category_slug);
CREATE INDEX ix_document_relations_document ON document_relations(document_id);

-- =============================================================================
-- 6) ZAEHLERSTAENDE
-- =============================================================================

CREATE TABLE meter_readings (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  unit_id UUID NOT NULL REFERENCES units(id) ON DELETE RESTRICT,

  reading_date DATE NOT NULL,
  val_kwh NUMERIC(15,3) NOT NULL CHECK (val_kwh >= 0),

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_meter_readings_unit_date_active
  ON meter_readings(unit_id, reading_date)
  WHERE deleted_at IS NULL;

CREATE INDEX ix_meter_readings_unit_id ON meter_readings(unit_id);
CREATE INDEX ix_meter_readings_reading_date ON meter_readings(reading_date);
CREATE INDEX ix_meter_readings_deleted_at ON meter_readings(deleted_at);

CREATE TRIGGER tr_meter_readings_updated_at
BEFORE UPDATE ON meter_readings FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_meter_readings_block_delete
BEFORE DELETE ON meter_readings FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

-- =============================================================================
-- 7) ACCOUNTING
-- =============================================================================

CREATE TABLE accounting_accounts (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  account_type TEXT NOT NULL
    CHECK (account_type IN ('asset','liability','equity','income','expense'))
);

CREATE TABLE accounting_journal (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  transaction_date DATE NOT NULL DEFAULT CURRENT_DATE,
  description TEXT NOT NULL,

  reference_type TEXT
    CHECK (reference_type IS NULL OR reference_type IN (
      'unit','lease','ownership','deposit','expense_claim',
      'meter_reading','inventory_item','document',
      'applicant','billing_period','billing_item','bank_transaction',
      'lendable_item','lending_record',
      'dunning_notice','work_event','warning','termination'
    )),
  reference_pk TEXT,

  is_locked BOOLEAN NOT NULL DEFAULT FALSE,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX ix_journal_transaction_date ON accounting_journal(transaction_date);
CREATE INDEX ix_journal_deleted_at ON accounting_journal(deleted_at);

CREATE TRIGGER tr_journal_updated_at
BEFORE UPDATE ON accounting_journal FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_journal_block_delete
BEFORE DELETE ON accounting_journal FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE accounting_entries (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  journal_id UUID NOT NULL REFERENCES accounting_journal(id) ON DELETE RESTRICT,
  account_code TEXT NOT NULL REFERENCES accounting_accounts(code) ON DELETE RESTRICT,

  debit NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (debit >= 0),
  credit NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (credit >= 0),

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,

  CHECK (
    (debit > 0 AND credit = 0) OR
    (credit > 0 AND debit = 0)
  )
);

CREATE INDEX ix_entries_journal_id ON accounting_entries(journal_id);
CREATE INDEX ix_entries_account_code ON accounting_entries(account_code);
CREATE INDEX ix_entries_deleted_at ON accounting_entries(deleted_at);

CREATE TRIGGER tr_entries_updated_at
BEFORE UPDATE ON accounting_entries FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_entries_block_delete
BEFORE DELETE ON accounting_entries FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE OR REPLACE FUNCTION accounting_enforce_journal_lock_rules()
RETURNS TRIGGER AS $$
DECLARE
  s_debit NUMERIC(18,2);
  s_credit NUMERIC(18,2);
BEGIN
  IF COALESCE(OLD.is_locked,false) = false AND COALESCE(NEW.is_locked,false) = true THEN
    SELECT COALESCE(SUM(debit),0), COALESCE(SUM(credit),0)
      INTO s_debit, s_credit
    FROM accounting_entries
    WHERE journal_id = NEW.id AND deleted_at IS NULL;

    IF s_debit <> s_credit THEN
      RAISE EXCEPTION 'Journal kann nicht gelocked werden: Debit (%) != Credit (%).', s_debit, s_credit;
    END IF;
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_journal_balance_on_lock
BEFORE UPDATE OF is_locked ON accounting_journal
FOR EACH ROW EXECUTE PROCEDURE accounting_enforce_journal_lock_rules();


CREATE OR REPLACE FUNCTION accounting_prevent_entry_change_when_locked()
RETURNS TRIGGER AS $$
DECLARE
  locked BOOLEAN;
  j_id UUID;
BEGIN
  j_id := COALESCE(NEW.journal_id, OLD.journal_id);
  SELECT is_locked INTO locked FROM accounting_journal WHERE id = j_id;

  IF locked THEN
    RAISE EXCEPTION 'Journal ist locked; Entries duerfen nicht geaendert/geloescht/neu angelegt werden.';
  END IF;

  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_entries_no_insert_when_locked
BEFORE INSERT ON accounting_entries
FOR EACH ROW EXECUTE PROCEDURE accounting_prevent_entry_change_when_locked();

CREATE TRIGGER tr_entries_no_update_when_locked
BEFORE UPDATE ON accounting_entries
FOR EACH ROW EXECUTE PROCEDURE accounting_prevent_entry_change_when_locked();

CREATE TRIGGER tr_entries_no_delete_when_locked
BEFORE DELETE ON accounting_entries
FOR EACH ROW EXECUTE PROCEDURE accounting_prevent_entry_change_when_locked();

-- =============================================================================
-- 7b) BILLING (Pachtrechnungen / Jahresabrechnungen)
-- =============================================================================

CREATE TABLE billing_periods (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  year INT NOT NULL CHECK (year BETWEEN 2000 AND 2100),
  label TEXT NOT NULL,

  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','calculated','approved','sent','archived')),

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_billing_periods_year ON billing_periods(year);
CREATE INDEX ix_billing_periods_status ON billing_periods(status);
CREATE INDEX ix_billing_periods_deleted_at ON billing_periods(deleted_at);

CREATE TRIGGER tr_billing_periods_updated_at
BEFORE UPDATE ON billing_periods FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_billing_periods_block_delete
BEFORE DELETE ON billing_periods FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE billing_items (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  billing_period_id UUID NOT NULL REFERENCES billing_periods(id) ON DELETE RESTRICT,
  lease_id UUID NOT NULL REFERENCES leases(id) ON DELETE RESTRICT,
  unit_id UUID NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
  tenant_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  item_type TEXT NOT NULL
    CHECK (item_type IN ('pacht','strom','wasser','umlage',
                         'arbeitsstunden_ersatz','mahngebuehr','leihgebuehr','sonstige')),

  description TEXT,
  quantity NUMERIC(15,3),
  unit_price NUMERIC(12,4),
  amount NUMERIC(12,2) NOT NULL,

  journal_id UUID REFERENCES accounting_journal(id) ON DELETE SET NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_billing_items_period ON billing_items(billing_period_id);
CREATE INDEX ix_billing_items_lease ON billing_items(lease_id);
CREATE INDEX ix_billing_items_unit ON billing_items(unit_id);
CREATE INDEX ix_billing_items_tenant ON billing_items(tenant_user_id);
CREATE INDEX ix_billing_items_deleted_at ON billing_items(deleted_at);

CREATE TRIGGER tr_billing_items_updated_at
BEFORE UPDATE ON billing_items FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_billing_items_block_delete
BEFORE DELETE ON billing_items FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE billing_snapshots (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  billing_period_id UUID NOT NULL REFERENCES billing_periods(id) ON DELETE RESTRICT,
  snapshot_data JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_billing_snapshots_period ON billing_snapshots(billing_period_id);

-- =============================================================================
-- 8) INVENTORY, DEPOSITS, WORK LOGS, EXPENSE CLAIMS
-- =============================================================================

CREATE TABLE inventory_items (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  unit_id UUID NOT NULL REFERENCES units(id) ON DELETE RESTRICT,

  name TEXT NOT NULL,
  description TEXT,
  quantity NUMERIC(12,3) NOT NULL DEFAULT 1 CHECK (quantity > 0),

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_inventory_unit_id ON inventory_items(unit_id);
CREATE INDEX ix_inventory_deleted_at ON inventory_items(deleted_at);

CREATE TRIGGER tr_inventory_updated_at
BEFORE UPDATE ON inventory_items FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_inventory_block_delete
BEFORE DELETE ON inventory_items FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE deposits (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  lease_id UUID NOT NULL REFERENCES leases(id) ON DELETE RESTRICT,

  amount NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
  currency CHAR(3) NOT NULL DEFAULT 'EUR' CHECK (currency ~ '^[A-Z]{3}$'),

  status TEXT NOT NULL DEFAULT 'held'
    CHECK (status IN ('held','returned','offset','cancelled')),

  journal_id UUID REFERENCES accounting_journal(id) ON DELETE SET NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_deposits_lease_id ON deposits(lease_id);
CREATE INDEX ix_deposits_deleted_at ON deposits(deleted_at);

CREATE TRIGGER tr_deposits_updated_at
BEFORE UPDATE ON deposits FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_deposits_block_delete
BEFORE DELETE ON deposits FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE work_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  date_performed DATE NOT NULL,
  hours_spent NUMERIC(5,2) NOT NULL CHECK (hours_spent > 0 AND hours_spent <= 24),
  applied_to_year INT NOT NULL CHECK (applied_to_year BETWEEN 2000 AND 2100),

  is_verified BOOLEAN NOT NULL DEFAULT FALSE,
  verified_by UUID REFERENCES users(id) ON DELETE SET NULL,
  verified_at TIMESTAMPTZ,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_work_logs_user_id ON work_logs(user_id);
CREATE INDEX ix_work_logs_year ON work_logs(applied_to_year);
CREATE INDEX ix_work_logs_date ON work_logs(date_performed);
CREATE INDEX ix_work_logs_deleted_at ON work_logs(deleted_at);

CREATE TRIGGER tr_work_logs_updated_at
BEFORE UPDATE ON work_logs FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_work_logs_block_delete
BEFORE DELETE ON work_logs FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE expense_claims (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  amount NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
  currency CHAR(3) NOT NULL DEFAULT 'EUR' CHECK (currency ~ '^[A-Z]{3}$'),

  status TEXT NOT NULL DEFAULT 'submitted'
    CHECK (status IN ('submitted','approved','rejected','paid','cancelled')),

  receipt_document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
  journal_id UUID REFERENCES accounting_journal(id) ON DELETE SET NULL,

  description TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_expense_claims_user_id ON expense_claims(user_id);
CREATE INDEX ix_expense_claims_status ON expense_claims(status);
CREATE INDEX ix_expense_claims_deleted_at ON expense_claims(deleted_at);

CREATE TRIGGER tr_expense_claims_updated_at
BEFORE UPDATE ON expense_claims FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_expense_claims_block_delete
BEFORE DELETE ON expense_claims FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

-- =============================================================================
-- 8b) LENDING (Geraeteverleih)
-- =============================================================================

CREATE TABLE lendable_items (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  name TEXT NOT NULL,
  description TEXT,
  category TEXT NOT NULL DEFAULT 'sonstige'
    CHECK (category IN ('werkzeug','garten','feier','sport','fahrzeug','sonstige')),

  item_mode TEXT NOT NULL DEFAULT 'trackable'
    CHECK (item_mode IN ('trackable', 'bulk')),

  serial_number TEXT,
  quantity_total INT NOT NULL DEFAULT 1 CHECK (quantity_total > 0),
  stock_total INT,
  stock_available INT,

  condition TEXT DEFAULT 'gut'
    CHECK (condition IN ('neu','gut','gebraucht','reparaturbeduerftig','defekt')),

  daily_fee NUMERIC(8,2) NOT NULL DEFAULT 0 CHECK (daily_fee >= 0),
  deposit_amount NUMERIC(8,2) NOT NULL DEFAULT 0 CHECK (deposit_amount >= 0),

  image_document_id UUID REFERENCES documents(id) ON DELETE SET NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_lendable_items_category ON lendable_items(category);
CREATE INDEX ix_lendable_items_deleted_at ON lendable_items(deleted_at);

CREATE TRIGGER tr_lendable_items_updated_at
BEFORE UPDATE ON lendable_items FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_lendable_items_block_delete
BEFORE DELETE ON lendable_items FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE lending_records (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  item_id UUID NOT NULL REFERENCES lendable_items(id) ON DELETE RESTRICT,
  borrower_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
  borrower_group_id UUID REFERENCES groups(id) ON DELETE RESTRICT,
  quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0),

  status TEXT NOT NULL DEFAULT 'reserved'
    CHECK (status IN ('reserved','checked_out','returned','overdue','cancelled')),

  reserved_from DATE NOT NULL,
  reserved_until DATE NOT NULL,
  checked_out_at TIMESTAMPTZ,
  returned_at TIMESTAMPTZ,

  condition_on_checkout TEXT,
  condition_on_return TEXT,

  fee_charged NUMERIC(8,2) NOT NULL DEFAULT 0 CHECK (fee_charged >= 0),
  journal_id UUID REFERENCES accounting_journal(id) ON DELETE SET NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,

  CHECK (reserved_until >= reserved_from),
  CHECK (
    (borrower_user_id IS NOT NULL AND borrower_group_id IS NULL) OR
    (borrower_user_id IS NULL AND borrower_group_id IS NOT NULL)
  )
);

CREATE INDEX ix_lending_records_item ON lending_records(item_id);
CREATE INDEX ix_lending_records_borrower ON lending_records(borrower_user_id);
CREATE INDEX ix_lending_records_borrower_group ON lending_records(borrower_group_id);
CREATE INDEX ix_lending_records_status ON lending_records(status);
CREATE INDEX ix_lending_records_dates ON lending_records(reserved_from, reserved_until);
CREATE INDEX ix_lending_records_deleted_at ON lending_records(deleted_at);

CREATE TRIGGER tr_lending_records_updated_at
BEFORE UPDATE ON lending_records FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_lending_records_block_delete
BEFORE DELETE ON lending_records FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

-- =============================================================================
-- 9) BANKING (Bankdatenimport)
-- =============================================================================

CREATE TABLE bank_accounts (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  iban_masked TEXT NOT NULL,
  name TEXT,
  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_bank_accounts_iban_active
  ON bank_accounts (iban_masked)
  WHERE deleted_at IS NULL;

CREATE INDEX ix_bank_accounts_deleted_at ON bank_accounts(deleted_at);

CREATE TRIGGER tr_bank_accounts_updated_at
BEFORE UPDATE ON bank_accounts FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_bank_accounts_block_delete
BEFORE DELETE ON bank_accounts FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE bank_transactions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  bank_account_id UUID NOT NULL REFERENCES bank_accounts(id) ON DELETE RESTRICT,

  transaction_date DATE NOT NULL,
  amount NUMERIC(12,2) NOT NULL,
  payee TEXT,
  description TEXT,
  source TEXT NOT NULL DEFAULT 'csv-import',

  checksum_sha256 TEXT,

  category_account_code TEXT REFERENCES accounting_accounts(code) ON DELETE SET NULL,
  journal_id UUID REFERENCES accounting_journal(id) ON DELETE SET NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_bank_tx_account ON bank_transactions(bank_account_id);
CREATE INDEX ix_bank_tx_date ON bank_transactions(transaction_date);
CREATE INDEX ix_bank_tx_checksum ON bank_transactions(checksum_sha256);
CREATE INDEX ix_bank_tx_deleted_at ON bank_transactions(deleted_at);

CREATE TRIGGER tr_bank_transactions_updated_at
BEFORE UPDATE ON bank_transactions FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_bank_transactions_block_delete
BEFORE DELETE ON bank_transactions FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE bank_categorization_rules (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  pattern TEXT NOT NULL,
  field TEXT NOT NULL DEFAULT 'description'
    CHECK (field IN ('description','payee')),
  category_account_code TEXT NOT NULL REFERENCES accounting_accounts(code) ON DELETE CASCADE,
  priority INT NOT NULL DEFAULT 10,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_bank_rules_deleted_at ON bank_categorization_rules(deleted_at);

CREATE TRIGGER tr_bank_rules_updated_at
BEFORE UPDATE ON bank_categorization_rules FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_bank_rules_block_delete
BEFORE DELETE ON bank_categorization_rules FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE bank_import_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  filename TEXT NOT NULL,
  format TEXT NOT NULL,
  detected_iban TEXT,

  total_rows INT NOT NULL DEFAULT 0,
  imported_rows INT NOT NULL DEFAULT 0,
  skipped_duplicates INT NOT NULL DEFAULT 0,

  bank_account_id UUID REFERENCES bank_accounts(id) ON DELETE SET NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_bank_import_logs_created_at ON bank_import_logs(created_at);

-- =============================================================================
-- 10) SHARED LINKS & QR-CODES
-- =============================================================================

CREATE TABLE shared_links (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  slug TEXT NOT NULL,

  target_type TEXT NOT NULL
    CHECK (target_type IN (
      'user','unit','lease','ownership','document','applicant',
      'billing_period','event','page','custom'
    )),
  target_pk TEXT,
  custom_url TEXT,

  is_public BOOLEAN NOT NULL DEFAULT FALSE,

  expires_at TIMESTAMPTZ,
  max_visits INT,
  visit_count INT NOT NULL DEFAULT 0,
  pin_code TEXT,

  context TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX ux_shared_links_slug_active
  ON shared_links (slug)
  WHERE deleted_at IS NULL;

CREATE INDEX ix_shared_links_target ON shared_links(target_type, target_pk);
CREATE INDEX ix_shared_links_deleted_at ON shared_links(deleted_at);

CREATE TRIGGER tr_shared_links_block_delete
BEFORE DELETE ON shared_links FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

-- =============================================================================
-- 10b) WEBHOOK SUBSCRIPTIONS
-- =============================================================================

CREATE TABLE webhook_subscriptions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  hook_name TEXT NOT NULL,
  target_url TEXT NOT NULL,
  secret TEXT,

  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  retry_count INT NOT NULL DEFAULT 3,
  timeout_ms INT NOT NULL DEFAULT 5000,

  last_triggered_at TIMESTAMPTZ,
  last_status_code INT,
  failure_count INT NOT NULL DEFAULT 0,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_webhooks_hook_name ON webhook_subscriptions(hook_name) WHERE deleted_at IS NULL AND is_active = TRUE;
CREATE INDEX ix_webhooks_deleted_at ON webhook_subscriptions(deleted_at);

CREATE TRIGGER tr_webhooks_updated_at
BEFORE UPDATE ON webhook_subscriptions FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_webhooks_block_delete
BEFORE DELETE ON webhook_subscriptions FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();

-- =============================================================================
-- 10c) WORKHOURS (Pflichtstunden + Einsatzplaner)
-- =============================================================================

CREATE TABLE work_hour_requirements (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  year INT NOT NULL UNIQUE,
  base_hours NUMERIC(5,2) NOT NULL,
  fee_per_missing_hour NUMERIC(8,2) NOT NULL,
  deadline DATE NOT NULL,
  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE TRIGGER tr_wh_requirements_updated_at
BEFORE UPDATE ON work_hour_requirements FOR EACH ROW EXECUTE PROCEDURE set_updated_at();


CREATE TABLE unit_work_hour_obligations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  unit_id UUID NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
  year INT NOT NULL,
  additional_hours NUMERIC(5,2) NOT NULL CHECK (additional_hours > 0),
  reason TEXT NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,

  UNIQUE(unit_id, year)
);

CREATE INDEX ix_unit_wh_obligations_unit ON unit_work_hour_obligations(unit_id);
CREATE INDEX ix_unit_wh_obligations_deleted_at ON unit_work_hour_obligations(deleted_at);

CREATE TRIGGER tr_unit_wh_obligations_updated_at
BEFORE UPDATE ON unit_work_hour_obligations FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_unit_wh_obligations_block_delete
BEFORE DELETE ON unit_work_hour_obligations FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE user_work_hour_adjustments (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  year INT NOT NULL,
  adjusted_hours NUMERIC(5,2) NOT NULL,
  reason TEXT NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,

  UNIQUE(user_id, year)
);

CREATE INDEX ix_user_wh_adjustments_user ON user_work_hour_adjustments(user_id);
CREATE INDEX ix_user_wh_adjustments_deleted_at ON user_work_hour_adjustments(deleted_at);

CREATE TRIGGER tr_user_wh_adjustments_updated_at
BEFORE UPDATE ON user_work_hour_adjustments FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_user_wh_adjustments_block_delete
BEFORE DELETE ON user_work_hour_adjustments FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE work_task_catalog (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  title TEXT NOT NULL,
  description TEXT,
  category TEXT NOT NULL DEFAULT 'sonstige'
    CHECK (category IN ('garten','wege','gebaeude','infrastruktur',
                        'verwaltung','veranstaltung','sonstige')),
  typical_participants INT NOT NULL DEFAULT 1 CHECK (typical_participants > 0),
  typical_hours NUMERIC(5,2) NOT NULL DEFAULT 2 CHECK (typical_hours > 0),
  is_seasonal BOOLEAN NOT NULL DEFAULT FALSE,
  season_months INT[],
  recurrence_interval_days INT CHECK (recurrence_interval_days IS NULL OR recurrence_interval_days > 0),
  last_performed_at DATE,
  unit_id UUID REFERENCES units(id) ON DELETE SET NULL,
  requires_equipment TEXT[],
  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_work_task_catalog_category ON work_task_catalog(category);
CREATE INDEX ix_work_task_catalog_deleted_at ON work_task_catalog(deleted_at);

CREATE TRIGGER tr_work_task_catalog_updated_at
BEFORE UPDATE ON work_task_catalog FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_work_task_catalog_block_delete
BEFORE DELETE ON work_task_catalog FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE work_events (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  title TEXT NOT NULL,
  description TEXT,
  event_date DATE NOT NULL,
  start_time TIME,
  end_time TIME,
  location TEXT,
  min_participants INT,
  max_participants INT,
  hours_credited NUMERIC(5,2) NOT NULL DEFAULT 2 CHECK (hours_credited > 0),
  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','planned','invitation_sent',
                      'in_progress','completed','cancelled')),
  cancellation_reason TEXT,

  ai_suggested_date DATE,
  ai_suggestion_reason TEXT,
  ai_participant_suggestion JSONB,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_work_events_date ON work_events(event_date);
CREATE INDEX ix_work_events_status ON work_events(status);
CREATE INDEX ix_work_events_deleted_at ON work_events(deleted_at);

CREATE TRIGGER tr_work_events_updated_at
BEFORE UPDATE ON work_events FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_work_events_block_delete
BEFORE DELETE ON work_events FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE work_event_tasks (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  work_event_id UUID NOT NULL REFERENCES work_events(id) ON DELETE CASCADE,
  catalog_task_id UUID REFERENCES work_task_catalog(id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  required_participants INT NOT NULL DEFAULT 1 CHECK (required_participants > 0),
  estimated_hours NUMERIC(5,2),
  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_work_event_tasks_event ON work_event_tasks(work_event_id);


CREATE TABLE work_event_participants (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  work_event_id UUID NOT NULL REFERENCES work_events(id) ON DELETE RESTRICT,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  assigned_task_id UUID REFERENCES work_event_tasks(id) ON DELETE SET NULL,

  status TEXT NOT NULL DEFAULT 'invited'
    CHECK (status IN ('invited','accepted','declined',
                      'attended','no_show','excused')),

  invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  responded_at TIMESTAMPTZ,
  attended_from TIME,
  attended_until TIME,
  hours_credited NUMERIC(5,2),

  work_log_id UUID REFERENCES work_logs(id) ON DELETE SET NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(work_event_id, user_id)
);

CREATE INDEX ix_work_event_participants_event ON work_event_participants(work_event_id);
CREATE INDEX ix_work_event_participants_user ON work_event_participants(user_id);
CREATE INDEX ix_work_event_participants_status ON work_event_participants(status);

CREATE TRIGGER tr_work_event_participants_updated_at
BEFORE UPDATE ON work_event_participants FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- =============================================================================
-- 10d) COMPLIANCE (Verstoesse, Mahnungen, Abmahnungen, Kuendigungen)
-- =============================================================================

CREATE TABLE legal_provisions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  source TEXT NOT NULL
    CHECK (source IN ('vereinssatzung','gartenordnung','hausordnung',
                      'bundeskleingartengesetz','landesgesetz','sonstige')),
  reference TEXT NOT NULL,
  full_text TEXT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_legal_provisions_source ON legal_provisions(source);

CREATE TRIGGER tr_legal_provisions_updated_at
BEFORE UPDATE ON legal_provisions FOR EACH ROW EXECUTE PROCEDURE set_updated_at();


CREATE TABLE incidents (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  subject_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
  subject_unit_id UUID REFERENCES units(id) ON DELETE RESTRICT,

  title TEXT NOT NULL,
  description TEXT NOT NULL,
  category TEXT NOT NULL
    CHECK (category IN (
      'gartenordnung','bauvorschrift','laerm','fehlverhalten',
      'zahlungsverzug','arbeitsstunden','sachbeschaedigung',
      'umwelt','tierhaltung','sonstiges'
    )),
  severity TEXT NOT NULL DEFAULT 'minor'
    CHECK (severity IN ('minor','moderate','major','critical')),

  incident_date DATE NOT NULL,
  incident_time TIME,
  location_description TEXT,

  legal_provision_id UUID REFERENCES legal_provisions(id) ON DELETE SET NULL,
  legal_basis_reference TEXT,
  legal_basis_text TEXT,

  status TEXT NOT NULL DEFAULT 'reported'
    CHECK (status IN (
      'reported','investigating','confirmed',
      'action_taken','resolved','dismissed'
    )),
  resolution_note TEXT,

  ai_suggested_severity TEXT,
  ai_suggested_category TEXT,
  ai_suggested_provision_id UUID REFERENCES legal_provisions(id) ON DELETE SET NULL,
  ai_confidence NUMERIC(3,2) CHECK (ai_confidence IS NULL OR (ai_confidence >= 0 AND ai_confidence <= 1)),

  reported_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_incidents_subject_user ON incidents(subject_user_id);
CREATE INDEX ix_incidents_subject_unit ON incidents(subject_unit_id);
CREATE INDEX ix_incidents_category ON incidents(category);
CREATE INDEX ix_incidents_severity ON incidents(severity);
CREATE INDEX ix_incidents_status ON incidents(status);
CREATE INDEX ix_incidents_date ON incidents(incident_date);
CREATE INDEX ix_incidents_deleted_at ON incidents(deleted_at);

CREATE TRIGGER tr_incidents_updated_at
BEFORE UPDATE ON incidents FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_incidents_block_delete
BEFORE DELETE ON incidents FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE incident_witnesses (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  witness_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  witness_name TEXT,
  witness_contact TEXT,
  statement TEXT NOT NULL,
  statement_date DATE NOT NULL DEFAULT CURRENT_DATE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_incident_witnesses_incident ON incident_witnesses(incident_id);


CREATE TABLE dunning_notices (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  dunning_level INT NOT NULL DEFAULT 1
    CHECK (dunning_level BETWEEN 1 AND 3),
  reason TEXT NOT NULL,

  reference_type TEXT
    CHECK (reference_type IN ('billing_item','lease','work_hours','sonstige')),
  reference_pk TEXT,

  amount_due NUMERIC(12,2) NOT NULL CHECK (amount_due >= 0),
  dunning_fee NUMERIC(8,2) NOT NULL DEFAULT 0 CHECK (dunning_fee >= 0),
  due_date DATE NOT NULL,

  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','approved','issued','reminded','paid','escalated','cancelled')),

  approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
  approved_at TIMESTAMPTZ,

  document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
  journal_id UUID REFERENCES accounting_journal(id) ON DELETE SET NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_dunning_user ON dunning_notices(user_id);
CREATE INDEX ix_dunning_status ON dunning_notices(status);
CREATE INDEX ix_dunning_level ON dunning_notices(dunning_level);
CREATE INDEX ix_dunning_deleted_at ON dunning_notices(deleted_at);

CREATE TRIGGER tr_dunning_updated_at
BEFORE UPDATE ON dunning_notices FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_dunning_block_delete
BEFORE DELETE ON dunning_notices FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE warnings (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  warning_type TEXT NOT NULL
    CHECK (warning_type IN ('gartenordnung','fehlverhalten','arbeitsstunden',
                            'bauvorschrift','laerm','sonstiges')),
  severity TEXT NOT NULL DEFAULT 'formal'
    CHECK (severity IN ('informal','formal','final')),

  description TEXT NOT NULL,
  issued_date DATE NOT NULL,
  valid_until DATE,
  response_deadline DATE,
  response_received_at TIMESTAMPTZ,
  response_text TEXT,

  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','approved','issued','acknowledged',
                      'responded','expired','escalated','withdrawn')),

  approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
  approved_at TIMESTAMPTZ,

  legal_provision_id UUID REFERENCES legal_provisions(id) ON DELETE SET NULL,
  document_id UUID REFERENCES documents(id) ON DELETE SET NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_warnings_user ON warnings(user_id);
CREATE INDEX ix_warnings_status ON warnings(status);
CREATE INDEX ix_warnings_type ON warnings(warning_type);
CREATE INDEX ix_warnings_deleted_at ON warnings(deleted_at);

CREATE TRIGGER tr_warnings_updated_at
BEFORE UPDATE ON warnings FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_warnings_block_delete
BEFORE DELETE ON warnings FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE terminations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  lease_id UUID REFERENCES leases(id) ON DELETE SET NULL,

  termination_type TEXT NOT NULL
    CHECK (termination_type IN ('ordentlich','ausserordentlich','fristlos')),

  reason TEXT NOT NULL,
  legal_provision_id UUID REFERENCES legal_provisions(id) ON DELETE SET NULL,
  legal_basis_text TEXT,

  notice_date DATE NOT NULL,
  effective_date DATE NOT NULL,
  clearance_deadline DATE,
  objection_deadline DATE,
  objection_received_at TIMESTAMPTZ,
  objection_text TEXT,

  board_resolution_date DATE,

  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','approved','issued','acknowledged',
                      'contested','enforced','withdrawn','resolved')),

  approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
  approved_at TIMESTAMPTZ,

  document_id UUID REFERENCES documents(id) ON DELETE SET NULL,

  note TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX ix_terminations_user ON terminations(user_id);
CREATE INDEX ix_terminations_lease ON terminations(lease_id);
CREATE INDEX ix_terminations_status ON terminations(status);
CREATE INDEX ix_terminations_deleted_at ON terminations(deleted_at);

CREATE TRIGGER tr_terminations_updated_at
BEFORE UPDATE ON terminations FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER tr_terminations_block_delete
BEFORE DELETE ON terminations FOR EACH ROW EXECUTE PROCEDURE prevent_permanent_delete();


CREATE TABLE incident_consequences (
  incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  consequence_type TEXT NOT NULL
    CHECK (consequence_type IN ('warning','dunning_notice','termination')),
  consequence_pk UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  PRIMARY KEY (incident_id, consequence_type, consequence_pk)
);

CREATE INDEX ix_incident_consequences_incident ON incident_consequences(incident_id);

-- =============================================================================
-- 11) AUDIT + REVISION VIEW
-- =============================================================================

CREATE TABLE audit_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

  table_name TEXT NOT NULL,
  record_pk TEXT NOT NULL,

  action TEXT NOT NULL CHECK (action IN ('INSERT','UPDATE','DELETE','LOGIN','STATUS_CHANGE','ANONYMIZE')),

  old_data JSONB,
  new_data JSONB,

  actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_audit_logs_table_record ON audit_logs(table_name, record_pk);
CREATE INDEX ix_audit_logs_actor ON audit_logs(actor_user_id);
CREATE INDEX ix_audit_logs_created_at ON audit_logs(created_at);

CREATE OR REPLACE VIEW view_revision_report AS
SELECT
  al.created_at AS zeitpunkt,
  CASE
    WHEN u.id IS NULL THEN NULL
    WHEN u.anonymized_at IS NOT NULL THEN '[ANONYMISIERT]'
    ELSE u.last_name || ', ' || u.first_name
  END AS bearbeiter,
  al.table_name AS bereich,
  al.action AS aktion,
  al.record_pk AS datensatz_id,
  jsonb_diff(al.old_data, al.new_data) AS geaenderte_werte,
  al.old_data AS stand_vorher,
  al.new_data AS stand_nachher
FROM audit_logs al
LEFT JOIN users u ON al.actor_user_id = u.id
ORDER BY al.created_at DESC;

-- =============================================================================
-- 12) AUDIT TRIGGER (automatisch auf allen relevanten Tabellen)
-- =============================================================================

CREATE OR REPLACE FUNCTION audit_trigger_func()
RETURNS TRIGGER AS $$
DECLARE
  record_id TEXT;
  actor UUID;
  act TEXT;
BEGIN
  actor := NULLIF(current_setting('app.actor_user_id', true), '')::UUID;

  IF TG_OP = 'INSERT' THEN
    record_id := NEW.id::TEXT;
    act := 'INSERT';
    INSERT INTO audit_logs (table_name, record_pk, action, new_data, actor_user_id)
    VALUES (TG_TABLE_NAME, record_id, act, to_jsonb(NEW), actor);
    RETURN NEW;
  ELSIF TG_OP = 'UPDATE' THEN
    record_id := NEW.id::TEXT;
    IF (to_jsonb(OLD)->>'deleted_at') IS NULL AND (to_jsonb(NEW)->>'deleted_at') IS NOT NULL THEN
      act := 'DELETE';
    ELSIF TG_TABLE_NAME IN ('leases','deposits','expense_claims','applicants','lending_records',
                            'dunning_notices','warnings','terminations','incidents','work_events')
      AND (to_jsonb(OLD)->>'status') IS DISTINCT FROM (to_jsonb(NEW)->>'status') THEN
      act := 'STATUS_CHANGE';
    ELSIF TG_TABLE_NAME = 'users'
      AND (to_jsonb(OLD)->>'anonymized_at') IS NULL AND (to_jsonb(NEW)->>'anonymized_at') IS NOT NULL THEN
      act := 'ANONYMIZE';
    ELSE
      act := 'UPDATE';
    END IF;
    INSERT INTO audit_logs (table_name, record_pk, action, old_data, new_data, actor_user_id)
    VALUES (TG_TABLE_NAME, record_id, act, to_jsonb(OLD), to_jsonb(NEW), actor);
    RETURN NEW;
  END IF;

  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
  tbl TEXT;
BEGIN
  FOR tbl IN
    SELECT unnest(ARRAY[
      'users','user_contact_points',
      'groups',
      'units','garages','parcels',
      'unit_ownerships','leases','applicants',
      'documents',
      'meter_readings',
      'accounting_journal','accounting_entries',
      'billing_periods','billing_items','billing_snapshots',
      'inventory_items','deposits','work_logs','expense_claims',
      'lendable_items','lending_records',
      'bank_accounts','bank_transactions','bank_categorization_rules',
      'shared_links','webhook_subscriptions',
      'work_hour_requirements','unit_work_hour_obligations','user_work_hour_adjustments',
      'work_task_catalog','work_events','work_event_participants',
      'legal_provisions','incidents','incident_witnesses',
      'dunning_notices','warnings','terminations'
    ])
  LOOP
    EXECUTE format(
      'CREATE TRIGGER tr_%s_audit AFTER INSERT OR UPDATE ON %I FOR EACH ROW EXECUTE PROCEDURE audit_trigger_func();',
      tbl, tbl
    );
  END LOOP;
END;
$$;

-- =============================================================================
-- 13) METADATA (EAV fuer Plugin-Erweiterungen)
-- =============================================================================

DO $$
DECLARE
  entity RECORD;
BEGIN
  FOR entity IN
    SELECT tbl_name, ref_table, ref_column
    FROM (VALUES
      ('users_meta',         'users',              'id'),
      ('groups_meta',        'groups',             'id'),
      ('units_meta',         'units',              'id'),
      ('leases_meta',        'leases',             'id'),
      ('documents_meta',     'documents',          'id'),
      ('applicants_meta',    'applicants',         'id'),
      ('journal_meta',       'accounting_journal', 'id'),
      ('billing_meta',       'billing_periods',    'id'),
      ('bank_tx_meta',       'bank_transactions',  'id'),
      ('lendable_meta',      'lendable_items',     'id'),
      ('work_events_meta',   'work_events',        'id'),
      ('incidents_meta',     'incidents',          'id')
    ) AS t(tbl_name, ref_table, ref_column)
  LOOP
    EXECUTE format(
      'CREATE TABLE %I (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        entity_id UUID NOT NULL REFERENCES %I(%I) ON DELETE CASCADE,
        meta_key TEXT NOT NULL,
        meta_value TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        UNIQUE(entity_id, meta_key)
      );
      CREATE INDEX ON %I (entity_id);
      CREATE TRIGGER tr_%s_updated_at BEFORE UPDATE ON %I FOR EACH ROW EXECUTE PROCEDURE set_updated_at();',
      entity.tbl_name, entity.ref_table, entity.ref_column,
      entity.tbl_name,
      entity.tbl_name, entity.tbl_name
    );
  END LOOP;
END;
$$;

-- =============================================================================
-- 14) SEED DATA
-- =============================================================================

-- Rollen
INSERT INTO roles (slug, display_name, description, is_protected) VALUES
  ('admin',     'Administrator',   'Vollzugriff auf alle Funktionen',              TRUE),
  ('vorstand',  'Vorstand',        'Vereinsvorstand mit erweiterten Rechten',      TRUE),
  ('kassierer', 'Kassierer',       'Finanzverwaltung und Buchhaltung',             TRUE),
  ('mitglied',  'Mitglied',        'Regulaeres Vereinsmitglied',                   FALSE),
  ('pruefer',   'Pruefer',         'Kassenpruefer mit Leserechten auf Finanzen',   FALSE);

-- Permissions
INSERT INTO permissions (slug, description, is_protected) VALUES
  ('users.read',           'Benutzer anzeigen',                    TRUE),
  ('users.write',          'Benutzer anlegen und bearbeiten',      TRUE),
  ('users.delete',         'Benutzer loeschen',                    TRUE),
  ('units.read',           'Pachteinheiten anzeigen',              TRUE),
  ('units.write',          'Pachteinheiten anlegen und bearbeiten',TRUE),
  ('units.delete',         'Pachteinheiten loeschen',              TRUE),
  ('leases.read',          'Mietvertraege anzeigen',               TRUE),
  ('leases.write',         'Mietvertraege anlegen und bearbeiten', TRUE),
  ('leases.delete',        'Mietvertraege loeschen',               TRUE),
  ('ownership.read',       'Eigentumsverhaeltnisse anzeigen',      TRUE),
  ('ownership.write',      'Eigentumsverhaeltnisse bearbeiten',    TRUE),
  ('accounting.read',      'Buchhaltung anzeigen',                 TRUE),
  ('accounting.write',     'Buchungen erstellen',                  TRUE),
  ('accounting.lock',      'Journal sperren',                      TRUE),
  ('billing.read',         'Abrechnungen anzeigen',                TRUE),
  ('billing.write',        'Abrechnungen erstellen',               TRUE),
  ('billing.approve',      'Abrechnungen freigeben',               TRUE),
  ('banking.import',       'Bankdaten importieren',                TRUE),
  ('banking.rules',        'Kategorisierungsregeln verwalten',     TRUE),
  ('documents.read',       'Dokumente anzeigen',                   TRUE),
  ('documents.write',      'Dokumente hochladen',                  TRUE),
  ('documents.delete',     'Dokumente loeschen',                   TRUE),
  ('audit.read',           'Audit-Log anzeigen',                   TRUE),
  ('applicants.read',      'Bewerbungen anzeigen',                 TRUE),
  ('applicants.write',     'Bewerbungen bearbeiten',               TRUE),
  ('applicants.assign',    'Pachteinheit zuweisen',                TRUE),
  ('groups.read',          'Gruppen anzeigen',                     TRUE),
  ('groups.write',         'Gruppen verwalten',                    TRUE),
  ('sharing.read',         'Shared Links anzeigen',                TRUE),
  ('sharing.write',        'QR-Codes/Links erstellen',             TRUE),
  ('metering.read',        'Zaehlerstaende anzeigen',              TRUE),
  ('metering.write',       'Zaehlerstaende erfassen',              TRUE),
  ('operations.read',      'Inventar/Kautionen/Spesen anzeigen',  TRUE),
  ('operations.write',     'Inventar/Kautionen/Spesen bearbeiten', TRUE),
  ('lending.read',         'Geraeteverleih anzeigen',              TRUE),
  ('lending.write',        'Geraeteverleih verwalten',             TRUE),
  ('workhours.read',       'Arbeitsstunden anzeigen',              TRUE),
  ('workhours.write',      'Arbeitsstunden verwalten',             TRUE),
  ('workhours.plan',       'Arbeitseinsaetze planen',              TRUE),
  ('compliance.read',      'Verstoesse/Mahnungen anzeigen',        TRUE),
  ('compliance.write',     'Verstoesse/Mahnungen erstellen',       TRUE),
  ('compliance.approve',   'Mahnungen/Abmahnungen freigeben',      TRUE),
  ('webhooks.read',        'Webhooks anzeigen',                    TRUE),
  ('webhooks.write',       'Webhooks verwalten',                   TRUE),
  ('admin.jobs',           'Job-Queue verwalten (Retry/Cancel)',    TRUE),
  ('meta.read',            'Metadaten lesen',                      TRUE),
  ('meta.write',           'Metadaten schreiben',                  TRUE);

-- Role -> Permission Zuordnungen
INSERT INTO role_permissions (role_slug, permission_slug)
SELECT 'admin', slug FROM permissions;

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('vorstand', 'users.read'),
  ('vorstand', 'users.write'),
  ('vorstand', 'units.read'),
  ('vorstand', 'units.write'),
  ('vorstand', 'leases.read'),
  ('vorstand', 'leases.write'),
  ('vorstand', 'ownership.read'),
  ('vorstand', 'ownership.write'),
  ('vorstand', 'accounting.read'),
  ('vorstand', 'billing.read'),
  ('vorstand', 'billing.approve'),
  ('vorstand', 'documents.read'),
  ('vorstand', 'documents.write'),
  ('vorstand', 'audit.read'),
  ('vorstand', 'applicants.read'),
  ('vorstand', 'applicants.write'),
  ('vorstand', 'applicants.assign'),
  ('vorstand', 'groups.read'),
  ('vorstand', 'groups.write'),
  ('vorstand', 'sharing.read'),
  ('vorstand', 'sharing.write'),
  ('vorstand', 'metering.read'),
  ('vorstand', 'operations.read'),
  ('vorstand', 'lending.read'),
  ('vorstand', 'lending.write'),
  ('vorstand', 'workhours.read'),
  ('vorstand', 'workhours.write'),
  ('vorstand', 'workhours.plan'),
  ('vorstand', 'compliance.read'),
  ('vorstand', 'compliance.write'),
  ('vorstand', 'compliance.approve'),
  ('vorstand', 'webhooks.read'),
  ('vorstand', 'webhooks.write'),
  ('vorstand', 'admin.jobs'),
  ('vorstand', 'meta.read');

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('kassierer', 'users.read'),
  ('kassierer', 'units.read'),
  ('kassierer', 'leases.read'),
  ('kassierer', 'accounting.read'),
  ('kassierer', 'accounting.write'),
  ('kassierer', 'accounting.lock'),
  ('kassierer', 'billing.read'),
  ('kassierer', 'billing.write'),
  ('kassierer', 'billing.approve'),
  ('kassierer', 'banking.import'),
  ('kassierer', 'banking.rules'),
  ('kassierer', 'documents.read'),
  ('kassierer', 'documents.write'),
  ('kassierer', 'operations.read'),
  ('kassierer', 'operations.write'),
  ('kassierer', 'metering.read'),
  ('kassierer', 'metering.write'),
  ('kassierer', 'workhours.read'),
  ('kassierer', 'compliance.read'),
  ('kassierer', 'groups.read');

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('mitglied', 'units.read'),
  ('mitglied', 'leases.read'),
  ('mitglied', 'documents.read'),
  ('mitglied', 'metering.read'),
  ('mitglied', 'lending.read'),
  ('mitglied', 'workhours.read'),
  ('mitglied', 'groups.read');

INSERT INTO role_permissions (role_slug, permission_slug) VALUES
  ('pruefer', 'accounting.read'),
  ('pruefer', 'billing.read'),
  ('pruefer', 'audit.read'),
  ('pruefer', 'banking.import'),
  ('pruefer', 'documents.read'),
  ('pruefer', 'operations.read'),
  ('pruefer', 'workhours.read'),
  ('pruefer', 'compliance.read'),
  ('pruefer', 'groups.read');

-- Kontenplan (vereinfachter SKR49)
INSERT INTO accounting_accounts (code, name, account_type) VALUES
  ('1000', 'Kasse',                         'asset'),
  ('1200', 'Bank',                          'asset'),
  ('1400', 'Forderungen aus Beitraegen',    'asset'),
  ('1500', 'Sonstige Forderungen',          'asset'),
  ('2000', 'Verbindlichkeiten',             'liability'),
  ('2100', 'Erhaltene Kautionen',           'liability'),
  ('2900', 'Sonstige Verbindlichkeiten',    'liability'),
  ('3000', 'Eigenkapital',                  'equity'),
  ('3900', 'Ruecklagen',                    'equity'),
  ('4000', 'Mitgliedsbeitraege',            'income'),
  ('4100', 'Pachteinnahmen',               'income'),
  ('4200', 'Spenden',                       'income'),
  ('4300', 'Erstattungen Nebenkosten',      'income'),
  ('4400', 'Leihgebuehren',                 'income'),
  ('4500', 'Arbeitsstunden-Ersatzgeld',     'income'),
  ('4600', 'Mahngebuehren',                 'income'),
  ('4900', 'Sonstige Ertraege',             'income'),
  ('6000', 'Instandhaltung',               'expense'),
  ('6100', 'Strom / Energie',              'expense'),
  ('6200', 'Wasser / Abwasser',            'expense'),
  ('6300', 'Versicherungen',               'expense'),
  ('6400', 'Verwaltungskosten',            'expense'),
  ('6500', 'Muellentsorgung',              'expense'),
  ('6600', 'Grundsteuer / Abgaben',        'expense'),
  ('6900', 'Sonstige Aufwendungen',        'expense');

-- Dokument-Kategorien
INSERT INTO document_categories (slug, display_name, description, sort_order, is_protected) VALUES
  ('vertrag',   'Vertrag',       'Miet-, Pacht- und sonstige Vertraege',  10, TRUE),
  ('beleg',     'Beleg',         'Quittungen, Kassenbelege',              20, TRUE),
  ('protokoll', 'Protokoll',     'Sitzungs- und Versammlungsprotokolle',  30, TRUE),
  ('rechnung',  'Rechnung',      'Ein- und Ausgangsrechnungen',           40, TRUE),
  ('bescheid',  'Bescheid',      'Behoerdliche Bescheide und Genehmigungen', 50, TRUE),
  ('foto',      'Foto',          'Fotos und Bilddokumentation',           60, FALSE),
  ('plan',      'Plan / Karte',  'Lageplaaene, Flurkarten, Grundrisse',   70, FALSE),
  ('sonstiges', 'Sonstiges',     'Nicht zugeordnete Dokumente',           99, FALSE);

-- Rechtsgrundlagen-Katalog
INSERT INTO legal_provisions (slug, title, source, reference, full_text, sort_order) VALUES
  ('sat-mitgliedschaft',   'Mitgliedschaftspflichten',        'vereinssatzung',          '§4',
   'Jedes Mitglied ist verpflichtet, die Satzung und die Ordnungen des Vereins einzuhalten.', 10),
  ('sat-beitragspflicht',  'Beitragspflicht',                 'vereinssatzung',          '§6 Abs. 1',
   'Jedes Mitglied ist zur Zahlung des Jahresbeitrags verpflichtet.', 20),
  ('sat-arbeitspflicht',   'Gemeinschaftsarbeit',             'vereinssatzung',          '§6 Abs. 3',
   'Jedes Mitglied ist zur Leistung von Gemeinschaftsarbeit verpflichtet. Nicht geleistete Stunden werden als Ersatzgeld berechnet.', 30),
  ('sat-kuendigung',       'Kuendigung durch den Verein',     'vereinssatzung',          '§7 Abs. 3',
   'Der Verein kann das Mitglied bei schwerwiegenden oder wiederholten Verstoessen ausschliessen.', 40),
  ('go-ruhezeiten',        'Ruhezeiten',                      'gartenordnung',           '§5 Abs. 2',
   'Ruhezeiten sind taeglich von 13:00-15:00 Uhr und 22:00-07:00 Uhr einzuhalten.', 50),
  ('go-bauvorschrift',     'Baugenehmigungspflicht',          'gartenordnung',           '§8 Abs. 1',
   'Bauliche Veraenderungen beduerfen der vorherigen schriftlichen Genehmigung durch den Vorstand.', 60),
  ('go-gartenpflege',      'Gartenpflege',                    'gartenordnung',           '§3 Abs. 1',
   'Jeder Paechter ist verpflichtet, seinen Garten ordnungsgemaess zu bewirtschaften und zu pflegen.', 70),
  ('go-tierhaltung',       'Tierhaltung',                     'gartenordnung',           '§10',
   'Das Halten von Tieren in der Gartenanlage bedarf der Genehmigung des Vorstands.', 80),
  ('go-laerm',             'Laermschutz',                     'gartenordnung',           '§5 Abs. 1',
   'Jeder Paechter hat unnoetigen Laerm zu vermeiden. Motorbetriebene Geraete sind nur ausserhalb der Ruhezeiten zu verwenden.', 90),
  ('bkleing-nutzung',      'Kleingaertnerische Nutzung',      'bundeskleingartengesetz', '§1 Abs. 1 Nr. 1',
   'Ein Kleingarten ist ein Garten, der dem Nutzer zur nichterwerbsmaessigen gaertnerischen Nutzung ueberlassen ist.', 100),
  ('bkleing-laube',        'Laube',                           'bundeskleingartengesetz', '§3 Abs. 2',
   'Im Kleingarten ist eine Laube in einfacher Ausfuehrung mit hoechstens 24 qm Grundflaeche zulaessig.', 110),
  ('bkleing-kuendigung',   'Kuendigung des Pachtverhaeltnisses', 'bundeskleingartengesetz', '§9',
   'Der Verpaechter kann den Kleingartenpachtvertrag kuendigen, wenn der Paechter ungeachtet einer Abmahnung eine nicht kleingaertnerische Nutzung fortsetzt.', 120);

-- Aufgabenkatalog
INSERT INTO work_task_catalog (title, description, category, typical_participants, typical_hours,
  is_seasonal, season_months, recurrence_interval_days) VALUES
  ('Hecken schneiden',          'Hecken an Vereinswegen und Grundstuecksgrenzen', 'garten', 3, 3, TRUE,  '{5,6,7,8,9}', 56),
  ('Wege kehren und reinigen',  'Vereinswege von Laub und Schmutz befreien',      'wege',   2, 2, FALSE, NULL, 28),
  ('Laub kehren',               'Herbstlaub auf Vereinswegen und Gemeinschaftsflaechen', 'garten', 5, 3, TRUE, '{10,11}', 14),
  ('Vereinshaus reinigen',      'Innenreinigung des Vereinshauses',                'gebaeude', 2, 2, FALSE, NULL, 28),
  ('Wasserleitung Kontrolle',   'Frostsicherung pruefen, Leitungen kontrollieren', 'infrastruktur', 1, 1, TRUE, '{3,4}', NULL),
  ('Wasserleitung winterfest',  'Leitungen entleeren und absperren',               'infrastruktur', 2, 2, TRUE, '{10,11}', NULL),
  ('Fruehjahrsputz',           'Grosser Gemeinschaftseinsatz zum Saisonstart',     'garten', 8, 4, TRUE, '{3}', NULL),
  ('Herbstputz',               'Grosser Gemeinschaftseinsatz zum Saisonende',      'garten', 8, 4, TRUE, '{10,11}', NULL),
  ('Spielplatz Kontrolle',     'Sicherheitskontrolle der Spielgeraete',            'infrastruktur', 1, 1, FALSE, NULL, 30),
  ('Zaun reparieren',          'Reparatur- und Instandhaltungsarbeiten am Zaun',   'infrastruktur', 2, 3, FALSE, NULL, NULL),
  ('Vereinsfest Auf-/Abbau',   'Aufbau und Abbau fuer Vereinsveranstaltungen',     'veranstaltung', 6, 4, FALSE, NULL, NULL),
  ('Muellsammelplatz reinigen', 'Container-Stellplatz sauber halten',              'wege', 1, 1, FALSE, NULL, 14);

-- =============================================================================
-- 15) FULL-TEXT SEARCH (PostgreSQL tsvector)
-- =============================================================================

ALTER TABLE users ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('german', coalesce(first_name,'') || ' ' || coalesce(last_name,'') || ' ' || coalesce(email,''))
  ) STORED;
CREATE INDEX ix_users_search ON users USING GIN(search_vector);

ALTER TABLE units ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('german', coalesce(label,'') || ' ' || coalesce(note,''))
  ) STORED;
CREATE INDEX ix_units_search ON units USING GIN(search_vector);

ALTER TABLE documents ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('german', coalesce(title,''))
  ) STORED;
CREATE INDEX ix_documents_search ON documents USING GIN(search_vector);

ALTER TABLE bank_transactions ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('german', coalesce(payee,'') || ' ' || coalesce(description,''))
  ) STORED;
CREATE INDEX ix_bank_tx_search ON bank_transactions USING GIN(search_vector);

ALTER TABLE lendable_items ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('german', coalesce(name,'') || ' ' || coalesce(description,''))
  ) STORED;
CREATE INDEX ix_lendable_items_search ON lendable_items USING GIN(search_vector);

ALTER TABLE incidents ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('german', coalesce(title,'') || ' ' || coalesce(description,''))
  ) STORED;
CREATE INDEX ix_incidents_search ON incidents USING GIN(search_vector);

ALTER TABLE work_task_catalog ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('german', coalesce(title,'') || ' ' || coalesce(description,''))
  ) STORED;
CREATE INDEX ix_work_task_catalog_search ON work_task_catalog USING GIN(search_vector);

-- =============================================================================
-- 16) VIEWS
-- =============================================================================

CREATE OR REPLACE VIEW view_work_hour_balance AS
SELECT
  u.id AS user_id,
  u.last_name || ', ' || u.first_name AS name,
  r.year,
  r.base_hours,
  COALESCE(uwo.additional_hours, 0) AS unit_hours,
  COALESCE(adj.adjusted_hours, 0) AS adjustment,
  (r.base_hours + COALESCE(uwo.additional_hours, 0) + COALESCE(adj.adjusted_hours, 0)) AS total_required,
  COALESCE(worked.total, 0) AS hours_worked,
  GREATEST(
    (r.base_hours + COALESCE(uwo.additional_hours, 0) + COALESCE(adj.adjusted_hours, 0))
    - COALESCE(worked.total, 0),
    0
  ) AS hours_missing,
  GREATEST(
    (r.base_hours + COALESCE(uwo.additional_hours, 0) + COALESCE(adj.adjusted_hours, 0))
    - COALESCE(worked.total, 0),
    0
  ) * r.fee_per_missing_hour AS replacement_fee,
  r.deadline,
  r.fee_per_missing_hour
FROM users u
CROSS JOIN work_hour_requirements r
LEFT JOIN leases l ON l.tenant_user_id = u.id
  AND l.deleted_at IS NULL AND l.status = 'active'
LEFT JOIN unit_work_hour_obligations uwo ON uwo.unit_id = l.unit_id
  AND uwo.year = r.year AND uwo.deleted_at IS NULL
LEFT JOIN user_work_hour_adjustments adj ON adj.user_id = u.id
  AND adj.year = r.year AND adj.deleted_at IS NULL
LEFT JOIN (
  SELECT user_id, applied_to_year, SUM(hours_spent) AS total
  FROM work_logs
  WHERE is_verified = TRUE AND deleted_at IS NULL
  GROUP BY user_id, applied_to_year
) worked ON worked.user_id = u.id AND worked.applied_to_year = r.year
WHERE u.deleted_at IS NULL AND u.is_active = TRUE;

COMMIT;
