-- +goose Up
create table if not exists scan_manifests (
    id uuid primary key default gen_random_uuid(),
    scan_id uuid not null references scans(id) on delete cascade,
    position integer not null,
    type text not null,
    path text not null,
    has_dependencies boolean null,
    warnings jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    unique (scan_id, position)
);

create index if not exists scan_manifests_scan_position_idx
    on scan_manifests (scan_id, position);

create table if not exists manifest_dependencies (
    id uuid primary key default gen_random_uuid(),
    manifest_id uuid not null references scan_manifests(id) on delete cascade,
    position integer not null,
    raw text not null,
    name text not null default '',
    version text not null default '',
    "constraint" text not null default '',
    section text not null default '',
    source text not null default '',
    extras jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    unique (manifest_id, position)
);

create index if not exists manifest_dependencies_manifest_position_idx
    on manifest_dependencies (manifest_id, position);

-- +goose Down
drop table if exists manifest_dependencies;
drop table if exists scan_manifests;
