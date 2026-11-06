package store

import (
	"fmt"
	"regexp"
	"strings"

	"deps.dev/util/semver"
)

// Only known ecosystems get constraint interpretation. Unknown ecosystems can
// still match an identical recorded version without assuming npm semantics.
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
	return err == nil && parsed.MatchVersion(version)
}
