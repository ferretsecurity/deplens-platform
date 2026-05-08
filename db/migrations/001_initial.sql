-- +goose Up
create extension if not exists pgcrypto;

create table if not exists tenants (
    id uuid primary key default gen_random_uuid(),
    slug text not null unique,
    name text not null,
    created_at timestamptz not null default now()
);

create table if not exists users (
    id uuid primary key default gen_random_uuid(),
    display_name text not null,
    primary_email text unique,
    created_at timestamptz not null default now()
);

create table if not exists auth_providers (
    id uuid primary key default gen_random_uuid(),
    provider_type text not null check (provider_type in ('local', 'oidc', 'saml')),
    slug text not null unique,
    display_name text not null,
    issuer text,
    metadata_url text,
    saml_metadata xml,
    is_enabled boolean not null default true,
    created_at timestamptz not null default now()
);

create table if not exists auth_identities (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    provider_id uuid not null references auth_providers(id) on delete cascade,
    subject text not null,
    email text,
    password_hash text,
    created_at timestamptz not null default now(),
    unique (provider_id, subject)
);

create table if not exists tenant_memberships (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    user_id uuid not null references users(id) on delete cascade,
    role text not null check (role in ('owner', 'admin', 'viewer')),
    created_at timestamptz not null default now(),
    unique (tenant_id, user_id)
);

create table if not exists tenant_invites (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    email text not null,
    role text not null check (role in ('owner', 'admin', 'viewer')),
    invite_token_hash text not null unique,
    invited_by_user_id uuid not null references users(id) on delete cascade,
    accepted_at timestamptz,
    expires_at timestamptz not null,
    created_at timestamptz not null default now()
);

create unique index if not exists tenant_invites_active_email_idx
    on tenant_invites (tenant_id, email)
    where accepted_at is null;

create table if not exists http_sessions (
    token text primary key,
    data bytea not null,
    expiry timestamptz not null
);

create index if not exists http_sessions_expiry_idx
    on http_sessions (expiry);

create table if not exists api_tokens (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    label text not null,
    token_hash text not null unique,
    scopes text[] not null,
    created_at timestamptz not null default now()
);

drop table if exists scans;
drop table if exists repositories;
drop table if exists projects;

create table if not exists repositories (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    slug text not null,
    name text not null,
    url text not null,
    default_branch text not null,
    created_at timestamptz not null default now(),
    unique (tenant_id, slug)
);

create table if not exists scans (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    repository_id uuid not null references repositories(id) on delete cascade,
    artifact_key text not null unique,
    artifact_sha256 text not null,
    schema_version text not null,
    root_path text not null,
    commit_sha text not null,
    source_ref text not null,
    scanned_at timestamptz not null,
    manifest_count integer not null,
    manifests_with_dependencies_count integer not null,
    manifests_without_dependencies_count integer not null,
    manifests_unknown_count integer not null,
    dependency_count integer not null,
    labels jsonb not null default '{}'::jsonb,
    annotation text not null default '',
    created_at timestamptz not null default now()
);

create index if not exists scans_tenant_repository_scanned_at_idx
    on scans (tenant_id, repository_id, scanned_at desc);

-- +goose Down
drop table if exists scans;
drop table if exists repositories;
drop table if exists projects;
drop table if exists api_tokens;
drop table if exists http_sessions;
drop table if exists tenant_invites;
drop table if exists tenant_memberships;
drop table if exists auth_identities;
drop table if exists auth_providers;
drop table if exists users;
drop table if exists tenants;
