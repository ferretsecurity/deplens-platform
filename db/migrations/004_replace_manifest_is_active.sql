-- +goose Up
-- +goose StatementBegin
do $$
begin
    if exists (
        select 1
        from information_schema.columns
        where table_schema = 'public'
          and table_name = 'manifests'
          and column_name = 'is_active'
    ) then
        alter table manifests
            add column if not exists disappeared_at timestamptz null;

        update manifests
        set disappeared_at = now()
        where is_active = false
          and disappeared_at is null;

        drop index if exists manifests_repository_active_path_idx;

        alter table manifests
            drop column is_active;
    end if;

    create index if not exists manifests_repository_active_path_idx
        on manifests (repository_id, path)
        where disappeared_at is null;
end
$$;
-- +goose StatementEnd

-- +goose Down
select 1;
