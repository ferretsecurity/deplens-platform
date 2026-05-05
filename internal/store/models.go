package store

import "time"

type TokenRecord struct {
	TenantID string
	Scopes   []string
}

type MembershipRecord struct {
	TenantID   string `json:"tenant_id"`
	TenantSlug string `json:"tenant_slug"`
	Role       string `json:"role"`
}

type UploadScanParams struct {
	TenantID            string
	ProjectSlug         string
	ProjectName         string
	RepositorySlug      string
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
}
