export type CurrentUser = {
  user_id: string;
  display_name?: string;
  memberships: MembershipRecord[];
  active_tenant_id?: string;
  role?: string;
};

export type MembershipRecord = {
  tenant_id: string;
  tenant_slug: string;
  role: string;
};

export type RepositoryListItem = {
  id: string;
  name: string;
  url: string;
  default_branch: string;
};

export type ScanListItem = {
  id: string;
  repository_id: string;
  commit_sha: string;
  scanned_at: string;
  manifest_count: number;
  dependency_count: number;
  labels: Record<string, string>;
  annotation: string;
};

export type ManifestDependencyItem = {
  id: string;
  raw: string;
  name: string;
  version: string;
  constraint: string;
  section: string;
  source: string;
  extras: Record<string, string>;
};

export type ScanManifestItem = {
  id: string;
  type: string;
  path: string;
  has_dependencies?: boolean | null;
  warnings: string[];
  dependencies: ManifestDependencyItem[];
};

export type APITokenMetadata = {
  id: string;
  label: string;
  scopes: string[];
  created_at: string;
};

export type CreatedAPIToken = APITokenMetadata & {
  token: string;
};
