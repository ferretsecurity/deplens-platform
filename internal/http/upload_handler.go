package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
)

func NewRouter(service UploadService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/scans", func(w http.ResponseWriter, r *http.Request) {
		token := auth.BearerToken(r.Header.Get("Authorization"))
		if token == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		input, err := decodeUploadRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		scanID, err := service.Upload(r, token, input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"scan_id": scanID})
	})
	return mux
}

func decodeUploadRequest(r *http.Request) (scans.UploadRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return scans.UploadRequest{}, errors.New("invalid json body")
	}

	var wrapped scans.UploadRequest
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&wrapped); err == nil && wrapped.SchemaVersion != "" {
		if err := validateWrappedUploadRequest(wrapped); err != nil {
			return scans.UploadRequest{}, err
		}
		return wrapped, nil
	}

	var raw scans.RawSnapshot
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&raw); err != nil {
		return scans.UploadRequest{}, errors.New("invalid json body")
	}
	if raw.Root == "" {
		return scans.UploadRequest{}, errors.New("invalid json body")
	}

	headers, err := rawUploadHeaders(r.Header)
	if err != nil {
		return scans.UploadRequest{}, err
	}

	return scans.UploadRequest{
		SchemaVersion: "v1alpha1",
		Repository: scans.RepositoryInput{
			Name:          headers.repositoryName,
			URL:           headers.repositoryURL,
			DefaultBranch: headers.defaultBranch,
		},
		Source: scans.SourceInput{
			CommitSHA: headers.commitSHA,
			Ref:       headers.ref,
			ScannedAt: headers.scannedAt,
		},
		Snapshot: scans.SnapshotInput{
			Root:      raw.Root,
			Manifests: raw.Manifests,
			Warnings:  raw.Warnings,
		},
	}, nil
}

func validateWrappedUploadRequest(input scans.UploadRequest) error {
	switch {
	case strings.TrimSpace(input.Repository.Name) == "":
		return errors.New("repository.name is required")
	case strings.TrimSpace(input.Repository.URL) == "":
		return errors.New("repository.url is required")
	case strings.TrimSpace(input.Repository.DefaultBranch) == "":
		return errors.New("repository.default_branch is required")
	case strings.TrimSpace(input.Source.CommitSHA) == "":
		return errors.New("source.commit_sha is required")
	case strings.TrimSpace(input.Source.Ref) == "":
		return errors.New("source.ref is required")
	case strings.TrimSpace(input.Source.ScannedAt) == "":
		return errors.New("source.scanned_at is required")
	case strings.TrimSpace(input.Snapshot.Root) == "":
		return errors.New("snapshot.root is required")
	default:
		return nil
	}
}

type rawUploadMetadata struct {
	repositoryName string
	repositoryURL  string
	defaultBranch  string
	commitSHA      string
	ref            string
	scannedAt      string
}

func rawUploadHeaders(header http.Header) (rawUploadMetadata, error) {
	metadata := rawUploadMetadata{
		repositoryName: strings.TrimSpace(header.Get("X-Deplens-Repository-Name")),
		repositoryURL:  strings.TrimSpace(header.Get("X-Deplens-Repository-URL")),
		defaultBranch:  strings.TrimSpace(header.Get("X-Deplens-Default-Branch")),
		commitSHA:      strings.TrimSpace(header.Get("X-Deplens-Commit-SHA")),
		ref:            strings.TrimSpace(header.Get("X-Deplens-Ref")),
		scannedAt:      strings.TrimSpace(header.Get("X-Deplens-Scanned-At")),
	}

	for _, required := range []struct {
		name  string
		value string
	}{
		{"X-Deplens-Repository-Name", metadata.repositoryName},
		{"X-Deplens-Repository-URL", metadata.repositoryURL},
		{"X-Deplens-Default-Branch", metadata.defaultBranch},
		{"X-Deplens-Commit-SHA", metadata.commitSHA},
		{"X-Deplens-Ref", metadata.ref},
		{"X-Deplens-Scanned-At", metadata.scannedAt},
	} {
		if required.value == "" {
			return rawUploadMetadata{}, errors.New(required.name + " is required for raw deplens uploads")
		}
	}

	return metadata, nil
}
