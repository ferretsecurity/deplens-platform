package scans

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotFromReportPreservesPackageType(t *testing.T) {
	report := json.RawMessage(`{"schema_version":1,"root":"/repo","sources":[{"detector":"python-pyproject","path":"pyproject.toml","analysis":{"presence":"present","extraction":"complete"},"dependencies":[{"name":"litellm","package_type":"pypi","raw":"litellm>=1.103.0,<2","source_group":"project.dependencies","version_constraint":">=1.103.0,<2"}]}],"check_runs":[],"findings":[]}`)
	snapshot, err := SnapshotFromReport(report)
	require.NoError(t, err)
	payload, err := json.Marshal(snapshot.Manifests[0].Dependencies[0])
	require.NoError(t, err)
	var dependency struct {
		PackageType string `json:"package_type"`
	}
	require.NoError(t, json.Unmarshal(payload, &dependency))
	require.Equal(t, "pypi", dependency.PackageType)
}
