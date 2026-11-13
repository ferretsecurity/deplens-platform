package assessment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssessmentRules(t *testing.T) {
	q, err := NewQuery("npm", "widget", ">0.0.0 <=2.3.4")
	require.NoError(t, err)
	manifest := func(constraint string, locks ...string) Source {
		return Source{Path: "package.json", Form: "manifest", Extraction: "complete", Lockfiles: locks, Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Constraint: constraint}}}
	}
	lock := func(file, version, extraction string) Source {
		return Source{Path: file, Form: "lockfile", Extraction: extraction, Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Version: version}}}
	}
	for _, tc := range []struct {
		name     string
		sources  []Source
		statuses []string
	}{
		{"unlocked overlap", []Source{manifest("^2.3.0")}, []string{"warning"}},
		{"earlier versions overlap", []Source{manifest("<2.3.4")}, []string{"warning"}},
		{"fixed declaration", []Source{manifest(">=2.3.5")}, nil},
		{"fixed lock suppresses warning", []Source{manifest("^2.3.0"), lock("package-lock.json", "2.3.5", "complete")}, nil},
		{"affected lock", []Source{manifest("^2.3.0"), lock("package-lock.json", "2.3.3", "complete")}, []string{"issue"}},
		{"competing locks", []Source{manifest("^2.3.0"), lock("package-lock.json", "2.3.5", "complete"), lock("yarn.lock", "2.3.3", "complete")}, []string{"issue"}},
		{"both fixed", []Source{manifest("^2.3.0"), lock("package-lock.json", "2.3.5", "complete"), lock("yarn.lock", "2.4.0", "complete")}, nil},
		{"partial fixed", []Source{manifest("^2.3.0"), lock("package-lock.json", "2.3.5", "partial")}, []string{"warning"}},
		{"partial affected", []Source{manifest("^2.3.0"), lock("package-lock.json", "2.3.3", "partial")}, []string{"issue", "warning"}},
		{"legacy extraction unknown", []Source{manifest("^2.3.0"), lock("package-lock.json", "2.3.5", "")}, []string{"warning"}},
		{"failed lock", []Source{manifest("^2.3.0"), {Path: "package-lock.json", Form: "lockfile", Extraction: "failed"}}, []string{"warning"}},
		{"missing resolution", []Source{manifest("^2.3.0"), {Path: "package-lock.json", Form: "lockfile", Extraction: "complete"}}, []string{"warning"}},
		{"incompatible resolution", []Source{manifest("~2.3.0"), lock("package-lock.json", "3.0.0", "complete")}, []string{"warning"}},
		{"missing mapped file", []Source{manifest("^2.3.0", "missing/pnpm-lock.yaml")}, []string{"warning"}},
		{"unrelated file", []Source{manifest("^2.3.0"), lock("other/package-lock.json", "2.3.5", "complete")}, []string{"warning"}},
		{"unknown constraint", []Source{manifest("latest")}, []string{"unknown"}},
		{"missing constraint", []Source{manifest("")}, []string{"unknown"}},
		{"orphan lock", []Source{lock("pnpm-lock.yaml", "2.3.1", "complete")}, []string{"issue"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings := q.Assess(tc.sources)
			var got []string
			for _, f := range findings {
				got = append(got, f.Evidence.Status)
			}
			require.Equal(t, tc.statuses, got)
		})
	}
}

func TestWorkspaceEvidenceIsNotAssignedToEveryMember(t *testing.T) {
	q, err := NewQuery("npm", "widget", "<=2.3.4")
	require.NoError(t, err)
	sources := []Source{
		{Path: "apps/a/package.json", Form: "manifest", Lockfiles: []string{"pnpm-lock.yaml"}, Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Constraint: "^2.3.0"}}},
		{Path: "apps/b/package.json", Form: "manifest", Lockfiles: []string{"pnpm-lock.yaml"}},
		{Path: "pnpm-lock.yaml", Form: "lockfile", Extraction: "complete", Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Version: "2.3.3"}}},
	}
	findings := q.Assess(sources)
	require.Len(t, findings, 2)
	require.Equal(t, []string{"apps/a/package.json"}, findings[0].Evidence.Manifests)
	require.Equal(t, "warning", findings[1].Evidence.Status)
	require.Equal(t, "workspace", findings[0].Evidence.Scope)
	require.Equal(t, ".", findings[0].Evidence.Root)
	require.Equal(t, "pnpm-lock.yaml", findings[0].Path)
}

