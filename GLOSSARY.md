# Dependency vulnerability assessment

The language used to describe dependency declarations, locked versions, and evidence of exposure to a vulnerability.

## Language

**Repository**:
A version-controlled collection of files containing one or more projects.
_Avoid_: Application as a synonym for repository

**Project**:
An application or library with its own dependency declarations. Its dependency resolutions may be recorded in a shared workspace lockfile.
_Avoid_: Repository as a synonym for project

**Workspace**:
A group of projects managed together by a package manager, potentially sharing a lockfile.
_Avoid_: Every project in a repository

**Dependency source**:
A file containing evidence about a project's dependencies, such as a dependency manifest or a lockfile.
_Avoid_: Manifest as a name for every dependency source

**Dependency manifest**:
A file declaring a project's dependencies and their requirements, including version constraints.
_Avoid_: Lockfile

**Version constraint**:
A declaration of which versions of a dependency are permitted. It does not establish which version was installed.
_Avoid_: Installed version

**Lockfile**:
A file recording dependency resolutions, including selected package versions, for a project or workspace.
_Avoid_: Dependency manifest, proof of deployment

**Competing lockfiles**:
Multiple lockfiles representing alternative dependency resolutions for the same project. Their recorded versions need not differ for the lockfiles to compete.
_Avoid_: Unrelated lockfiles in the same repository

**Affected range**:
The set of versions of a particular package identified as vulnerable, interpreted using that package's ecosystem version rules.
_Avoid_: Fixed version as a substitute for the affected range

**Vulnerability assessment**:
An evaluation of dependency evidence for a particular package and affected range within a project or workspace.
_Avoid_: Proof of exploitation

**Vulnerability issue**:
Evidence that a dependency resolution, recorded in a lockfile or reported by a dependency inventory provider, contains a package version within an affected range. It does not by itself establish that the vulnerable version is deployed or exploitable.
_Avoid_: Confirmed exploitation

**Potential vulnerability warning**:
Evidence that a dependency manifest permits versions within an affected range without sufficient corresponding lockfile evidence to determine the selected versions. It does not establish that an affected version was installed.
_Avoid_: Confirmed vulnerable installation
