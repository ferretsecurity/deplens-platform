package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"

	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	apply := flag.Bool("apply", false, "fill empty package types; default is dry-run")
	tenant := flag.String("tenant-id", "", "limit to one tenant")
	scan := flag.String("scan-id", "", "limit to one scan")
	flag.Parse()
	if err := run(context.Background(), os.Getenv("DATABASE_URL"), os.Getenv("BLOB_FILESYSTEM_ROOT"), *tenant, *scan, *apply, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, databaseURL, root, tenantID, scanID string, apply bool, out io.Writer) error {
	if databaseURL == "" || root == "" {
		return fmt.Errorf("DATABASE_URL and BLOB_FILESYSTEM_ROOT are required")
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query(ctx, `select s.id::text from scans s
 where ($1 = '' or s.tenant_id::text = $1) and ($2 = '' or s.id::text = $2)
 and ($2 <> '' or exists (select 1 from scan_manifests sm join manifest_dependencies d on d.scan_manifest_id = sm.id where sm.scan_id = s.id and d.package_type = ''))
 order by s.id`, tenantID, scanID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if scanID != "" && len(ids) == 0 {
		return fmt.Errorf("scan not found in scope")
	}
	for _, id := range ids {
		if err := repairScan(ctx, db, blob.NewFileSystemStore(root), id, apply, out); err != nil {
			return fmt.Errorf("scan %s: %w", id, err)
		}
	}
	return nil
}

type typeUpdate struct{ id, packageType string }

func repairScan(ctx context.Context, db *pgxpool.Pool, artifacts *blob.FileSystemStore, scanID string, apply bool, out io.Writer) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedRepositoryID string
	err = tx.QueryRow(ctx, `select r.id from repositories r join scans s on s.repository_id = r.id and s.tenant_id = r.tenant_id where s.id = $1 for update of r`, scanID).Scan(&lockedRepositoryID)
	if err != nil {
		return err
	}
	var key, savedSHA string
	if err := tx.QueryRow(ctx, `select artifact_key, artifact_sha256 from scans where id = $1 and repository_id = $2`, scanID, lockedRepositoryID).Scan(&key, &savedSHA); err != nil {
		return err
	}
	payload, err := artifacts.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != savedSHA {
		return fmt.Errorf("artifact SHA mismatch")
	}
	var envelope struct {
		Report json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return err
	}
	if len(envelope.Report) == 0 || string(envelope.Report) == "null" {
		fmt.Fprintf(out, "scan %s skipped: legacy artifact without native report\n", scanID)
		return nil
	}
	var header struct {
		SchemaVersion json.RawMessage `json:"schema_version"`
	}
	if err := json.Unmarshal(envelope.Report, &header); err != nil {
		return err
	}
	if string(header.SchemaVersion) != "1" {
		fmt.Fprintf(out, "scan %s skipped: unsupported native report schema %s\n", scanID, header.SchemaVersion)
		return nil
	}
	snapshot, err := scans.SnapshotFromReport(envelope.Report)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `select sm.id::text, sm.position, sm.type, m.path from scan_manifests sm join manifests m on m.id = sm.manifest_id where sm.scan_id = $1 order by sm.position`, scanID)
	if err != nil {
		return err
	}
	type retainedManifest struct {
		id             string
		position       int
		detector, path string
	}
	var retained []retainedManifest
	for rows.Next() {
		var m retainedManifest
		if err := rows.Scan(&m.id, &m.position, &m.detector, &m.path); err != nil {
			rows.Close()
			return err
		}
		retained = append(retained, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var updates []typeUpdate
	for _, m := range retained {
		if m.position < 0 || m.position >= len(snapshot.Manifests) {
			return fmt.Errorf("manifest position %d outside report", m.position)
		}
		original := snapshot.Manifests[m.position]
		if m.path != original.Path || m.detector != original.Type {
			return fmt.Errorf("manifest %d path/detector mismatch", m.position)
		}
		deps, err := tx.Query(ctx, `select id::text, position, raw, name, version, "constraint", section, source, extras, package_type from manifest_dependencies where scan_manifest_id = $1 order by position`, m.id)
		if err != nil {
			return err
		}
		count := 0
		for deps.Next() {
			var id string
			var position int
			var saved scans.DependencyInput
			var extras []byte
			if err := deps.Scan(&id, &position, &saved.Raw, &saved.Name, &saved.Version, &saved.Constraint, &saved.Section, &saved.Source, &extras, &saved.PackageType); err != nil {
				deps.Close()
				return err
			}
			if err := json.Unmarshal(extras, &saved.Extras); err != nil {
				deps.Close()
				return err
			}
			if position != count || position >= len(original.Dependencies) {
				deps.Close()
				return fmt.Errorf("manifest %d dependency position mismatch", m.position)
			}
			expected := original.Dependencies[position]
			if saved.Raw != expected.Raw || saved.Name != expected.Name || saved.Version != expected.Version || saved.Constraint != expected.Constraint || saved.Section != expected.Section || saved.Source != expected.Source || !maps.Equal(saved.Extras, expected.Extras) {
				deps.Close()
				return fmt.Errorf("manifest %d dependency %d mismatch", m.position, position)
			}
			if saved.PackageType != "" && saved.PackageType != expected.PackageType {
				deps.Close()
				return fmt.Errorf("manifest %d dependency %d package type conflict", m.position, position)
			}
			if saved.PackageType == "" && expected.PackageType != "" {
				updates = append(updates, typeUpdate{id, expected.PackageType})
			}
			count++
		}
		err = deps.Err()
		deps.Close()
		if err != nil {
			return err
		}
		if count != len(original.Dependencies) {
			return fmt.Errorf("manifest %d dependency count mismatch", m.position)
		}
	}
	if apply {
		for _, update := range updates {
			tag, err := tx.Exec(ctx, `update manifest_dependencies d set package_type = $2 from scan_manifests sm where d.id = $1 and d.package_type = '' and sm.id = d.scan_manifest_id and sm.scan_id = $3`, update.id, update.packageType, scanID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("dependency %s changed during repair", update.id)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "scan %s apply=%t package_types=%d retained_manifests=%d\n", scanID, apply, len(updates), len(retained))
	return nil
}