func TestPackageIdentityAndRanges(t *testing.T) {
	for _, tc := range []struct {
		ecosystem, name, dependency, affected, constraint string
		want                                              bool
	}{
		{"npm", "widget", "widget", "<=2.3.4", "<2.3.4", true},
		{"npm", "widget", "widget", "<=2.3.4", ">=2.3.5", false},
		{"npm", "widget", "widget-extra", "<=2.3.4", "*", false},
		{"pypi", "My_Package", "my.package", ">0,<=2.3.4", ">=2.0,<3,!=2.3.4", true},
		{"pypi", "widget", "widget", "==2.3.4", "!=2.3.4", false},
		{"cargo", "widget", "widget", "<=2.3.4", "^2.3.5", false},
		{"composer", "widget", "widget", "<=2.3.4", "^2.0", true},
		{"gem", "widget", "widget", "<=2.3.4", "~> 2.3", true},
	} {
		t.Run(tc.ecosystem+tc.constraint+tc.dependency, func(t *testing.T) {
			q, err := NewQuery(tc.ecosystem, tc.name, tc.affected)
			require.NoError(t, err)
			findings := q.Assess([]Source{{Path: "manifest", Form: "manifest", Dependencies: []Dependency{{Name: tc.dependency, Ecosystem: tc.ecosystem, Constraint: tc.constraint}}}})
			require.Equal(t, tc.want, len(findings) > 0)
		})
	}
	for _, value := range []string{"", "latest", "broken", ">=3 <2"} {
		_, err := NewQuery("npm", "widget", value)
		require.Error(t, err)
	}
}

func TestNativeRangeSemantics(t *testing.T) {
	for _, tc := range []struct {
		ecosystem, affected, declaration string
		want                             bool
	}{
		{"pypi", "==1.0", "==1.0+linux", true},
		{"pypi", "==1.0+linux", "==1.0", true},
		{"pypi", "==1.0", "==1.0,!=1.0", false},
		{"pypi", "==1.0", ">1.0", false},
		{"npm", ">=1.0.0 <2.0.0", ">=1.5.0-alpha <1.5.0", false},
		{"npm", ">=1.5.0-alpha <1.5.0", ">1.5.0-beta <1.5.0", true},
		{"npm", ">1.5.0-beta <1.5.0-beta.0", "*", false},
		{"npm", "<=1.5.0-beta.0", ">1.5.0-beta <=1.5.0-beta.0", true},
		{"npm", ">1.0.0 <1.0.1", "*", false},
		{"cargo", ">=1.0.0 <2.0.0", ">=1.5.0-alpha <1.5.0", false},
	} {
		t.Run(tc.ecosystem+tc.affected+tc.declaration, func(t *testing.T) {
			q, err := NewQuery(tc.ecosystem, "widget", tc.affected)
			if tc.affected == ">1.0.0 <1.0.1" || tc.affected == ">1.5.0-beta <1.5.0-beta.0" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, err := q.overlaps(tc.declaration)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
	for _, tc := range []struct{ ecosystem, value string }{
		{"pypi", "==1.0,!=1.0"}, {"npm", ">1.0.0 <1.0.1"},
	} {
		_, err := NewQuery(tc.ecosystem, "widget", tc.value)
		require.Error(t, err)
	}
}

func TestNonregistryResolutionsDoNotEstablishRegistrySafety(t *testing.T) {
	q, err := NewQuery("npm", "widget", "<=2.3.4")
	require.NoError(t, err)
	for _, origin := range []string{"git", "path", "url", "workspace"} {
		t.Run(origin, func(t *testing.T) {
			dependency := Dependency{Name: "widget", Ecosystem: "npm", Version: "2.3.5", Origin: origin}
			findings := q.Assess([]Source{
				{Path: "package.json", Form: "manifest", Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Constraint: "^2.3.0"}}},
				{Path: "package-lock.json", Form: "lockfile", Extraction: "complete", Dependencies: []Dependency{dependency}},
			})
			require.Len(t, findings, 2)
			require.Equal(t, "unknown", findings[0].Evidence.Status)
			require.Equal(t, "warning", findings[1].Evidence.Status)
			observation := q.AssessObservation(dependency, "package.json")
			require.NotNil(t, observation)
			require.Equal(t, "unknown", observation.Evidence.Status)
		})
	}
}

func TestSharedFixedLockDoesNotProveMemberResolution(t *testing.T) {
	q, err := NewQuery("npm", "widget", "<=2.3.4")
	require.NoError(t, err)
	findings := q.Assess([]Source{
		{Path: "package.json", Form: "manifest", Lockfiles: []string{"pnpm-lock.yaml"}, Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Constraint: "^2.3.0"}}},
		{Path: "apps/a/package.json", Form: "manifest", Lockfiles: []string{"pnpm-lock.yaml"}, Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Constraint: "^2.3.0"}}},
		{Path: "pnpm-lock.yaml", Form: "lockfile", Extraction: "complete", Dependencies: []Dependency{{Name: "widget", Ecosystem: "npm", Version: "2.3.5"}}},
	})
	require.Len(t, findings, 2)
	for _, finding := range findings {
		require.Equal(t, "warning", finding.Evidence.Status)
		require.Contains(t, finding.Evidence.Reasons[0], "cannot be attributed")
	}
}

