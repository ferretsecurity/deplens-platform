package api

import (
	"os"
	"strings"
	"testing"
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
		"/api/v1/projects:",
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
