package scans

import (
	"encoding/json"
	"fmt"
)

type scannerReport struct {
	SchemaVersion int             `json:"schema_version"`
	Root          string          `json:"root"`
	Sources       []scannerSource `json:"sources"`
}

type scannerSource struct {
	Detector string `json:"detector"`
	Path     string `json:"path"`
	Analysis struct {
		Presence   string `json:"presence"`
		Extraction string `json:"extraction"`
	} `json:"analysis"`
	Dependencies []struct {
		PackageType       string            `json:"package_type"`
		Raw               string            `json:"raw"`
		Name              string            `json:"name"`
		Version           string            `json:"version"`
		VersionConstraint string            `json:"version_constraint"`
		SourceGroup       string            `json:"source_group"`
		OriginKind        string            `json:"origin_kind"`
		Attributes        map[string]string `json:"attributes"`
	} `json:"dependencies"`
	Diagnostics []struct {
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	} `json:"diagnostics"`
}

func SnapshotFromReport(payload json.RawMessage) (SnapshotInput, error) {
	var report scannerReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return SnapshotInput{}, fmt.Errorf("invalid scanner report: %w", err)
	}
	if report.SchemaVersion != 1 {
		return SnapshotInput{}, fmt.Errorf("unsupported scanner schema_version %d", report.SchemaVersion)
	}
	if report.Root == "" {
		return SnapshotInput{}, fmt.Errorf("root is required")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return SnapshotInput{}, err
	}
	for _, name := range []string{"sources", "check_runs", "findings"} {
		value, ok := fields[name]
		if !ok {
			return SnapshotInput{}, fmt.Errorf("%s is required for scanner reports", name)
		}
		var collection []json.RawMessage
		if err := json.Unmarshal(value, &collection); err != nil {
			return SnapshotInput{}, fmt.Errorf("%s must be an array", name)
		}
	}

	snapshot := SnapshotInput{Root: report.Root, Manifests: make([]ManifestInput, 0, len(report.Sources))}
	for _, source := range report.Sources {
		if source.Detector == "" || source.Path == "" {
			return SnapshotInput{}, fmt.Errorf("source detector and path are required")
		}
		manifest := ManifestInput{Type: source.Detector, Path: source.Path, Warnings: []string{}, Dependencies: []DependencyInput{}}
		switch source.Analysis.Presence {
		case "present":
			present := true
			manifest.HasDependencies = &present
		case "absent":
			present := false
			manifest.HasDependencies = &present
		case "unknown":
		default:
			return SnapshotInput{}, fmt.Errorf("invalid dependency presence %q", source.Analysis.Presence)
		}
		switch source.Analysis.Extraction {
		case "unsupported", "complete", "partial", "failed":
		default:
			return SnapshotInput{}, fmt.Errorf("invalid extraction state %q", source.Analysis.Extraction)
		}
		for _, diagnostic := range source.Diagnostics {
			manifest.Warnings = append(manifest.Warnings, diagnostic.Severity+" ["+diagnostic.Code+"]: "+diagnostic.Message)
		}
		for _, dependency := range source.Dependencies {
			manifest.Dependencies = append(manifest.Dependencies, DependencyInput{
				PackageType: dependency.PackageType, Raw: dependency.Raw, Name: dependency.Name, Version: dependency.Version,
				Constraint: dependency.VersionConstraint, Section: dependency.SourceGroup,
				Source: dependency.OriginKind, Extras: dependency.Attributes,
			})
		}
		snapshot.Manifests = append(snapshot.Manifests, manifest)
	}
	return snapshot, nil
}
