import { cookies } from "next/headers";

import { serverApiFetch } from "./api";
import { env } from "./env";
import type { APITokenMetadata, RepositoryListItem, ScanListItem, ScanManifestItem } from "./types";

function cookieHeaders() {
  const cookieHeader = cookies().toString();
  return cookieHeader ? { cookie: cookieHeader } : undefined;
}

export async function listRepositoriesServer() {
  return serverApiFetch<RepositoryListItem[]>(`${env.API_INTERNAL_BASE_URL}/api/v1/repositories`, {
    headers: cookieHeaders()
  });
}

export async function listScansServer(repositoryId: string, from: string, to: string) {
  const params = new URLSearchParams({
    repository_id: repositoryId,
    from,
    to
  });
  return serverApiFetch<ScanListItem[]>(`${env.API_INTERNAL_BASE_URL}/api/v1/scans?${params.toString()}`, {
    headers: cookieHeaders()
  });
}

export async function getScanServer(scanId: string) {
  return serverApiFetch<ScanListItem>(`${env.API_INTERNAL_BASE_URL}/api/v1/scans/${scanId}`, {
    headers: cookieHeaders()
  });
}

export async function listScanManifestsServer(scanId: string) {
  return serverApiFetch<{ items: ScanManifestItem[] }>(
    `${env.API_INTERNAL_BASE_URL}/api/v1/scans/${scanId}/manifests`,
    {
      headers: cookieHeaders()
    }
  );
}

export async function listAPITokensServer() {
  return serverApiFetch<APITokenMetadata[]>(`${env.API_INTERNAL_BASE_URL}/api/v1/tokens`, {
    headers: cookieHeaders()
  });
}
