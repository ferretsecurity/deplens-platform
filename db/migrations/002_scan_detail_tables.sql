-- +goose Up
create table if not exists manifests (
    id uuid primary key default gen_random_uuid(),
    repository_id uuid not null references repositories(id) on delete cascade,
    path text not null,
    first_seen_at timestamptz not null,
    last_seen_at timestamptz not null,
    is_active boolean not null,
    labels jsonb not null default '{}'::jsonb,
    unique (repository_id, path)
);

create index if not exists manifests_repository_active_path_idx
    on manifests (repository_id, is_active, path);

create table if not exists scan_manifests (
    id uuid primary key default gen_random_uuid(),
    scan_id uuid not null references scans(id) on delete cascade,
    manifest_id uuid not null references manifests(id) on delete cascade,
    position integer not null,
    type text not null,
    has_dependencies boolean null,
    warnings jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    unique (scan_id, position),
    unique (scan_id, manifest_id)
);

create index if not exists scan_manifests_scan_position_idx
    on scan_manifests (scan_id, position);

create table if not exists manifest_dependencies (
    id uuid primary key default gen_random_uuid(),
    scan_manifest_id uuid not null references scan_manifests(id) on delete cascade,
    position integer not null,
    raw text not null,
    name text not null default '',
    version text not null default '',
    "constraint" text not null default '',
    section text not null default '',
    source text not null default '',
    extras jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    unique (scan_manifest_id, position)
);

create index if not exists manifest_dependencies_manifest_position_idx
    on manifest_dependencies (scan_manifest_id, position);

-- +goose Down
drop table if exists manifest_dependencies;
drop table if exists scan_manifests;
drop table if exists manifests;
