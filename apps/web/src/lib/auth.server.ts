import { cookies } from "next/headers";

import { serverApiFetch } from "./api";
import { env } from "./env";
import type { CurrentUser } from "./types";

export async function fetchCurrentUserServer() {
  const cookieHeader = (await cookies()).toString();
  return serverApiFetch<CurrentUser>(`${env.API_INTERNAL_BASE_URL}/auth/me`, {
    headers: cookieHeader ? { cookie: cookieHeader } : undefined
  });
}
