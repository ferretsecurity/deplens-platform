import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { PackageListResponse } from "@/lib/packages";
import type { listPackagesServer } from "@/lib/queries";
import PackagesPage from "./page";

const mocks = vi.hoisted(() => ({ listPackagesServer: vi.fn<typeof listPackagesServer>() }));
vi.mock("@/lib/queries", () => mocks);

const response = {
  items: [
    { source: "cli", name: "@scope/react", version: "19", constraint: "^19", ecosystem: "", repository_id: "11111111-1111-1111-1111-111111111111", repository_name: "CLI repo", manifest_path: "packages/app/package.json" },
    { source: "socket", name: "@scope/react", version: "19", constraint: "", ecosystem: "npm", repository_id: "22222222-2222-2222-2222-222222222222", repository_name: "Socket repo", manifest_path: null }
  ],
  pagination: { page: 1, page_size: 25, total: 2, total_pages: 1, has_previous: false, has_next: false },
  filters: { q: "" }
} satisfies PackageListResponse;

describe("PackagesPage", () => {
  it("shows package usages with source, repository and manifest file", async () => {
    mocks.listPackagesServer.mockResolvedValue(response);
    render(await PackagesPage({}));
    expect(mocks.listPackagesServer).toHaveBeenCalledWith({ q: "", page: 1, page_size: 25 });
    expect(screen.getByText("Showing 1-2 of 2 package usages")).toBeInTheDocument();
    expect(screen.getAllByText("@scope/react")).toHaveLength(2);
    expect(screen.getByText("19 / ^19")).toBeInTheDocument();
    expect(screen.getByText("CLI")).toBeInTheDocument();
    expect(screen.getByText("Socket")).toBeInTheDocument();
    expect(screen.getByText("npm")).toBeInTheDocument();
    expect(screen.getByText("packages/app/package.json")).toBeInTheDocument();
    expect(screen.getByText("Not reported")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Socket repo" })).toHaveAttribute("href", "/app/repositories/22222222-2222-2222-2222-222222222222/manifests");
  });

  it("keeps literal searches and page size in page links and resets the page when searching", async () => {
    mocks.listPackagesServer.mockResolvedValue({
      ...response,
      pagination: { page: 2, page_size: 10, total: 30, total_pages: 3, has_previous: true, has_next: true },
      filters: { q: "@scope/react%" }
    });
    render(await PackagesPage({ searchParams: Promise.resolve({ q: " @scope/react% ", page: "2", page_size: "10" }) }));
    expect(mocks.listPackagesServer).toHaveBeenCalledWith({ q: "@scope/react%", page: 2, page_size: 10 });
    const input = screen.getByRole("searchbox", { name: "Search packages" });
    expect(input).toHaveValue("@scope/react%");
    const form = input.closest("form");
    expect(form).toHaveAttribute("method", "get");
    expect(form).toHaveAttribute("action", "/app/packages");
    expect(form?.querySelector('[name="page"]')).toBeNull();
    expect(form?.querySelector('[name="page_size"]')).toHaveValue("10");
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute("href", "/app/packages?q=%40scope%2Freact%25&page_size=10");
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute("href", "/app/packages?q=%40scope%2Freact%25&page=3&page_size=10");
    expect(screen.getByRole("link", { name: "Clear" })).toHaveAttribute("href", "/app/packages");
  });

  it.each([
    { q: "missing", message: "No packages match this search." },
    { q: "", message: "No active packages found." }
  ])("shows an empty state for '$q'", async ({ q, message }) => {
    mocks.listPackagesServer.mockResolvedValue({ items: [], pagination: { page: 1, page_size: 25, total: 0, total_pages: 0, has_previous: false, has_next: false }, filters: { q } });
    render(await PackagesPage({ searchParams: Promise.resolve({ q }) }));
    expect(screen.getByText(message)).toBeInTheDocument();
    expect(screen.getByText("Showing 0-0 of 0 package usages")).toBeInTheDocument();
  });

  it("lets the user return from an empty page beyond the last result", async () => {
    mocks.listPackagesServer.mockResolvedValue({ ...response, items: [], pagination: { page: 9, page_size: 25, total: 2, total_pages: 1, has_previous: true, has_next: false } });
    render(await PackagesPage({ searchParams: Promise.resolve({ page: "9" }) }));
    expect(screen.getByText("No package usages on this page.")).toBeInTheDocument();
    expect(screen.getByText("Showing 0-0 of 2 package usages")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute("href", "/app/packages?page=8");
  });
});
