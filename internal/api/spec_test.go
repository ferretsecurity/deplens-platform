package api

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestOpenAPIDefinesScanUploadAndHistoryPaths(t *testing.T) {
	specBytes, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	spec := string(specBytes)
	for _, want := range []string{
		"/api/v1/scans:",
		"/api/v1/scans/{scan_id}:",
		"/api/v1/scans/{scan_id}/manifests:",
		"/api/v1/scans/{scan_id}/metadata:",
		"/api/v1/repositories:",
		"/api/v1/tokens:",
	} {
		if !strings.Contains(spec, want) {
			t.Fatalf("OpenAPI spec missing path %q", want)
		}
	}
	if !strings.Contains(spec, "ScanUploadRequest") {
		t.Fatal("OpenAPI spec missing ScanUploadRequest schema")
	}
}

func TestOpenAPISpecDoesNotExposeProjects(t *testing.T) {
	data, err := os.ReadFile("../../api/openapi.yaml")
	require.NoError(t, err)
	require.NotContains(t, string(data), "/api/v1/projects")
	require.NotContains(t, string(data), "X-Deplens-Project-Slug")
	require.NotContains(t, string(data), "X-Deplens-Project-Name")
	require.NotContains(t, string(data), "\"project\"")
}

func TestOpenAPISpecDoesNotExposeRepositorySlugContract(t *testing.T) {
	data, err := os.ReadFile("../../api/openapi.yaml")
	require.NoError(t, err)

	spec := string(data)
	require.NotContains(t, spec, "repository_slug")
	require.NotContains(t, spec, "X-Deplens-Repository-Slug")
	require.Contains(t, spec, "repository_id")

	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string `yaml:"required"`
				Properties map[string]struct {
					Format string `yaml:"format"`
				} `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
		Paths map[string]struct {
			Get struct {
				Parameters []struct {
					Name   string `yaml:"name"`
					Schema struct {
						Format string `yaml:"format"`
					} `yaml:"schema"`
				} `yaml:"parameters"`
				Responses map[string]struct {
					Content map[string]struct {
						Schema struct {
							Ref   string `yaml:"$ref"`
							Items struct {
								Ref string `yaml:"$ref"`
							} `yaml:"items"`
						} `yaml:"schema"`
					} `yaml:"content"`
				} `yaml:"responses"`
			} `yaml:"get"`
			Post struct {
				Parameters []struct {
					Name        string `yaml:"name"`
					Required    bool   `yaml:"required"`
					Description string `yaml:"description"`
				} `yaml:"parameters"`
			} `yaml:"post"`
		} `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))

	require.Equal(t, []string{"name", "url", "default_branch"}, doc.Components.Schemas["RepositoryInput"].Required)
	require.Len(t, doc.Paths["/api/v1/scans"].Get.Parameters, 3)
	require.Equal(t, "repository_id", doc.Paths["/api/v1/scans"].Get.Parameters[0].Name)
	require.Equal(t, "uuid", doc.Paths["/api/v1/scans"].Get.Parameters[0].Schema.Format)
	require.Equal(t, "uuid", doc.Components.Schemas["RepositorySummary"].Properties["id"].Format)
	require.Equal(t, "uuid", doc.Components.Schemas["ScanSummary"].Properties["id"].Format)
	require.Equal(t, "uuid", doc.Components.Schemas["ScanSummary"].Properties["repository_id"].Format)
	require.Equal(t, "#/components/schemas/ScanSummary", doc.Paths["/api/v1/scans/{scan_id}"].Get.Responses["200"].Content["application/json"].Schema.Ref)
	require.Equal(t, "#/components/schemas/ScanSummary", doc.Paths["/api/v1/scans"].Get.Responses["200"].Content["application/json"].Schema.Items.Ref)
	require.Equal(t, "#/components/schemas/RepositorySummary", doc.Paths["/api/v1/repositories"].Get.Responses["200"].Content["application/json"].Schema.Items.Ref)

	rawHeaderNames := []string{
		"X-Deplens-Repository-Name",
		"X-Deplens-Repository-URL",
		"X-Deplens-Default-Branch",
		"X-Deplens-Commit-SHA",
		"X-Deplens-Ref",
		"X-Deplens-Scanned-At",
	}
	headers := make(map[string]struct {
		required    bool
		description string
	}, len(doc.Paths["/api/v1/scans"].Post.Parameters))
	for _, parameter := range doc.Paths["/api/v1/scans"].Post.Parameters {
		headers[parameter.Name] = struct {
			required    bool
			description string
		}{
			required:    parameter.Required,
			description: parameter.Description,
		}
	}

	for _, header := range rawHeaderNames {
		got, ok := headers[header]
		require.True(t, ok, "missing raw upload header %s", header)
		require.False(t, got.required, "header %s must remain optional at the top-level POST contract", header)
		require.Equal(t, "Required when uploading raw deplens JSON.", got.description)
	}
}
