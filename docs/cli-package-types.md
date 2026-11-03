# CLI package types

Native scanner uploads preserve each dependency's `package_type` string. Inventory exposes that value as `ecosystem`. Missing values stay empty. Values such as `unknown`, future types, `pypi`, and `pip` remain distinct. Scanner attributes remain in `extras` independently.

Apply migration 007 and deploy every upload writer before repairing old observations. The repair reads original native reports from the filesystem blob store and verifies their saved SHA. Legacy artifacts without native reports and unsupported report schemas are reported as skipped. Missing artifacts and observation mismatches fail the command.

Preview repairs using the same database and blob root as the application:

```sh
DATABASE_URL=... BLOB_FILESYSTEM_ROOT=... go run ./cmd/repair-cli-package-types
```

Use `--tenant-id UUID` or `--scan-id UUID` to narrow the scope. Add `--apply` to fill empty package types. Without a scan scope, the command selects retained scans with empty dependency types, including disappeared manifests.

Each scan runs in its own transaction and locks its repository against concurrent uploads. All retained dependencies must match their original source and dependency positions, path, detector, and saved fields. Partially retained reports are supported. Conflicting nonempty types abort that scan. Earlier scans may already have committed when a later scan fails. Rerunning is safe.

The command updates only empty `manifest_dependencies.package_type` values. It preserves observation IDs, timestamps, lifecycle state, labels, and artifacts. It does not replay uploads. Missing types in the original report stay empty.
