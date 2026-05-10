import { describe, expect, it, vi } from "vitest";

import { ApiError, clientApiFetch, serverApiFetch } from "./api";

describe("api helpers", () => {
  it("returns parsed JSON for successful requests", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 }))
    );

    await expect(clientApiFetch<{ ok: boolean }>("/test")).resolves.toEqual({ ok: true });
  });

  it("returns undefined for no-content responses", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(null, { status: 204 })));

    await expect(serverApiFetch<void>("/test")).resolves.toBeUndefined();
  });

  it("returns undefined for successful responses with an empty body", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(null, { status: 200 })));

    await expect(clientApiFetch<void>("/test")).resolves.toBeUndefined();
  });

  it("throws ApiError for failed responses", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("nope", { status: 401 })));

    await expect(clientApiFetch("/test")).rejects.toMatchObject<ApiError>({
      status: 401
    });
  });
});
