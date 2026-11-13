package assessment

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/git-pkgs/vers"
)

var systems = map[string]bool{"npm": true, "pypi": true, "cargo": true, "go": true, "maven": true, "nuget": true, "gem": true, "composer": true}
var pythonSeparators = regexp.MustCompile(`[-_.]+`)

func Ecosystem(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "pip":
		return "pypi"
	case "rubygems":
		return "gem"
	}
	return value
}

func PackageName(ecosystem, name string) string {
	name = strings.TrimSpace(name)
	switch Ecosystem(ecosystem) {
	case "pypi":
		return pythonSeparators.ReplaceAllString(strings.ToLower(name), "-")
	case "npm", "nuget", "composer":
		return strings.ToLower(name)
	}
	return name
}

type Query struct {
	Ecosystem string
	Name      string
	Range     string
	affected  *vers.Range
}

func NewQuery(ecosystem, name, affectedRange string) (Query, error) {
	q := Query{Ecosystem: Ecosystem(ecosystem), Name: PackageName(ecosystem, name), Range: strings.TrimSpace(affectedRange)}
	_, ok := systems[q.Ecosystem]
	if !ok {
		return q, fmt.Errorf("unsupported ecosystem")
	}
	if q.Name == "" || len(q.Name) > 200 || q.Range == "" || len(q.Range) > 200 {
		return q, fmt.Errorf("package name and affected range are required and must be at most 200 characters")
	}
	constraint, err := parseRange(q.Range, q.Ecosystem)
	if err != nil {
		return q, fmt.Errorf("invalid affected range for %s: %w", q.Ecosystem, err)
	}
	nonempty, overlapErr := rangesOverlap(constraint, constraint, q.Ecosystem)
	if overlapErr != nil {
		return q, fmt.Errorf("affected range could not be assessed: %w", overlapErr)
	}
	if !nonempty {
		return q, fmt.Errorf("affected range is empty")
	}
	q.affected = constraint
	return q, nil
}

func (q Query) Matches(d Dependency) bool {
	return Ecosystem(d.Ecosystem) == q.Ecosystem && PackageName(q.Ecosystem, d.Name) == q.Name
}

func (q Query) versionAffected(value string) (bool, error) {
	if value == "" || !vers.ValidWithScheme(value, q.Ecosystem) {
		return false, fmt.Errorf("version %q could not be assessed", value)
	}
	return q.affected.Contains(value), nil
}

func (q Query) overlaps(value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, fmt.Errorf("version constraint is missing")
	}
	constraint, err := parseRange(value, q.Ecosystem)
	if err != nil {
		return false, fmt.Errorf("version constraint could not be assessed")
	}
	return rangesOverlap(constraint, q.affected, q.Ecosystem)
}

func registryOrigin(d Dependency) bool {
	return d.Origin == "" || d.Origin == "registry"
}

func (q Query) dependencyAffected(d Dependency) (bool, error) {
	if !registryOrigin(d) {
		return false, fmt.Errorf("dependency is not a registry package; its identity could not be assessed")
	}
	return q.versionAffected(d.Version)
}

func rangesOverlap(left, right *vers.Range, ecosystem string) (bool, error) {
	// Native containment also applies exclusions, public Python equality, and prerelease rules.
	for _, r := range []*vers.Range{left, right} {
		for _, interval := range r.Intervals {
			for _, bound := range []string{interval.Min, interval.Max} {
				if bound != "" && left.Contains(bound) && right.Contains(bound) {
					return true, nil
				}
			}
		}
	}
	intersection := left.Intersect(right)
	if ecosystem == "npm" || ecosystem == "cargo" {
		return semverOverlap(left, right, intersection)
	}
	for _, interval := range intersection.Intervals {
		if interval.IsEmptyWithScheme(ecosystem) {
			continue
		}
		if interval.Min != "" && interval.Max != "" && vers.CompareWithScheme(interval.Min, interval.Max, ecosystem) == 0 {
			continue
		}
		return true, nil
	}
	if ecosystem == "pypi" {
		for _, r := range []*vers.Range{left, right} {
			for _, interval := range r.Intervals {
				if strings.Contains(interval.Min, "+") || strings.Contains(interval.Max, "+") {
					return false, fmt.Errorf("overlap involving Python local versions could not be established")
				}
			}
		}
	}
	return false, nil
}

func semverOverlap(left, right, intersection *vers.Range) (bool, error) {
	for _, interval := range intersection.Intervals {
		candidate := "0.0.0"
		if interval.Min != "" {
			minimum, err := vers.ParseVersion(interval.Min)
			if err != nil {
				continue
			}
			candidate = fmt.Sprintf("%d.%d.%d", minimum.Major, minimum.Minor, minimum.Patch)
			if minimum.Prerelease == "" && !interval.MinInclusive {
				if minimum.Patch == math.MaxInt {
					return false, fmt.Errorf("overlap exceeds supported version precision")
				}
				candidate = fmt.Sprintf("%d.%d.%d", minimum.Major, minimum.Minor, minimum.Patch+1)
			}
		}
		for attempt := 0; attempt <= len(left.Exclusions)+len(right.Exclusions); attempt++ {
			if left.Contains(candidate) && right.Contains(candidate) {
				return true, nil
			}
			if attempt == len(left.Exclusions)+len(right.Exclusions) {
				break
			}
			parsed, err := vers.ParseVersion(candidate)
			if err != nil {
				break
			}
			if parsed.Patch == math.MaxInt {
				return false, fmt.Errorf("overlap exceeds supported version precision")
			}
			candidate = fmt.Sprintf("%d.%d.%d", parsed.Major, parsed.Minor, parsed.Patch+1)
		}
		// Prereleases require an explicit bound with the same release tuple in both operands.
		for _, r := range []*vers.Range{left, right} {
			for _, original := range r.Intervals {
				for _, bound := range []string{original.Min, original.Max} {
					parsed, err := vers.ParseVersion(bound)
					if err != nil || parsed.Prerelease == "" {
						continue
					}
					candidate = fmt.Sprintf("%d.%d.%d-0", parsed.Major, parsed.Minor, parsed.Patch)
					if interval.Min != "" && vers.CompareWithScheme(candidate, interval.Min, "npm") <= 0 {
						candidate = strings.Split(interval.Min, "+")[0]
						if !interval.MinInclusive {
							candidate += ".0"
						}
					}
					for attempt := 0; attempt <= len(left.Exclusions)+len(right.Exclusions); attempt++ {
						if left.Contains(candidate) && right.Contains(candidate) {
							return true, nil
						}
						candidate += ".0"
					}
				}
			}
		}
	}
	return false, nil
}

func parseRange(value, ecosystem string) (*vers.Range, error) {
	parsed, err := vers.ParseNative(value, ecosystem)
	if err != nil {
		return nil, err
	}
	for _, interval := range parsed.Intervals {
		for _, bound := range []string{interval.Min, interval.Max} {
			if bound != "" && !vers.ValidWithScheme(bound, ecosystem) {
				return nil, fmt.Errorf("range contains an unsupported version or tag")
			}
		}
	}
	for _, excluded := range parsed.Exclusions {
		if !vers.ValidWithScheme(excluded, ecosystem) {
			return nil, fmt.Errorf("range contains an unsupported exclusion")
		}
	}
	return parsed, nil
}
