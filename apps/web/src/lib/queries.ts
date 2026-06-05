import { cookies } from "next/headers";

import { serverApiFetch } from "./api";
import { env } from "./env";
import type {
  APITokenMetadata,
  DependencyListItem,
  RepositoryListItem,
  RepositoryManifestItem,
  ScanListItem,
  ScanManifestItem
} from "./types";

async function cookieHeaders() {
  const cookieHeader = (await cookies()).toString();
  return cookieHeader ? { cookie: cookieHeader } : undefined;
}

export async function listRepositoriesServer() {
  return serverApiFetch<RepositoryListItem[]>(`${env.API_INTERNAL_BASE_URL}/api/v1/repositories`, {
    headers: await cookieHeaders()
  });
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
