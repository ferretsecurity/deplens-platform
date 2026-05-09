-- +goose Up
-- +goose StatementBegin
do $$
begin
    if exists (
        select 1
        from information_schema.columns
        where table_schema = 'public'
          and table_name = 'repositories'
          and column_name = 'slug'
    ) then
        create temp table repository_merge_map on commit drop as
        with ranked as (
            select
                id,
                tenant_id,
                name,
                created_at,
                first_value(id) over (
                    partition by tenant_id, name
                    order by created_at asc, id asc
                ) as canonical_id,
                row_number() over (
                    partition by tenant_id, name
                    order by created_at asc, id asc
                ) as rn
            from repositories
        )
        select id as duplicate_id, canonical_id
        from ranked
        where rn > 1;

        update repositories canonical
        set
            url = latest.url,
            default_branch = latest.default_branch
        from (
            with ranked as (
                select
                    first_value(id) over (
                        partition by tenant_id, name
                        order by created_at asc, id asc
                    ) as canonical_id,
                    first_value(url) over (
                        partition by tenant_id, name
                        order by created_at desc, id desc
                    ) as url,
                    first_value(default_branch) over (
                        partition by tenant_id, name
                        order by created_at desc, id desc
                    ) as default_branch,
                    row_number() over (
                        partition by tenant_id, name
                        order by created_at asc, id asc
                    ) as rn
                from repositories
            )
            select canonical_id, url, default_branch
            from ranked
            where rn = 1
        ) as latest
        where canonical.id = latest.canonical_id;

        update manifests canonical
        set
            first_seen_at = least(canonical.first_seen_at, merged.first_seen_at),
            last_seen_at = greatest(canonical.last_seen_at, merged.last_seen_at),
            is_active = canonical.is_active or merged.is_active,
            labels = canonical.labels || merged.labels
        from (
            select
                map.canonical_id,
                manifest.path,
                min(manifest.first_seen_at) as first_seen_at,
                max(manifest.last_seen_at) as last_seen_at,
                bool_or(manifest.is_active) as is_active,
                coalesce(
                    jsonb_object_agg(label.key, label.value) filter (where label.key is not null),
                    '{}'::jsonb
                ) as labels
            from repository_merge_map map
            join manifests manifest on manifest.repository_id = map.duplicate_id
            left join lateral jsonb_each(manifest.labels) as label(key, value) on true
            group by map.canonical_id, manifest.path
        ) as merged
        where canonical.repository_id = merged.canonical_id
          and canonical.path = merged.path;

        insert into manifests (
            repository_id,
            path,
            first_seen_at,
            last_seen_at,
            is_active,
            labels
        )
        select
            map.canonical_id,
            manifest.path,
            min(manifest.first_seen_at),
            max(manifest.last_seen_at),
            bool_or(manifest.is_active),
            coalesce(
                jsonb_object_agg(label.key, label.value) filter (where label.key is not null),
                '{}'::jsonb
            ) as labels
        from repository_merge_map map
        join manifests manifest on manifest.repository_id = map.duplicate_id
        left join lateral jsonb_each(manifest.labels) as label(key, value) on true
        group by map.canonical_id, manifest.path
        on conflict (repository_id, path) do nothing;

        update scan_manifests scan_manifest
        set manifest_id = canonical.id
        from manifests duplicate
        join repository_merge_map map on map.duplicate_id = duplicate.repository_id
        join manifests canonical
          on canonical.repository_id = map.canonical_id
         and canonical.path = duplicate.path
        where scan_manifest.manifest_id = duplicate.id;

        delete from manifests manifest
        using repository_merge_map map
        where manifest.repository_id = map.duplicate_id;

        update scans scan
        set repository_id = map.canonical_id
        from repository_merge_map map
        where scan.repository_id = map.duplicate_id;

        delete from repositories repository
        using repository_merge_map map
        where repository.id = map.duplicate_id;

        alter table repositories drop constraint if exists repositories_tenant_id_slug_key;
        alter table repositories drop column if exists slug;
        create unique index if not exists repositories_tenant_id_name_idx
            on repositories (tenant_id, name);
    end if;
end
$$;
-- +goose StatementEnd

-- +goose Down
alter table repositories add column if not exists slug text;

update repositories
set slug = coalesce(nullif(slug, ''), name)
where slug is null or slug = '';

drop index if exists repositories_tenant_id_name_idx;
alter table repositories drop constraint if exists repositories_tenant_id_name_key;
alter table repositories add constraint repositories_tenant_id_slug_key unique (tenant_id, slug);
