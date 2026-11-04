-- +goose Up
alter table manifest_dependencies add column package_type text not null default '';

-- +goose Down
alter table manifest_dependencies drop column package_type;
