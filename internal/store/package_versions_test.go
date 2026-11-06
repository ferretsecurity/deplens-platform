package store

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPackageMatchesVersion(t *testing.T) {
	for _, tc := range []struct {
		ecosystem, recorded, constraint, requested string
		match                                      bool
	}{
		{"pypi", "", ">=1.103.0,<2", "1.103.1", true},
		{"pypi", "", "==1.103.1", "1.103.1+cpu", true},
		{"pypi", "", ">=1.103.1,<2", "1.103.1+cpu", true},
		{"pypi", "", "!=1.103.1", "1.103.1+cpu", false},
		{"pypi", "", "==1.103.1+cpu", "1.103.1+cpu", true},
		{"pypi", "", ">=1,!=1.103.1+cpu", "1.103.1+cpu", false},
		{"pypi", "", ">=1,!=1.103.1+cpu", "1.103.1+gpu", true},
		{"pypi", "", "==1.103.1+cpu", "1.103.1+gpu", false},
		{"pip", "", ">=1.103.0,<2", "1.102.9", false},
		{"pypi", "", ">=1.103.0,<2", "1.103.0", true},
		{"pypi", "", ">=1.103.0,<2", "2.0.0", false},
		{"pypi", "", "==1.103.1", "1.103.1", true},
		{"pypi", "", "==1.103.1", "1.103.2", false},
		{"pypi", "", ">=1.103.1,<2.0.0,!=1.104.0rc1,!=1.107.0", "1.107.0", false},
		{"pypi", "", ">=1.103.1,<2.0.0,!=1.104.0rc1,!=1.107.0", "1.106.6", true},
		{"pypi", "", ">=1.103.1,<2.0.0,!=1.104.0rc1,!=1.107.0", "1.104.0rc1", false},
		{"pypi", "", ">=1.104.0rc1,<2", "1.104.0rc2", true},
		{"pypi", "", ">=1.103,<2", "1.104.0rc1", false},
		{"pypi", "", "~=1.103.1", "1.103.9", true},
		{"pypi", "", "~=1.103.1", "1.104.0", false},
		{"pypi", "", "==1.103.*", "1.103.2", true},
		{"pypi", "", "!=1.103.*,>=1", "1.103.2", false},
		{"pypi", "1.100.4", ">=1.95,<2", "1.103.1", false},
		{"pypi", "1.103.1", "invalid", "1.103.1", true},
		{"pypi", "1.103.1.0", "", "1.103.1", true},
		{"npm", "", "^1.103.0", "1.103.1", true},
		{"unknown", "", "^1.103.0", "1.103.1", false},
		{"", "1.103.1", "", "1.103.1", true},
		{"unknown", "1.103.0", ">=1", "1.103.1", false},
		{"pypi", "", "", "1.103.1", false},
		{"pypi", "", "broken", "1.103.1", false},
		{"pypi", "", "https://example.com/pkg", "1.103.1", false},
	} {
		t.Run(tc.ecosystem+"/"+tc.recorded+"/"+tc.constraint+"/"+tc.requested, func(t *testing.T) {
			require.Equal(t, tc.match, packageMatchesVersion(PackageUsageItem{Ecosystem: tc.ecosystem, Version: tc.recorded, Constraint: tc.constraint}, tc.requested))
		})
	}
}

func TestValidatePackageVersion(t *testing.T) {
	for _, version := range []string{"", "1.103.1", "1.104.0rc1", "v1.2.3", "1!2.0", "1.2.3+linux"} {
		require.NoError(t, ValidatePackageVersion(version), version)
	}
	for _, version := range []string{"*", "1.*", "1.x", "1.X", "1://example.com", "^1.2", ">=1", "latest", "garbage", "1.2 || 2", "https://example.com"} {
		require.Error(t, ValidatePackageVersion(version), version)
	}
}
