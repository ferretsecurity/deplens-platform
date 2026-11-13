-- +goose Up
alter table scan_manifests
    add column form text not null default '',
    add column extraction text not null default '',
    add column lockfiles jsonb not null default 'null'::jsonb;

-- +goose Down
alter table scan_manifests drop column lockfiles, drop column extraction, drop column form;
