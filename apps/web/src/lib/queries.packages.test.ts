import { describe, expect, it, vi } from "vitest";

import { listPackagesServer } from "./queries";

const mocks = vi.hoisted(() => ({
  serverApiFetch: vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<unknown>>()
}));
vi.mock("./api", () => mocks);
vi.mock("./env", () => ({ env: { API_INTERNAL_BASE_URL: "http://api.example" } }));
vi.mock("next/headers", () => ({ cookies: async () => ({ toString: () => "deplens_session=test" }) }));

const response = {
  items: [{ source: "socket", name: "@scope/react", version: "19", constraint: "", ecosystem: "npm", repository_id: "11111111-1111-1111-1111-111111111111", repository_name: "Repo", manifest_path: null }],
  pagination: { page: 2, page_size: 10, total: 20, total_pages: 2, has_previous: true, has_next: false },
  filters: { q: "@scope/react%" }
};

describe("listPackagesServer", () => {
  it("encodes the search and pagination and forwards the session", async () => {
    mocks.serverApiFetch.mockResolvedValue(response);
    await expect(listPackagesServer({ q: "@scope/react%", page: 2, page_size: 10 })).resolves.toEqual(response);
    expect(mocks.serverApiFetch).toHaveBeenCalledWith("http://api.example/api/v1/packages?q=%40scope%2Freact%25&page=2&page_size=10", { headers: { cookie: "deplens_session=test" } });
  });

  it("rejects an invalid package response at the API boundary", async () => {
    mocks.serverApiFetch.mockResolvedValue({ ...response, items: [{ ...response.items[0], source: "unexpected" }] });
    await expect(listPackagesServer({ q: "", page: 1, page_size: 25 })).rejects.toThrow();
    mocks.serverApiFetch.mockResolvedValue({ ...response, pagination: { ...response.pagination, total: -1 } });
    await expect(listPackagesServer({ q: "", page: 1, page_size: 25 })).rejects.toThrow();
  });
});
