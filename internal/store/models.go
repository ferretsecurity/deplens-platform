package store

import "time"

type TokenRecord struct {
	TenantID string
	Scopes   []string
}

type APITokenMetadata struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
}

type MembershipRecord struct {
	TenantID   string `json:"tenant_id"`
	TenantSlug string `json:"tenant_slug"`
	Role       string `json:"role"`
}

// UploadScanParams carries repository-only scan upload metadata.
type UploadScanParams struct {
	TenantID            string
	RepositoryName      string
	URL                 string
	DefaultBranch       string
	ArtifactKey         string
	ArtifactSHA256      string
	SchemaVersion       string
	RootPath            string
	CommitSHA           string
	SourceRef           string
	ScannedAt           time.Time
	ManifestCount       int
	WithDependencies    int
	WithoutDependencies int
	UnknownDependencies int
	DependencyCount     int
	Labels              map[string]string
	Annotation          string
	Manifests           []UploadManifestParams
}

type UploadManifestParams struct {
	Position        int
	Type            string
	Path            string
	HasDependencies *bool
	Warnings        []string
	Dependencies    []UploadDependencyParams
}

type UploadDependencyParams struct {
	Position   int
	Raw        string
	Name       string
	Version    string
	Constraint string
	Section    string
	Source     string
	Extras     map[string]string
}

type ScanManifestItem struct {
	ID              string                   `json:"id"`
	Type            string                   `json:"type"`
	Path            string                   `json:"path"`
	HasDependencies *bool                    `json:"has_dependencies"`
	Warnings        []string                 `json:"warnings"`
	Dependencies    []ManifestDependencyItem `json:"dependencies"`
}

type ManifestDependencyItem struct {
	ID         string            `json:"id"`
	Raw        string            `json:"raw"`
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Constraint string            `json:"constraint"`
	Section    string            `json:"section"`
	Source     string            `json:"source"`
	Extras     map[string]string `json:"extras"`
}

type DependencyListItem struct {
	Raw               string `json:"raw"`
	Name              string `json:"name"`
	OccurrenceCount   int    `json:"occurrence_count"`
	RepositoryCount   int    `json:"repository_count"`
	ManifestFileCount int    `json:"manifest_file_count"`
	LockFileCount     int    `json:"lock_file_count"`
}