func TestPinnedDeclarationsAndProviderVersionsAcrossEcosystems(t *testing.T) {
	for _, tc := range []struct {
		ecosystem, affected, version, fixed string
	}{
		{"npm", "<=2.3.4", "2.3.4", "2.3.5"},
		{"pypi", "<=2.3.4", "2.3.4", "2.3.5"},
		{"cargo", "<=2.3.4", "2.3.4", "2.3.5"},
		{"go", "<=v2.3.4", "v2.3.4", "v2.3.5"},
		{"maven", "(,2.3.4]", "2.3.4", "2.3.5"},
		{"nuget", "(,2.3.4]", "2.3.4", "2.3.5"},
		{"gem", "<=2.3.4", "2.3.4", "2.3.5"},
		{"composer", "<=2.3.4", "2.3.4", "2.3.5"},
	} {
		t.Run(tc.ecosystem, func(t *testing.T) {
			q, err := NewQuery(tc.ecosystem, "widget", tc.affected)
			require.NoError(t, err)
			dependency := Dependency{Name: "widget", Ecosystem: tc.ecosystem, Version: tc.version, Origin: "registry"}
			findings := q.Assess([]Source{{Path: "manifest", Form: "manifest", Dependencies: []Dependency{dependency}}})
			require.Len(t, findings, 1)
			require.Equal(t, "warning", findings[0].Evidence.Status)
			finding := q.AssessObservation(dependency, "manifest")
			require.NotNil(t, finding)
			require.Equal(t, "issue", finding.Evidence.Status)
			dependency.Version = tc.fixed
			require.Nil(t, q.AssessObservation(dependency, "manifest"))
			require.Empty(t, q.Assess([]Source{{Path: "manifest", Form: "manifest", Dependencies: []Dependency{dependency}}}))
		})
	}
}

func TestUnresolvedPythonLocalOverlapIsUnknown(t *testing.T) {
	q, err := NewQuery("pypi", "widget", "==1.0")
	require.NoError(t, err)
	findings := q.Assess([]Source{{Path: "pyproject.toml", Form: "manifest", Dependencies: []Dependency{
		{Name: "widget", Ecosystem: "pypi", Constraint: ">1.0+linux,<1.0+z"},
	}}})
	require.Len(t, findings, 1)
	require.Equal(t, "unknown", findings[0].Evidence.Status)
}

func TestInventorySourcesAreNotDependencyDeclarations(t *testing.T) {
	q, err := NewQuery("go", "example.com/widget", "<=v2.3.4")
	require.NoError(t, err)
	for _, form := range []string{"checksum-file", "source-code", "tool-config", ""} {
		t.Run(form, func(t *testing.T) {
			findings := q.Assess([]Source{{Path: "go.sum", Form: form, Extraction: "complete", Lockfiles: []string{"irrelevant.lock"},
				Dependencies: []Dependency{{Name: "example.com/widget", Ecosystem: "go", Version: "v2.3.4"}}}})
			require.Len(t, findings, 1)
			require.Equal(t, "unknown", findings[0].Evidence.Status)
			require.Empty(t, findings[0].Evidence.Manifests)
			require.Empty(t, findings[0].Evidence.Lockfiles)
		})
	}
}

func TestLocalBuildDefinitionLockfilesSupersedeConstraints(t *testing.T) {
	for _, tc := range []struct{ manifest, lockfile, ecosystem, name, affected, constraint string }{
		{"service/build.gradle", "service/gradle.lockfile", "maven", "org:widget", "(,2.3.4]", "[2.0,3.0)"},
		{"service/build.gradle.kts", "service/gradle.lockfile", "maven", "org:widget", "(,2.3.4]", "[2.0,3.0)"},
		{"service/app.csproj", "service/packages.lock.json", "nuget", "Widget", "(,2.3.4]", "[2.0,3.0)"},
		{"service/app.fsproj", "service/packages.lock.json", "nuget", "Widget", "(,2.3.4]", "[2.0,3.0)"},
		{"service/app.vbproj", "service/packages.lock.json", "nuget", "Widget", "(,2.3.4]", "[2.0,3.0)"},
	} {
		t.Run(tc.manifest, func(t *testing.T) {
			q, err := NewQuery(tc.ecosystem, tc.name, tc.affected)
			require.NoError(t, err)
			findings := q.Assess([]Source{
				{Path: tc.manifest, Form: "build-definition", Dependencies: []Dependency{{Name: tc.name, Ecosystem: tc.ecosystem, Constraint: tc.constraint}}},
				{Path: tc.lockfile, Form: "lockfile", Extraction: "complete", Dependencies: []Dependency{{Name: tc.name, Ecosystem: tc.ecosystem, Version: "2.3.5"}}},
			})
			require.Empty(t, findings)
		})
	}
}
