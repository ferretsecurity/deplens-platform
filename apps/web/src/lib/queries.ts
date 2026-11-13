import { snykInventorySchema } from "./snyk";
import { socketInventorySchema } from "./socket";
import { cookies } from "next/headers";

import { serverApiFetch } from "./api";
import { env } from "./env";
import { parseScannerReportResponse } from "./scanner-report";
import { packageListResponseSchema } from "./packages";
import type {
  APITokenMetadata,
  DependencyListItem,
  RepositoryListResponse,
  RepositoryManifestItem,
  ScanListItem,
  ScanManifestItem
} from "./types";

async function cookieHeaders() {
  const cookieHeader = (await cookies()).toString();
  return cookieHeader ? { cookie: cookieHeader } : undefined;
}

type ListRepositoriesParams = {
  q?: string;
  page?: number;
  page_size?: number;
};

export async function listRepositoriesServer(params: ListRepositoriesParams = {}) {
  const searchParams = new URLSearchParams();
  if (params.q) {
    searchParams.set("q", params.q);
  }
  if (params.page) {
    searchParams.set("page", String(params.page));
  }
  if (params.page_size) {
    searchParams.set("page_size", String(params.page_size));
  }

  const query = searchParams.toString();
  const url = `${env.API_INTERNAL_BASE_URL}/api/v1/repositories${query ? `?${query}` : ""}`;

  return serverApiFetch<RepositoryListResponse>(url, {
    headers: await cookieHeaders()
  });
}

export async function listRepositoryOptionsServer() {
  const response = await listRepositoriesServer({ page: 1, page_size: 100 });
  const items = [...response.items];
  let pagination = response.pagination;
  while (pagination.has_next) {
    const next = await listRepositoriesServer({ page: pagination.page + 1, page_size: pagination.page_size });
    items.push(...next.items);
    pagination = next.pagination;
  }
  return items;
}

export async function listRepositoryManifestsServer(repositoryId: string) {
  return serverApiFetch<{ items: RepositoryManifestItem[] }>(
    `${env.API_INTERNAL_BASE_URL}/api/v1/repositories/${repositoryId}/manifests`,
    {
      headers: await cookieHeaders()
    }
  );
}

export async function listDependenciesServer() {
  return serverApiFetch<{ items: DependencyListItem[] }>(`${env.API_INTERNAL_BASE_URL}/api/v1/dependencies`, {
    headers: await cookieHeaders()
  });
}

export async function listPackagesServer(params: { q: string; version: string; page: number; page_size: number; ecosystem?: string; affected_range?: string }) {
  const searchParams = new URLSearchParams({
    q: params.q,
    version: params.version,
    page: String(params.page),
    page_size: String(params.page_size)
  });
  if (params.ecosystem) searchParams.set("ecosystem", params.ecosystem);
  if (params.affected_range) searchParams.set("affected_range", params.affected_range);
  const response = await serverApiFetch<unknown>(
    `${env.API_INTERNAL_BASE_URL}/api/v1/packages?${searchParams.toString()}`,
    { headers: await cookieHeaders() }
  );
  return packageListResponseSchema.parse(response);
}

export async function listScansServer(repositoryId: string, from: string, to: string) {
  const params = new URLSearchParams({
    repository_id: repositoryId,
    from,
    to
  });
  return serverApiFetch<ScanListItem[]>(`${env.API_INTERNAL_BASE_URL}/api/v1/scans?${params.toString()}`, {
    headers: await cookieHeaders()
  });
}

export async function getScanServer(scanId: string) {
  return serverApiFetch<ScanListItem>(`${env.API_INTERNAL_BASE_URL}/api/v1/scans/${scanId}`, {
    headers: await cookieHeaders()
  });
}

export async function listScanManifestsServer(scanId: string) {
  return serverApiFetch<{ items: ScanManifestItem[] }>(
    `${env.API_INTERNAL_BASE_URL}/api/v1/scans/${scanId}/manifests`,
    {
      headers: await cookieHeaders()
    }
  );
}

export async function listAPITokensServer() {
  return serverApiFetch<APITokenMetadata[]>(`${env.API_INTERNAL_BASE_URL}/api/v1/tokens`, {
    headers: await cookieHeaders()
  });
}

export async function getScanReportServer(scanId: string) {
  const response = await serverApiFetch<unknown>(
    `${env.API_INTERNAL_BASE_URL}/api/v1/scans/${scanId}/report`,
    { headers: await cookieHeaders() }
  );
  return parseScannerReportResponse(response);
}

export async function getRepositoryInventoryServer(repositoryId: string) {
  const response = await serverApiFetch<unknown>(`${env.API_INTERNAL_BASE_URL}/api/v1/repositories/${repositoryId}/inventory`, {headers: await cookieHeaders()});
  return socketInventorySchema.parse(response);
}

export async function getRepositorySnykInventoryServer(repositoryId: string) {
  const response = await serverApiFetch<unknown>(`${env.API_INTERNAL_BASE_URL}/api/v1/repositories/${repositoryId}/snyk-inventory`, { headers: await cookieHeaders() });
  return snykInventorySchema.parse(response);
}
