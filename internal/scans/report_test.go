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

func TestSnapshotFromReportKeepsOpenPackageTypesAndIndependentAttributes(t *testing.T) {
	for _, packageType := range []string{"", "unknown", "future:ecosystem", "pypi", "pip"} {
		t.Run(packageType, func(t *testing.T) {
			report := json.RawMessage(`{"schema_version":1,"root":"/repo","sources":[{"detector":"mixed","path":"manifest","analysis":{"presence":"present","extraction":"complete"},"dependencies":[{"name":"same","package_type":"` + packageType + `","attributes":{"package_type":"attribute-value"}},{"name":"same","attributes":{"package_type":"not-an-ecosystem"}}]}],"check_runs":[],"findings":[]}`)
			snapshot, err := SnapshotFromReport(report)
			require.NoError(t, err)
			deps := snapshot.Manifests[0].Dependencies
			require.Equal(t, packageType, deps[0].PackageType)
			require.Equal(t, "attribute-value", deps[0].Extras["package_type"])
			require.Empty(t, deps[1].PackageType)
			require.Equal(t, "not-an-ecosystem", deps[1].Extras["package_type"])
		})
	}
}
