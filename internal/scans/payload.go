package scans

import "encoding/json"

type UploadRequest struct {
	Report        json.RawMessage   `json:"report,omitempty"`
	SchemaVersion string            `json:"schema_version"`
	Repository    RepositoryInput   `json:"repository"`
	Source        SourceInput       `json:"source"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotation    string            `json:"annotation,omitempty"`
	Snapshot      SnapshotInput     `json:"snapshot"`
}

type RawSnapshot struct {
	Root      string          `json:"root"`
	Manifests []ManifestInput `json:"manifests"`
	Warnings  []string        `json:"warnings,omitempty"`
}

type RepositoryInput struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}

type SourceInput struct {
	CommitSHA string `json:"commit_sha"`
	Ref       string `json:"ref"`
	ScannedAt string `json:"scanned_at"`
}

type SnapshotInput struct {
	Root      string          `json:"root"`
	Manifests []ManifestInput `json:"manifests"`
	Warnings  []string        `json:"warnings,omitempty"`
}

type ManifestInput struct {
	Form            string            `json:"form,omitempty"`
	Extraction      string            `json:"extraction,omitempty"`
	Lockfiles       []string          `json:"lockfiles"`
	Type            string            `json:"type"`
	Path            string            `json:"path"`
	HasDependencies *bool             `json:"has_dependencies"`
	Dependencies    []DependencyInput `json:"dependencies,omitempty"`
	Warnings        []string          `json:"warnings,omitempty"`
}

type DependencyInput struct {
	PackageType string            `json:"package_type,omitempty"`
	Raw         string            `json:"raw"`
	Name        string            `json:"name,omitempty"`
	Version     string            `json:"version,omitempty"`
	Constraint  string            `json:"constraint,omitempty"`
	Section     string            `json:"section,omitempty"`
	Source      string            `json:"source,omitempty"`
	Extras      map[string]string `json:"extras,omitempty"`
}
