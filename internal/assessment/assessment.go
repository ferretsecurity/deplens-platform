package assessment

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

type Dependency struct {
	Name       string `json:"name"`
	Ecosystem  string `json:"ecosystem"`
	Version    string `json:"version"`
	Constraint string `json:"constraint"`
	Origin     string `json:"origin"`
}

type Source struct {
	Path         string       `json:"path"`
	Form         string       `json:"form"`
	Extraction   string       `json:"extraction"`
	Lockfiles    []string     `json:"lockfiles"`
	Dependencies []Dependency `json:"dependencies"`
}

type Evidence struct {
	Status    string   `json:"status"`
	Scope     string   `json:"scope"`
	Root      string   `json:"root"`
	Reasons   []string `json:"reasons"`
	Lockfiles []string `json:"lockfiles"`
	Manifests []string `json:"manifests"`
}

type Finding struct {
	Path       string
	Version    string
	Constraint string
	Evidence   Evidence
}

var lockfileNames = map[string][]string{
	"package.json":   {"package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"},
	"pyproject.toml": {"uv.lock", "poetry.lock", "pdm.lock"},
	"build.gradle":   {"gradle.lockfile"}, "build.gradle.kts": {"gradle.lockfile"},
	".csproj": {"packages.lock.json"}, ".fsproj": {"packages.lock.json"}, ".vbproj": {"packages.lock.json"},
	"Pipfile": {"Pipfile.lock"}, "Cargo.toml": {"Cargo.lock"},
	"composer.json": {"composer.lock"}, "Gemfile": {"Gemfile.lock"},
}

func isLockfile(s Source) bool {
	if s.Form != "" {
		return s.Form == "lockfile"
	}
	for _, names := range lockfileNames {
		if slices.Contains(names, path.Base(s.Path)) {
			return true
		}
	}
	return false
}

func declarationSource(source Source) bool {
	if source.Form == "" {
		return path.Base(source.Path) != "go.sum"
	}
	switch source.Form {
	case "manifest", "requirements", "constraint-file", "build-definition", "version-catalog":
		return true
	}
	return false
}

func candidates(source Source, sources map[string]Source) []string {
	if source.Lockfiles != nil {
		return source.Lockfiles
	}
	result := []string{}
	names := lockfileNames[path.Base(source.Path)]
	if names == nil {
		names = lockfileNames[path.Ext(source.Path)]
	}
	for _, name := range names {
		candidate := path.Join(path.Dir(source.Path), name)
		if lock, exists := sources[candidate]; exists && isLockfile(lock) {
			result = append(result, candidate)
		}
	}
	return result
}

