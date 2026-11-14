import React from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import SnykPage from "./page";

const configuration = {
  enabled: true, organization: "test-org", region: "eu", token_configured: true,
  interval_minutes: 360, next_run_at: null, last_run: null
};

afterEach(() => vi.unstubAllGlobals());

it("preserves a saved token when saving and queues synchronization using the saved configuration", async () => {
  const requests: { url: string; method: string; body: unknown }[] = [];
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    requests.push({ url, method: init?.method ?? "GET", body: init?.body ? JSON.parse(String(init.body)) : null });
    return new Response(JSON.stringify(init?.method ? { saved: true } : configuration), { status: 200 });
  });
  vi.stubGlobal("fetch", fetchMock);
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><SnykPage /></QueryClientProvider>);
  await waitFor(() => expect(screen.getByLabelText("Snyk region")).toHaveValue("eu"));
  expect(screen.getByLabelText("API token")).toHaveValue("");
  expect(screen.getByLabelText("API token")).toHaveAttribute("type", "password");
  fireEvent.click(screen.getByRole("button", { name: "Save configuration" }));
  await screen.findByText("Configuration saved.");
  expect(requests.find((request) => request.method === "PUT")?.body).toEqual({ organization: "test-org", region: "eu", authentication: "token", interval_minutes: 360 });
  fireEvent.click(screen.getByRole("button", { name: "Sync now" }));
  await screen.findByText("Sync queued. You can leave this page while it runs.");
  expect(requests.some((request) => request.url.endsWith("/snyk/sync") && request.method === "POST")).toBe(true);
});

it.each(["projects", "targets"])("shows the Snyk HTTP 403 and access checks for %s after testing the saved connection", async (resource) => {
  const fetchMock = vi.fn(async (url: string) => new Response(JSON.stringify(url.endsWith("/test")
    ? { error: { code: "snyk_upstream_error", upstream_status: 403, resource, message: "untrusted-upstream-secret" } }
    : configuration), { status: url.endsWith("/test") ? 502 : 200 }));
  vi.stubGlobal("fetch", fetchMock);
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><SnykPage /></QueryClientProvider>);
  await waitFor(() => expect(screen.getByRole("button", { name: "Test connection" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(`Snyk denied access to ${resource} (HTTP 403). Ask your Snyk administrator to check View Projects (org.project.read) and organization API access for this token in the configured organization.`);
  expect(screen.getByRole("alert")).not.toHaveTextContent("untrusted-upstream-secret");
  expect(fetchMock).toHaveBeenCalledWith("/api/v1/integrations/snyk/test", expect.objectContaining({ method: "POST" }));
  expect(screen.getByLabelText("API token")).toHaveValue("");
});

it.each([
  { resource: "dependency_graph", status: 429, label: "dependency graphs" },
  { resource: "projects", status: 204, label: "projects" },
  { resource: "targets", status: 302, label: "targets" }
])("shows the safe upstream resource and HTTP $status", async ({ resource, status, label }) => {
  vi.stubGlobal("fetch", vi.fn(async (url: string) => new Response(JSON.stringify(url.endsWith("/test")
    ? { error: { code: "snyk_upstream_error", upstream_status: status, resource } }
    : configuration), { status: url.endsWith("/test") ? 502 : 200 })));
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><SnykPage /></QueryClientProvider>);
  await waitFor(() => expect(screen.getByRole("button", { name: "Test connection" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(`Snyk could not read ${label} (HTTP ${status}). Check the Snyk connection and try again.`);
});

it.each([
  { name: "plain text", status: 502, body: "internal-error-secret" },
  { name: "invalid JSON", status: 502, body: '{"error":"internal-error-secret"' },
  { name: "unknown code", status: 502, body: JSON.stringify({ error: { code: "internal-error-secret", upstream_status: 403, resource: "projects" } }) },
  { name: "unknown resource", status: 502, body: JSON.stringify({ error: { code: "snyk_upstream_error", upstream_status: 403, resource: "internal-error-secret" } }) },
  { name: "invalid status", status: 502, body: JSON.stringify({ error: { code: "snyk_upstream_error", upstream_status: 200, resource: "projects" } }) },
  { name: "local administrator denial", status: 403, body: JSON.stringify({ error: { code: "snyk_upstream_error", upstream_status: 403, resource: "projects" } }) }
])("preserves the safe fallback for $name", async ({ status, body }) => {
  vi.stubGlobal("fetch", vi.fn(async (url: string) => url.endsWith("/test")
    ? new Response(body, { status })
    : new Response(JSON.stringify(configuration), { status: 200 })));
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><SnykPage /></QueryClientProvider>);
  await waitFor(() => expect(screen.getByRole("button", { name: "Test connection" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(status === 403
    ? "Administrator access is required."
    : "Snyk request failed. Check the organization ID, region, token permissions and deployment settings.");
  expect(screen.getByRole("alert")).not.toHaveTextContent("internal-error-secret");
});
