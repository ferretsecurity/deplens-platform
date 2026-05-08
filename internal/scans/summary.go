package scans

type Summary struct {
	ManifestCount                  int
	ManifestsWithDependenciesCount int
	ManifestsWithoutDependencies   int
	ManifestsUnknownCount          int
	DependencyCount                int
}

func BuildSummary(input UploadRequest) Summary {
	summary := Summary{
		ManifestCount: len(input.Snapshot.Manifests),
	}

	for _, manifest := range input.Snapshot.Manifests {
		switch {
		case manifest.HasDependencies == nil:
			summary.ManifestsUnknownCount++
		case *manifest.HasDependencies:
			summary.ManifestsWithDependenciesCount++
		default:
			summary.ManifestsWithoutDependencies++
		}
		summary.DependencyCount += len(manifest.Dependencies)
	}

	return summary
}