func (q Query) Assess(sources []Source) []Finding {
	byPath := make(map[string]Source, len(sources))
	for _, source := range sources {
		byPath[source.Path] = source
	}
	references := map[string][]string{}
	for _, source := range sources {
		if isLockfile(source) || !declarationSource(source) {
			continue
		}
		for _, lock := range candidates(source, byPath) {
			references[lock] = append(references[lock], source.Path)
		}
	}
	findings := []Finding{}
	for _, source := range sources {
		if !isLockfile(source) {
			continue
		}
		for _, d := range source.Dependencies {
			if !q.Matches(d) {
				continue
			}
			affected, err := q.dependencyAffected(d)
			if !affected && err == nil {
				continue
			}
			status, reason := "issue", "Lockfile records an affected version."
			if err != nil {
				status, reason = "unknown", err.Error()
			}
			scope := "project"
			manifests := []string{}
			for _, manifest := range references[source.Path] {
				if path.Dir(manifest) != path.Dir(source.Path) {
					scope = "workspace"
				}
				for _, declaration := range byPath[manifest].Dependencies {
					if q.Matches(declaration) && registryOrigin(declaration) {
						manifests = append(manifests, manifest)
						break
					}
				}
			}
			slices.Sort(manifests)
			findings = append(findings, Finding{Path: source.Path, Version: d.Version, Evidence: Evidence{
				Status: status, Scope: scope, Root: path.Dir(source.Path), Reasons: []string{reason},
				Lockfiles: []string{source.Path}, Manifests: manifests,
			}})
		}
	}
	for _, source := range sources {
		if isLockfile(source) {
			continue
		}
		for _, d := range source.Dependencies {
			if !q.Matches(d) {
				continue
			}
			if !declarationSource(source) {
				findings = append(findings, Finding{Path: source.Path, Version: d.Version, Constraint: d.Constraint,
					Evidence: Evidence{Status: "unknown", Scope: "project", Root: path.Dir(source.Path),
						Reasons:   []string{"This source does not establish a dependency declaration or a selected version."},
						Lockfiles: []string{}, Manifests: []string{}}})
				continue
			}
			constraint := d.Constraint
			if constraint == "" && d.Version != "" {
				constraint = "=" + d.Version
				if q.Ecosystem == "pypi" {
					constraint = "==" + d.Version
				}
				if q.Ecosystem == "maven" || q.Ecosystem == "nuget" {
					constraint = "[" + d.Version + "]"
				}
			}
			overlap, err := q.overlaps(constraint)
			if !registryOrigin(d) {
				overlap, err = false, fmt.Errorf("dependency is not a registry package; its identity could not be assessed")
			}
			if !overlap && err == nil {
				continue
			}
			locks := candidates(source, byPath)
			reasons := []string{}
			if len(locks) == 0 {
				reasons = append(reasons, "No corresponding lockfile was found.")
			}
			for _, lockPath := range locks {
				lock, exists := byPath[lockPath]
				if !exists || !isLockfile(lock) || lock.Extraction != "complete" {
					reasons = append(reasons, lockPath+": lockfile could not be completely assessed.")
					continue
				}
				shared := len(references[lockPath]) > 1 || path.Dir(source.Path) != path.Dir(lockPath)
				for _, manifest := range references[lockPath] {
					shared = shared || path.Dir(manifest) != path.Dir(lockPath)
				}
				if shared {
					reasons = append(reasons, lockPath+": shared lockfile resolutions cannot be attributed to this declaration.")
					continue
				}
				found, valid := false, true
				declared, parseErr := parseRange(constraint, q.Ecosystem)
				for _, resolution := range lock.Dependencies {
					if !q.Matches(resolution) {
						continue
					}
					if _, versionErr := q.dependencyAffected(resolution); versionErr != nil {
						valid = false
						continue
					}
					if parseErr == nil && declared.Contains(resolution.Version) {
						found = true
					}
				}
				if !found || !valid {
					reasons = append(reasons, lockPath+": no complete resolution matching this declaration was established.")
				}
			}
			if len(reasons) == 0 && err == nil {
				continue
			}
			status := "warning"
			if err != nil {
				status = "unknown"
				reasons = append(reasons, err.Error())
			}
			findings = append(findings, Finding{Path: source.Path, Version: d.Version, Constraint: d.Constraint, Evidence: Evidence{
				Status: status, Scope: "project", Root: path.Dir(source.Path), Reasons: reasons,
				Lockfiles: append([]string{}, locks...), Manifests: []string{source.Path},
			}})
		}
	}
	return findings
}

// Provider observations are evaluated independently from CLI scans of potentially different commits.
func (q Query) AssessObservation(d Dependency, file string) *Finding {
	if !q.Matches(d) {
		return nil
	}
	affected, err := q.dependencyAffected(d)
	if !affected && err == nil {
		return nil
	}
	status, reason := "issue", "Provider reports an affected version; this is independent of CLI lockfile evidence."
	if err != nil {
		status, reason = "unknown", err.Error()
	}
	root := path.Dir(file)
	if strings.TrimSpace(file) == "" {
		root = ""
	}
	return &Finding{Path: file, Version: d.Version, Evidence: Evidence{Status: status, Scope: "repository", Root: root,
		Reasons: []string{reason}, Lockfiles: []string{}, Manifests: []string{}}}
}
