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
