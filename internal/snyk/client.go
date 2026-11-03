package snyk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/socket"
)

const apiVersion = "2024-10-15"

var regions = map[string]string{"us": "snyk.io", "us2": "us.snyk.io", "eu": "eu.snyk.io", "au": "au.snyk.io"}

func validRegion(region string) bool { _, ok := regions[region]; return ok }

type Client struct {
	HTTP  *http.Client
	slots chan struct{}
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, slots: make(chan struct{}, 4)}
}
func (c *Client) get(ctx context.Context, region, endpoint, token string, out interface{}) error {
	host, ok := regions[region]
	if !ok {
		return ErrConfig
	}
	base := "https://api." + host
	u, err := url.Parse(base + endpoint)
	if err != nil {
		return errors.New("invalid Snyk request")
	}
	for attempt := 0; attempt < 4; attempt++ {
		select {
		case c.slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if e != nil {
			<-c.slots
			return e
		}
		authorization := "token " + token
		if strings.HasPrefix(token, "Bearer ") {
			authorization = token
		}
		req.Header.Set("Authorization", authorization)
		req.Header.Set("Accept", "application/vnd.api+json")
		if strings.HasPrefix(endpoint, "/v1/") {
			req.Header.Set("Accept", "application/json")
		}
		resp, e := c.HTTP.Do(req)
		delay := time.Duration(1<<attempt) * time.Second
		retry := e != nil
		if e == nil {
			retry = resp.StatusCode == 429 || resp.StatusCode >= 500
			if n, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && n >= 0 {
				delay = time.Duration(n) * time.Second
			} else if at, e := http.ParseTime(resp.Header.Get("Retry-After")); e == nil {
				delay = time.Until(at)
				if delay < 0 {
					delay = 0
				}
			}
			if resp.StatusCode == 200 {
				data, readErr := io.ReadAll(io.LimitReader(resp.Body, (128<<20)+1))
				if readErr != nil {
					e = errors.New("Snyk response could not be read")
					retry = true
				} else if len(data) > 128<<20 {
					e = errors.New("Snyk response exceeds import limit")
				} else {
					e = json.Unmarshal(data, out)
					if e != nil {
						e = errors.New("invalid Snyk response")
					}
				}
			} else {
				e = fmt.Errorf("Snyk request failed (HTTP %d); check token permissions and organization", resp.StatusCode)
			}
			resp.Body.Close()
		} else {
			e = errors.New("Snyk request could not be completed")
		}
		<-c.slots
		if !retry || attempt == 3 {
			return e
		}
		if delay > 60*time.Second {
			return errors.New("Snyk rate limit requires a later sync")
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
	return errors.New("Snyk request failed")
}

type projectResource struct {
	ID         string `json:"id"`
	Attributes struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		TargetFile string `json:"target_file"`
		Branch     string `json:"target_reference"`
		Status     string `json:"status"`
	} `json:"attributes"`
	Relationships struct {
		Target struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"target"`
	} `json:"relationships"`
}
type targetResource struct {
	ID         string `json:"id"`
	Attributes struct {
		Name string `json:"display_name"`
		URL  string `json:"url"`
	} `json:"attributes"`
}
type Repository struct {
	ID, Name, SCMURL, DashboardURL, Branch, Head string
	Projects                                     []Project
}
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	TargetFile string `json:"target_file"`
	Branch     string `json:"branch"`
	URL        string `json:"url"`
}
type Snapshot struct {
	Files     []string          `json:"files"`
	Artifacts []socket.Artifact `json:"artifacts"`
	Metadata  socket.Metadata   `json:"metadata"`
	Projects  []Project         `json:"projects"`
}

func nextPage(raw, expected string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, e := url.Parse(raw)
	if e != nil || u.IsAbs() || u.Host != "" || u.Path != expected || u.Fragment != "" {
		return "", errors.New("invalid Snyk pagination link")
	}
	return u.String(), nil
}
func (c *Client) Repositories(ctx context.Context, org, region, token string) ([]Repository, error) {
	if !validRegion(region) {
		return nil, ErrConfig
	}
	prefix := "/rest/orgs/" + url.PathEscape(org)
	targets := map[string]targetResource{}
	seen := map[string]bool{}
	for endpoint := prefix + "/targets?version=" + apiVersion + "&limit=100"; endpoint != ""; {
		if seen[endpoint] || len(seen) > 10000 {
			return nil, errors.New("Snyk pagination did not finish")
		}
		seen[endpoint] = true
		var page struct {
			Data  []targetResource `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if e := c.get(ctx, region, endpoint, token, &page); e != nil {
			return nil, e
		}
		if page.Data == nil {
			return nil, errors.New("invalid Snyk target list")
		}
		for _, t := range page.Data {
			if t.ID == "" {
				return nil, errors.New("Snyk target has no stable identity")
			}
			targets[t.ID] = t
		}
		var e error
		endpoint, e = nextPage(page.Links.Next, prefix+"/targets")
		if e != nil {
			return nil, e
		}
	}
	repos := map[string]*Repository{}
	seen = map[string]bool{}
	projectIDs := map[string]bool{}
	for endpoint := prefix + "/projects?version=" + apiVersion + "&limit=100"; endpoint != ""; {
		if seen[endpoint] || len(seen) > 10000 {
			return nil, errors.New("Snyk pagination did not finish")
		}
		seen[endpoint] = true
		var page struct {
			Data  []projectResource `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if e := c.get(ctx, region, endpoint, token, &page); e != nil {
			return nil, e
		}
		if page.Data == nil {
			return nil, errors.New("invalid Snyk project list")
		}
		for _, p := range page.Data {
			if p.ID == "" {
				return nil, errors.New("Snyk project has no stable identity")
			}
			if projectIDs[p.ID] {
				continue
			}
			projectIDs[p.ID] = true
			targetID := p.Relationships.Target.Data.ID
			if targetID == "" {
				targetID = "project:" + p.ID
			}
			target := targets[targetID]
			repo := repos[targetID]
			if repo == nil {
				name := target.Attributes.Name
				if name == "" {
					name = p.Attributes.Name
				}
				repo = &Repository{ID: targetID, Name: name, SCMURL: safeSCM(target.Attributes.URL), Projects: []Project{}}
				repos[targetID] = repo
			}
			projectURL := "https://app." + regions[region] + "/org/" + url.PathEscape(org) + "/project/" + url.PathEscape(p.ID)
			if repo.DashboardURL == "" {
				repo.DashboardURL = projectURL
			}
			if repo.Branch == "" {
				repo.Branch = p.Attributes.Branch
			}
			repo.Projects = append(repo.Projects, Project{ID: p.ID, Name: p.Attributes.Name, Type: p.Attributes.Type, TargetFile: p.Attributes.TargetFile, Branch: p.Attributes.Branch, URL: projectURL})
		}
		var e error
		endpoint, e = nextPage(page.Links.Next, prefix+"/projects")
		if e != nil {
			return nil, e
		}
	}
	result := make([]Repository, 0, len(repos))
	for _, r := range repos {
		sort.Slice(r.Projects, func(i, j int) bool { return r.Projects[i].ID < r.Projects[j].ID })
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
func safeSCM(raw string) string {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return raw
}

type graphResponse struct {
	DepGraph struct {
		PkgManager struct {
			Name string `json:"name"`
		} `json:"pkgManager"`
		Pkgs []struct {
			ID   string `json:"id"`
			Info struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"info"`
		} `json:"pkgs"`
		Graph struct {
			Root  string `json:"rootNodeId"`
			Nodes []struct {
				ID    string `json:"nodeId"`
				PkgID string `json:"pkgId"`
				Deps  []struct {
					ID string `json:"nodeId"`
				} `json:"deps"`
			} `json:"nodes"`
		} `json:"graph"`
	} `json:"depGraph"`
}

func (c *Client) Snapshot(ctx context.Context, org, region, token string, repo Repository) (Snapshot, error) {
	result := Snapshot{Files: []string{}, Artifacts: []socket.Artifact{}, Projects: repo.Projects, Metadata: socket.Metadata{SCMURL: repo.SCMURL, Branch: repo.Branch, ReportURL: repo.DashboardURL}}
	files := map[string]bool{}
	for _, p := range repo.Projects {
		if len(result.Artifacts) > 500000 {
			return result, errors.New("Snyk artifact count exceeds import limit")
		}
		if p.TargetFile != "" {
			files[p.TargetFile] = true
		}
		// Code and IaC projects have no package graph.
		if p.Type == "sast" || p.Type == "code" || p.Type == "terraformconfig" || p.Type == "k8sconfig" || p.Type == "helmconfig" || p.Type == "cloudformationconfig" || p.Type == "armconfig" || p.Type == "customconfig" {
			result.Metadata.Incomplete = true
			continue
		}
		var response graphResponse
		if e := c.get(ctx, region, "/v1/org/"+url.PathEscape(org)+"/project/"+url.PathEscape(p.ID)+"/dep-graph", token, &response); e != nil {
			return result, e
		}
		graph := response.DepGraph
		if graph.Pkgs == nil || graph.Graph.Root == "" {
			return result, errors.New("invalid Snyk dependency graph")
		}
		rootPkg := ""
		directNodes := map[string]bool{}
		for _, n := range graph.Graph.Nodes {
			if n.ID == graph.Graph.Root {
				rootPkg = n.PkgID
				for _, d := range n.Deps {
					directNodes[d.ID] = true
				}
			}
		}
		directPkgs := map[string]bool{}
		for _, n := range graph.Graph.Nodes {
			if directNodes[n.ID] {
				directPkgs[n.PkgID] = true
			}
		}
		for _, pkg := range graph.Pkgs {
			if pkg.ID == rootPkg {
				continue
			}
			refs := []socket.Reference{}
			if p.TargetFile != "" {
				refs = append(refs, socket.Reference{File: p.TargetFile})
			}
			direct := directPkgs[pkg.ID]
			if len(result.Artifacts) >= 500000 {
				return result, errors.New("Snyk artifact count exceeds import limit")
			}
			result.Artifacts = append(result.Artifacts, socket.Artifact{ID: p.ID + ":" + pkg.ID, Ecosystem: ecosystem(graph.PkgManager.Name), Name: pkg.Info.Name, Version: pkg.Info.Version, Direct: &direct, References: refs})
		}
	}
	for file := range files {
		result.Files = append(result.Files, file)
	}
	sort.Strings(result.Files)
	return result, nil
}

func ecosystem(manager string) string {
	switch manager {
	case "yarn", "pnpm":
		return "npm"
	default:
		return manager
	}
}
