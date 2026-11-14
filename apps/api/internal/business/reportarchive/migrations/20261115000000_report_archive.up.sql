-- Report archive phase 1: tenant-scoped reports, originals, OCR runs, parse runs,
-- template rules, review events and the durable processing queue.
-- Originals are immutable bytea in this phase so file persistence and business writes commit
-- atomically; report_files is the only place a PDF is read from.

CREATE TABLE report_files (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    file_name text NOT NULL,
    content_type text NOT NULL DEFAULT 'application/pdf',
    size bigint NOT NULL CHECK (size > 0),
    pages integer NOT NULL CHECK (pages > 0),
    content bytea NOT NULL,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, sha256)
);

CREATE TABLE report_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_id uuid NOT NULL REFERENCES report_files(id),
    report_number text NOT NULL DEFAULT '',
    report_name text NOT NULL DEFAULT '',
    product_name text NOT NULL DEFAULT '',
    institution text NOT NULL DEFAULT '',
    client_unit text NOT NULL DEFAULT '',
    inspected_unit text NOT NULL DEFAULT '',
    manufacturer text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    basis text NOT NULL DEFAULT '',
    report_date date,
    conclusion text NOT NULL DEFAULT '',
    detection_result text NOT NULL DEFAULT 'unknown'
        CHECK (detection_result IN ('unknown','qualified','unqualified')),
    archive_status text NOT NULL DEFAULT 'pending_ocr'
        CHECK (archive_status IN ('pending_ocr','ocr_running','parsing','needs_review','archived','voided')),
    review_stage text NOT NULL DEFAULT '',
    review_reason text NOT NULL DEFAULT '',
    archive_reason text NOT NULL DEFAULT '',
    report_version integer NOT NULL DEFAULT 1 CHECK (report_version > 0),
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    revision_of uuid REFERENCES report_reports(id),
    duplicate_of uuid REFERENCES report_reports(id),
    is_current boolean NOT NULL DEFAULT false,
    ocr_run_id uuid,
    parse_run_id uuid,
    rule_id uuid,
    rule_version integer,
    field_sources jsonb NOT NULL DEFAULT '{}'::jsonb,
    raw_fields jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    voided_at timestamptz,
    void_reason text NOT NULL DEFAULT ''
);

-- One current, non-voided version per tenant + institution + report number. Empty keys are drafts
-- and never claim the current slot.
CREATE UNIQUE INDEX report_reports_current_key
    ON report_reports (tenant_id, lower(institution), lower(report_number))
    WHERE is_current AND voided_at IS NULL AND institution <> '' AND report_number <> '';
CREATE INDEX report_reports_tenant_status ON report_reports (tenant_id, archive_status, updated_at DESC);
CREATE INDEX report_reports_tenant_date ON report_reports (tenant_id, report_date DESC NULLS LAST);
CREATE INDEX report_reports_file ON report_reports (tenant_id, file_id);

CREATE TABLE report_details (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES report_reports(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    section text NOT NULL DEFAULT '',
    item_name text NOT NULL DEFAULT '',
    requirement text NOT NULL DEFAULT '',
    unit text NOT NULL DEFAULT '',
    method text NOT NULL DEFAULT '',
    result_raw text NOT NULL DEFAULT '',
    judgement text NOT NULL DEFAULT '',
    position integer NOT NULL CHECK (position >= 0),
    source_page integer NOT NULL CHECK (source_page >= 1),
    source_text text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (report_id, position)
);
CREATE INDEX report_details_report ON report_details (report_id, position);
CREATE INDEX report_details_item ON report_details (tenant_id, item_name);
CREATE INDEX report_details_judgement ON report_details (tenant_id, judgement);
CREATE INDEX report_details_combo ON report_details (tenant_id, item_name, judgement);

CREATE TABLE report_ocr_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES report_reports(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('running','succeeded','failed')),
    provider text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    raw jsonb,
    pages jsonb NOT NULL DEFAULT '[]'::jsonb,
    text text NOT NULL DEFAULT '',
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_ocr_runs_report ON report_ocr_runs (report_id, created_at DESC);

CREATE TABLE report_parse_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES report_reports(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ocr_run_id uuid REFERENCES report_ocr_runs(id),
    rule_id uuid,
    rule_version integer,
    status text NOT NULL CHECK (status IN ('succeeded','failed','candidate','applied','superseded')),
    fields jsonb NOT NULL DEFAULT '{}'::jsonb,
    details jsonb NOT NULL DEFAULT '[]'::jsonb,
    issues jsonb NOT NULL DEFAULT '[]'::jsonb,
    raw jsonb,
    applied_by text NOT NULL DEFAULT '',
    applied_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_parse_runs_report ON report_parse_runs (report_id, created_at DESC);
CREATE INDEX report_parse_runs_candidate ON report_parse_runs (report_id, status) WHERE status = 'candidate';

CREATE TABLE report_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES report_reports(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    action text NOT NULL,
    actor text NOT NULL,
    reason text NOT NULL DEFAULT '',
    before jsonb,
    after jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_events_report ON report_events (report_id, created_at);

CREATE TABLE report_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid REFERENCES tenants(id) ON DELETE CASCADE,
    template_key text NOT NULL,
    name text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('draft','active','retired')),
    config jsonb NOT NULL,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    activated_at timestamptz,
    retired_at timestamptz,
    CHECK (length(template_key) BETWEEN 1 AND 80),
    CHECK (length(name) BETWEEN 1 AND 120)
);
CREATE UNIQUE INDEX report_rules_version_key
    ON report_rules (COALESCE(tenant_id::text, ''), template_key, version);
CREATE UNIQUE INDEX report_rules_active_key
    ON report_rules (COALESCE(tenant_id::text, ''), template_key)
    WHERE status = 'active';

CREATE TABLE report_rule_trials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    rule_id uuid NOT NULL REFERENCES report_rules(id) ON DELETE CASCADE,
    created_by text NOT NULL,
    text_sha256 text NOT NULL,
    result jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE report_upload_batches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by text NOT NULL,
    total integer NOT NULL DEFAULT 0,
    accepted integer NOT NULL DEFAULT 0,
    failed integer NOT NULL DEFAULT 0,
    duplicate integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_upload_batches_tenant ON report_upload_batches (tenant_id, created_at DESC);

CREATE TABLE report_upload_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id uuid NOT NULL REFERENCES report_upload_batches(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_name text NOT NULL,
    status text NOT NULL CHECK (status IN ('accepted','rejected','duplicate')),
    report_id uuid REFERENCES report_reports(id),
    file_id uuid REFERENCES report_files(id),
    sha256 text NOT NULL DEFAULT '',
    size bigint NOT NULL DEFAULT 0,
    pages integer NOT NULL DEFAULT 0,
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_upload_items_batch ON report_upload_items (batch_id, created_at);

CREATE TABLE report_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    report_id uuid NOT NULL REFERENCES report_reports(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('process_report','reparse_report')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed','paused')),
    attempts integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 4 CHECK (max_attempts > 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by text NOT NULL DEFAULT '',
    last_error text NOT NULL DEFAULT '',
    error_stage text NOT NULL DEFAULT '',
    idempotency_key text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_jobs_due ON report_jobs (status, available_at) WHERE status = 'queued';
CREATE INDEX report_jobs_report ON report_jobs (report_id, created_at DESC);
