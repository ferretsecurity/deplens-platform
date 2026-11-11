import { describe, expect, it, vi } from "vitest";

import { listPackagesServer, listRepositoryOptionsServer } from "./queries";

const mocks = vi.hoisted(() => ({
  serverApiFetch: vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<unknown>>()
}));
vi.mock("./api", () => mocks);
vi.mock("./env", () => ({ env: { API_INTERNAL_BASE_URL: "http://api.example" } }));
vi.mock("next/headers", () => ({ cookies: async () => ({ toString: () => "deplens_session=test" }) }));

const response = {
  items: [{ source: "socket", name: "@scope/react", origin_kind: "", destination: "", version: "19", constraint: "", ecosystem: "npm", repository_id: "11111111-1111-1111-1111-111111111111", repository_name: "Repo", manifest_path: null }],
  pagination: { page: 2, page_size: 10, total: 20, total_pages: 2, has_previous: true, has_next: false },
  filters: { version: "1.103.1", q: "@scope/react%" }
};

describe("listPackagesServer", () => {
  it("encodes the search and pagination and forwards the session", async () => {
    mocks.serverApiFetch.mockResolvedValue(response);
    await expect(listPackagesServer({ q: "@scope/react%", version: "1.103.1", page: 2, page_size: 10 })).resolves.toEqual(response);
    expect(mocks.serverApiFetch).toHaveBeenCalledWith("http://api.example/api/v1/packages?q=%40scope%2Freact%25&version=1.103.1&page=2&page_size=10", { headers: { cookie: "deplens_session=test" } });
  });

  it("rejects an invalid package response at the API boundary", async () => {
    mocks.serverApiFetch.mockResolvedValue({ ...response, items: [{ ...response.items[0], source: "unexpected" }] });
    await expect(listPackagesServer({ q: "", version: "", page: 1, page_size: 25 })).rejects.toThrow();
    mocks.serverApiFetch.mockResolvedValue({ ...response, pagination: { ...response.pagination, total: -1 } });
    await expect(listPackagesServer({ q: "", version: "", page: 1, page_size: 25 })).rejects.toThrow();
  });

  it("accepts unnamed remote dependencies and preserves future origin kinds", async () => {
    const remoteResponse = {
      ...response,
      items: [
        { ...response.items[0], source: "cli", name: "", origin_kind: "url", destination: "https://example.test/package.whl" },
        { ...response.items[0], source: "cli", origin_kind: "future-origin" }
      ]
    };
    mocks.serverApiFetch.mockResolvedValue(remoteResponse);
    await expect(listPackagesServer({ q: "", version: "", page: 1, page_size: 25 })).resolves.toEqual(remoteResponse);
  });

  it.each(["origin_kind", "destination"])("requires the %s API field", async (field) => {
    mocks.serverApiFetch.mockResolvedValue({ ...response, items: [{ ...response.items[0], [field]: undefined }] });
    await expect(listPackagesServer({ q: "", version: "", page: 1, page_size: 25 })).rejects.toThrow();
  });
});

it("reads repository options through the last response page", async () => {
  mocks.serverApiFetch.mockReset();
  mocks.serverApiFetch.mockResolvedValueOnce({ items: [{ id: "first" }], pagination: { page: 1, page_size: 100, has_next: true } })
    .mockResolvedValueOnce({ items: [{ id: "beyond100" }], pagination: { page: 2, page_size: 100, has_next: false } });
  await expect(listRepositoryOptionsServer()).resolves.toEqual([{ id: "first" }, { id: "beyond100" }]);
  expect(mocks.serverApiFetch).toHaveBeenCalledTimes(2);
  expect(mocks.serverApiFetch).toHaveBeenLastCalledWith("http://api.example/api/v1/repositories?page=2&page_size=100", { headers: { cookie: "deplens_session=test" } });
});
