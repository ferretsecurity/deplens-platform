package socket

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

var ErrProcessing = errors.New("Socket scan is still processing; try syncing later")

const maxResponse = 128 << 20

type Repository struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Head         string `json:"head_full_scan_id"`
	DashboardURL string `json:"html_url"`
	Branch       string `json:"default_branch"`
	Workspace    string `json:"workspace"`
}
type Reference struct {
	File  string          `json:"file"`
	Start json.RawMessage `json:"start,omitempty"`
	End   json.RawMessage `json:"end,omitempty"`
}
type Artifact struct {
	ID         string      `json:"id"`
	Ecosystem  string      `json:"type"`
	Name       string      `json:"name"`
	Namespace  string      `json:"namespace"`
	Version    string      `json:"version"`
	Direct     *bool       `json:"direct"`
	Dev        *bool       `json:"dev"`
	References []Reference `json:"manifestFiles"`
}
type Metadata struct {
	Branch     string `json:"branch"`
	Commit     string `json:"commit_hash"`
	SCMURL     string `json:"integration_repo_url"`
	ReportURL  string `json:"html_report_url"`
	Incomplete bool   `json:"incomplete"`
}
type Snapshot struct {
	Files     []string   `json:"files"`
	Artifacts []Artifact `json:"artifacts"`
	Metadata  Metadata   `json:"metadata"`
}
type Client struct {
	HTTP  *http.Client
	base  string
	slots chan struct{}
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, base: "https://api.socket.dev/v0", slots: make(chan struct{}, 4)}
}
func (c *Client) get(ctx context.Context, endpoint, token, accept string, decode func(io.Reader) error) error {
	for attempt := 0; attempt < 4; attempt++ {
		select {
		case c.slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+endpoint, nil)
		if err != nil {
			<-c.slots
			return errors.New("invalid Socket request")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", accept)
		resp, err := c.HTTP.Do(req)
		delay := time.Duration(1<<attempt) * time.Second
		retry := err != nil
		if err == nil {
			retry = resp.StatusCode == 429 || resp.StatusCode >= 500 || resp.StatusCode == 202
			if n, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && n >= 0 {
				delay = time.Duration(n) * time.Second
			} else if at, e := http.ParseTime(resp.Header.Get("Retry-After")); e == nil {
				delay = time.Until(at)
				if delay < 0 {
					delay = 0
				}
			}
			switch {
			case resp.StatusCode == 200:
				body := &responseReader{Reader: resp.Body}
				limited := &io.LimitedReader{R: body, N: maxResponse + 1}
				err = decode(limited)
				if body.err != nil {
					err = body.err
					var networkError net.Error
					retry = errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &networkError)
				}
				if limited.N <= 0 {
					err = errors.New("Socket response exceeds import limit")
				}
			case resp.StatusCode == 202:
				err = ErrProcessing
			default:
				err = fmt.Errorf("Socket request failed (HTTP %d); check token scopes and organization", resp.StatusCode)
			}
			resp.Body.Close()
		} else {
			err = errors.New("Socket request could not be completed")
		}
		<-c.slots
		if !retry || attempt == 3 {
			return err
		}
		if delay > 60*time.Second {
			return errors.New("Socket rate limit requires a later sync")
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
	return errors.New("Socket request failed")
}

type responseReader struct {
	io.Reader
	err error
}

func (r *responseReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}
func orgPath(org string) string { return "/orgs/" + url.PathEscape(org) }
func (c *Client) Repositories(ctx context.Context, org, token string) ([]Repository, error) {
	result := make([]Repository, 0)
	seen := map[string]bool{}
	pages := map[int]bool{}
	for page := 1; page != 0; {
		if pages[page] || len(pages) >= 10000 {
			return nil, errors.New("Socket pagination did not finish")
		}
		pages[page] = true
		var response struct {
			Results []Repository `json:"results"`
			Next    *int         `json:"nextPage"`
		}
		err := c.get(ctx, orgPath(org)+"/repos?per_page=100&include_archived=true&sort=created_at&direction=asc&page="+strconv.Itoa(page), token, "application/json", func(r io.Reader) error {
			response.Results = nil
			response.Next = nil
			return json.NewDecoder(r).Decode(&response)
		})
		if err != nil {
			return nil, err
		}
		if response.Results == nil {
			return nil, errors.New("invalid Socket repository list")
		}
		for _, repo := range response.Results {
			if repo.ID == "" {
				return nil, errors.New("Socket repository has no stable identity")
			}
			if !seen[repo.ID] {
				result = append(result, repo)
				seen[repo.ID] = true
			}
		}
		page = 0
		if response.Next != nil {
			page = *response.Next
		}
	}
	return result, nil
}
func (c *Client) Snapshot(ctx context.Context, org, token, scan string) (Snapshot, error) {
	result := Snapshot{Files: []string{}, Artifacts: []Artifact{}}
	endpoint := orgPath(org) + "/full-scans/" + url.PathEscape(scan)
	err := c.get(ctx, endpoint+"/metadata", token, "application/json", func(r io.Reader) error {
		result.Metadata = Metadata{}
		return json.NewDecoder(r).Decode(&result.Metadata)
	})
	if err != nil {
		return result, err
	}
	if result.Metadata.Incomplete {
		return result, ErrProcessing
	}
	err = c.get(ctx, endpoint+"?include_scores=false&include_license_details=false", token, "application/x-ndjson", func(r io.Reader) error {
		artifacts := []Artifact{}
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 8<<20)
		for scanner.Scan() {
			var row struct {
				Artifact
				Event string `json:"_type"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
				return errors.New("invalid Socket artifact stream")
			}
			if row.Event == "scores" {
				continue
			}
			if row.ID == "" || row.Ecosystem == "" {
				return errors.New("invalid Socket artifact")
			}
			if row.References == nil {
				row.References = []Reference{}
			}
			artifacts = append(artifacts, row.Artifact)
			if len(artifacts) > 500000 {
				return errors.New("Socket artifact count exceeds import limit")
			}
		}
		result.Artifacts = artifacts
		return scanner.Err()
	})
	if err != nil {
		return result, err
	}
	files := map[string]bool{}
	err = c.get(ctx, endpoint+"/files/tar", token, "application/x-tar", func(r io.Reader) error {
		files = map[string]bool{}
		reader := bufio.NewReader(r)
		magic, _ := reader.Peek(2)
		var input io.Reader = reader
		if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
			gz, e := gzip.NewReader(reader)
			if e != nil {
				return errors.New("invalid Socket archive")
			}
			defer gz.Close()
			input = gz
		}
		limited := &io.LimitedReader{R: input, N: maxResponse + 1}
		archive := tar.NewReader(limited)
		count := 0
		for {
			header, e := archive.Next()
			if e == io.EOF {
				if limited.N <= 0 {
					return errors.New("Socket archive exceeds import limit")
				}
				return nil
			}
			if e != nil {
				return errors.New("invalid Socket file archive")
			}
			count++
			if count > 100000 {
				return errors.New("Socket file count exceeds import limit")
			}
			if header.Typeflag == tar.TypeReg {
				name, e := cleanFile(header.Name)
				if e != nil {
					return e
				}
				files[name] = true
			}
		}
	})
	if err != nil {
		return result, err
	}
	for i, a := range result.Artifacts {
		for j, ref := range a.References {
			name, e := cleanFile(ref.File)
			if e != nil {
				return result, e
			}
			result.Artifacts[i].References[j].File = name
			files[name] = true
		}
	}
	for file := range files {
		result.Files = append(result.Files, file)
	}
	return result, nil
}
func cleanFile(name string) (string, error) {
	name = path.Clean(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return "", errors.New("invalid Socket file path")
	}
	return name, nil
}
func DashboardURL(raw string) string {
	u, e := url.Parse(raw)
	if e == nil && u.Scheme == "https" && (u.Host == "socket.dev" || u.Host == "www.socket.dev") && u.User == nil {
		return u.String()
	}
	return ""
}
func NormalizeSCM(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "git@") {
		raw = "https://" + strings.Replace(strings.TrimPrefix(raw, "git@"), ":", "/", 1)
	}
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "ssh") {
		return ""
	}
	repoPath := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), ".git")
	if strings.EqualFold(u.Hostname(), "github.com") {
		repoPath = strings.ToLower(repoPath)
	}
	return strings.ToLower(u.Host) + repoPath
}
