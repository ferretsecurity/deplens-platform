package store

import (
	"fmt"
	"regexp"
	"strings"

	"deps.dev/util/semver"
)

var packageVersionSystems = map[string]semver.System{
	"npm": semver.NPM, "pypi": semver.PyPI, "pip": semver.PyPI,
	"cargo": semver.Cargo, "go": semver.Go, "maven": semver.Maven,
	"nuget": semver.NuGet, "gem": semver.RubyGems, "rubygems": semver.RubyGems,
	"composer": semver.Composer,
}

var concretePackageVersion = regexp.MustCompile(`^[vV]?[0-9][0-9A-Za-z.!+_-]*$`)

// ValidatePackageVersion accepts a concrete version in a supported syntax,
// rather than an expression, wildcard, tag, or URL. Empty means no filter.
func ValidatePackageVersion(version string) error {
	if version == "" {
		return nil
	}
	for _, part := range strings.Split(version, ".") {
		if part == "x" || part == "X" {
			return fmt.Errorf("version must not contain a wildcard")
		}
	}
	if len(version) <= 200 && concretePackageVersion.MatchString(version) {
		for _, system := range packageVersionSystems {
			parsed, err := system.Parse(version)
			if err == nil && !parsed.IsWildcard() {
				return nil
			}
		}
	}
	return fmt.Errorf("version must be a concrete package version of at most 200 characters")
}

func packageMatchesVersion(item PackageUsageItem, requested string) bool {
	recorded := strings.TrimSpace(item.Version)
	system, known := packageVersionSystems[strings.ToLower(strings.TrimSpace(item.Ecosystem))]
	if recorded != "" && recorded == requested {
		return true
	}
	if !known {
		return false
	}
	version, err := system.Parse(requested)
	if err != nil || version.IsWildcard() {
		return false
	}
	if recorded != "" {
		installed, err := system.Parse(recorded)
		return err == nil && !installed.IsWildcard() && installed.Compare(version) == 0
	}
	constraint := strings.TrimSpace(item.Constraint)
	if constraint == "" {
		return false
	}
	parsed, err := system.ParseConstraint(constraint)
	if err != nil {
		return false
	}
	if system == semver.PyPI && strings.Contains(requested, "+") {
		public, _, _ := strings.Cut(requested, "+")
		for _, specifier := range strings.Split(constraint, ",") {
			specifier = strings.TrimSpace(specifier)
			if strings.Contains(specifier, "+") {
				operator := "=="
				if strings.HasPrefix(specifier, "!=") {
					operator = "!="
				}
				target, err := system.Parse(strings.TrimSpace(strings.TrimPrefix(specifier, operator)))
				if err != nil || (version.Compare(target) == 0) != (operator == "==") {
					return false
				}
			} else {
				clause, err := system.ParseConstraint(specifier)
				if err != nil || !clause.Match(public) {
					return false
				}
			}
		}
		return true
	}
	return parsed.MatchVersion(version)
}
