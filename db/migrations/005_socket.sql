-- +goose Up
create table socket_connections (
 id uuid primary key default gen_random_uuid(),
 tenant_id uuid not null unique references tenants(id) on delete cascade,
 organization text not null,
 token_cipher bytea not null,
 interval_minutes integer not null default 0 check(interval_minutes in (0,60,360,720,1440)),
 next_run_at timestamptz,
 created_at timestamptz not null default now()
);
create table socket_runs (
 id uuid primary key default gen_random_uuid(),
 connection_id uuid not null references socket_connections(id) on delete cascade,
 status text not null check(status in ('queued','running','completed','partial','failed','cancelled')),
 owner uuid,
 lease_until timestamptz,
 started_at timestamptz not null default now(),
 finished_at timestamptz,
 total integer not null default 0,
 completed integer not null default 0,
 failed integer not null default 0,
 message text not null default ''
);
create unique index socket_one_active_run on socket_runs(connection_id) where status in ('queued','running');
create table socket_repositories (
 id uuid primary key default gen_random_uuid(),
 connection_id uuid not null references socket_connections(id) on delete cascade,
 remote_id text not null,
 name text not null,
 scm_url text not null default '',
 dashboard_url text not null default '',
 default_branch text not null default '',
 cli_repository_id uuid references repositories(id) on delete set null,
 mapping_explicit boolean not null default false,
 available boolean not null default true,
 snapshot jsonb,
 scan_id text not null default '',
 last_error text not null default '',
 synced_at timestamptz,
 unique(connection_id,remote_id)
);
create index socket_repository_cli on socket_repositories(cli_repository_id);
create view inventory_repositories as
 select r.id,r.tenant_id,r.name,r.url,r.default_branch,true as cli,
 exists(select 1 from socket_repositories sr where sr.cli_repository_id=r.id) as socket,
 coalesce((select sr.dashboard_url from socket_repositories sr where sr.cli_repository_id=r.id order by sr.id limit 1),'') as socket_url
 from repositories r
 union all
 select sr.id,c.tenant_id,sr.name,sr.scm_url,sr.default_branch,false,true,sr.dashboard_url
 from socket_repositories sr join socket_connections c on c.id=sr.connection_id
 where sr.cli_repository_id is null;
-- +goose Down
drop view inventory_repositories;
drop table socket_repositories;
drop table socket_runs;
drop table socket_connections;
