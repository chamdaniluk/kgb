-- 001_schema.sql — skema awal SI CENDIKIA (9 tabel)
-- Sumber: ERD v1.0 FINAL §2 (struktur kolom) dan §5 (indeks & batasan).
-- Konvensi: CHECK constraint untuk enum; updated_at di-set aplikasi (tanpa trigger).

CREATE TABLE units (
    id         int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       text NOT NULL UNIQUE,
    name       text NOT NULL,
    type       text NOT NULL CHECK (type IN ('korwil', 'smp', 'skb')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role          text NOT NULL CHECK (role IN ('asn', 'verifikator_unit', 'verifikator_dinas', 'pimpinan', 'admin')),
    name          text NOT NULL,
    unit_id       int REFERENCES units(id),
    is_active     boolean NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE teachers (
    id               int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id          int UNIQUE REFERENCES users(id),
    nip              varchar(18) NOT NULL UNIQUE,
    name             text NOT NULL,
    asn_type         text NOT NULL CHECK (asn_type IN ('pns', 'pppk')),
    unit_id          int NOT NULL REFERENCES units(id),
    pangkat_gol      text NOT NULL,
    masa_kerja_tahun int NOT NULL CHECK (masa_kerja_tahun >= 0),
    tmt_kgb_last     date,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE submissions (
    id             int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    teacher_id     int NOT NULL REFERENCES teachers(id),
    status         text NOT NULL CHECK (status IN
        ('menunggu_unit', 'menunggu_dinas', 'menunggu_tte',
         'dikembalikan_unit', 'dikembalikan_dinas', 'terbit')),
    proposed_tmt   date NOT NULL,
    current_salary numeric(12, 0),
    next_salary    numeric(12, 0),
    file_name      text,
    file_path      text,
    file_size      int,
    rejection_note text,
    submitted_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE letter_number_templates (
    id         int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pattern    text NOT NULL,
    is_active  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE letters (
    id             int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    submission_id  int NOT NULL UNIQUE REFERENCES submissions(id),
    template_id    int NOT NULL REFERENCES letter_number_templates(id),
    number         text NOT NULL UNIQUE,
    pdf_path       text NOT NULL,
    signer_user_id int NOT NULL REFERENCES users(id),
    tte_receipt_id text,
    issued_at      timestamptz NOT NULL DEFAULT now(),
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id  int REFERENCES users(id),
    submission_id  int REFERENCES submissions(id),
    action         text NOT NULL,
    details        jsonb,
    ip             text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE bkn_imports (
    id           int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    file_name    text NOT NULL,
    imported_by  int NOT NULL REFERENCES users(id),
    rows_total   int NOT NULL DEFAULT 0,
    rows_created int NOT NULL DEFAULT 0,
    rows_updated int NOT NULL DEFAULT 0,
    rows_skipped int NOT NULL DEFAULT 0,
    notes        text,
    imported_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE salary_scales (
    id               int GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    asn_type         text NOT NULL CHECK (asn_type IN ('pns', 'pppk')),
    golongan         text NOT NULL,
    masa_kerja_tahun int NOT NULL CHECK (masa_kerja_tahun >= 0),
    gaji             numeric(12, 0) NOT NULL,
    UNIQUE (asn_type, golongan, masa_kerja_tahun)
);

-- Indeks query panas (ERD §5)
CREATE INDEX idx_submissions_status  ON submissions (status);
CREATE INDEX idx_submissions_teacher ON submissions (teacher_id);
CREATE INDEX idx_teachers_unit       ON teachers (unit_id);
CREATE INDEX idx_audit_submission    ON audit_logs (submission_id, created_at);

-- Hanya satu pengajuan aktif per guru. Status terbit adalah riwayat final.
CREATE UNIQUE INDEX uq_submission_one_active_per_teacher
    ON submissions (teacher_id)
    WHERE status <> 'terbit';

-- Hanya satu template nomor surat aktif pada satu waktu (ERD §5)
CREATE UNIQUE INDEX uq_letter_template_active
    ON letter_number_templates (is_active) WHERE is_active;

-- Template awal dapat diganti admin, tetapi sistem langsung dapat menerbitkan surat
-- setelah instalasi tanpa konfigurasi tersembunyi.
INSERT INTO letter_number_templates (pattern, is_active)
SELECT '800/{SEQ}/4.2/{YEAR}', true
WHERE NOT EXISTS (SELECT 1 FROM letter_number_templates);
