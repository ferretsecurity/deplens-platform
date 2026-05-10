import { clientApiFetch } from "./api";
import type { CurrentUser } from "./types";

export function login(email: string, password: string) {
  return clientApiFetch<void>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password })
  });
}

export function logout() {
  return clientApiFetch<void>("/auth/logout", { method: "POST" });
}

export function fetchCurrentUserClient() {
  return clientApiFetch<CurrentUser>("/auth/me");
}
