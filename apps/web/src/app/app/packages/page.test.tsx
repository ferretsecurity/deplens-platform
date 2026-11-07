import React from "react";
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api";
import type { PackageListResponse } from "@/lib/packages";
import type { listPackagesServer } from "@/lib/queries";
import PackagesPage from "./page";

const mocks = vi.hoisted(() => ({ listPackagesServer: vi.fn<typeof listPackagesServer>() }));
vi.mock("@/lib/queries", () => mocks);
vi.mock("@/components/empty-inventory", () => ({ EmptyInventory: () => <p>Upload guidance</p> }));

const response = {
  items: [
    { source: "cli", name: "@scope/react", version: "19", constraint: "^19", ecosystem: "", repository_id: "11111111-1111-1111-1111-111111111111", repository_name: "CLI repo", manifest_path: "packages/app/package.json" },
    { source: "socket", name: "@scope/react", version: "19", constraint: "", ecosystem: "npm", repository_id: "22222222-2222-2222-2222-222222222222", repository_name: "Socket repo", manifest_path: null }
  ],
  pagination: { page: 1, page_size: 25, total: 2, total_pages: 1, has_previous: false, has_next: false },
  filters: { version: "", q: "" }
} satisfies PackageListResponse;

describe("PackagesPage", () => {
  it("keeps the form available when a version is rejected", async () => {
    mocks.listPackagesServer.mockRejectedValueOnce(new ApiError(400, "invalid version"));
    render(await PackagesPage({ searchParams: Promise.resolve({ q: "litellm", version: "latest" }) }));
    expect(screen.getByRole("alert")).toHaveTextContent("Enter a concrete version");
    expect(screen.getByRole("textbox", { name: "Version" })).toHaveValue("latest");
    expect(screen.getByRole("button", { name: "Search" })).toBeInTheDocument();
    expect(screen.queryByText(/Showing/)).not.toBeInTheDocument();
  });

  it("does not hide server failures as invalid input", async () => {
    mocks.listPackagesServer.mockRejectedValueOnce(new ApiError(500, "server error"));
    await expect(PackagesPage({})).rejects.toThrow("server error");
  });

  it("shows package usages with source, repository and manifest file", async () => {
    mocks.listPackagesServer.mockResolvedValue(response);
    render(await PackagesPage({}));
    expect(mocks.listPackagesServer).toHaveBeenCalledWith({ q: "", version: "", page: 1, page_size: 25 });
    expect(screen.getByText("Showing 1-2 of 2 package usages")).toBeInTheDocument();
    expect(screen.getAllByText("@scope/react")).toHaveLength(1);
    expect(screen.getByRole("heading", { name: "@scope/react" })).toBeInTheDocument();
    expect(screen.getByText("2 usages on this page")).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Package" })).not.toBeInTheDocument();
    expect(screen.queryByText("Package groups may continue across pages.")).not.toBeInTheDocument();
    expect(screen.getByText("19 / ^19")).toBeInTheDocument();
    expect(screen.getByText("CLI")).toBeInTheDocument();
    expect(screen.getByText("Socket")).toBeInTheDocument();
    expect(screen.getByText("npm")).toBeInTheDocument();
    expect(screen.getByText("packages/app/package.json")).toBeInTheDocument();
    expect(screen.getByText("Not reported")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Socket repo" })).toHaveAttribute("href", "/app/repositories/22222222-2222-2222-2222-222222222222/manifests");
  });

  it("groups nonadjacent usages by exact package name while keeping each usage's details", async () => {
    mocks.listPackagesServer.mockResolvedValue({
      ...response,
      items: [
        response.items[0],
        { ...response.items[1], name: "react-dom", version: "", constraint: "", ecosystem: "npm" },
        response.items[1],
        { ...response.items[0], source: "snyk", version: "18", constraint: "^18", ecosystem: "yarn", repository_name: "Snyk repo", manifest_path: "frontend/package.json" },
        { ...response.items[1], name: "React", ecosystem: "nuget" }
      ],
      pagination: { ...response.pagination, total: 5 }
    });
    render(await PackagesPage({}));

    expect(screen.getAllByRole("heading", { level: 2 }).map((heading) => heading.textContent)).toEqual(["@scope/react", "react-dom", "React"]);
    const react = within(screen.getByRole("region", { name: "@scope/react" }));
    expect(react.getAllByRole("row")).toHaveLength(4);
    expect(react.getByText("3 usages on this page")).toBeInTheDocument();
    expect(react.getByText("19 / ^19")).toBeInTheDocument();
    expect(react.getByText("19")).toBeInTheDocument();
    expect(react.getByText("18 / ^18")).toBeInTheDocument();
    expect(react.getByText("Unspecified")).toBeInTheDocument();
    expect(react.getByText("npm")).toBeInTheDocument();
    expect(react.getByText("yarn")).toBeInTheDocument();
    expect(react.getByText("CLI")).toBeInTheDocument();
    expect(react.getByText("Socket")).toBeInTheDocument();
    expect(react.getByText("Snyk")).toBeInTheDocument();
    expect(react.getByRole("link", { name: "CLI repo" })).toHaveAttribute("href", "/app/repositories/11111111-1111-1111-1111-111111111111/manifests");
    expect(react.getByRole("link", { name: "Socket repo" })).toHaveAttribute("href", "/app/repositories/22222222-2222-2222-2222-222222222222/manifests");
    expect(react.getByRole("link", { name: "Snyk repo" })).toHaveAttribute("href", "/app/repositories/11111111-1111-1111-1111-111111111111/manifests");
    expect(react.getByText("packages/app/package.json")).toBeInTheDocument();
    expect(react.getByText("Not reported")).toBeInTheDocument();
    expect(react.getByText("frontend/package.json")).toBeInTheDocument();
    const reactDom = within(screen.getByRole("region", { name: "react-dom" }));
    expect(reactDom.getAllByRole("row")).toHaveLength(2);
    expect(reactDom.getByText("1 usage on this page")).toBeInTheDocument();
    expect(reactDom.getByText("Unspecified")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "React" })).getByText("nuget")).toBeInTheDocument();
  });

  it("keeps literal searches and page size in page links and resets the page when searching", async () => {
    mocks.listPackagesServer.mockResolvedValue({
      ...response,
      pagination: { page: 2, page_size: 10, total: 30, total_pages: 3, has_previous: true, has_next: true },
      filters: { version: "", q: "@scope/react%" }
    });
    render(await PackagesPage({ searchParams: Promise.resolve({ q: " @scope/react% ", version: " 1.103.1 ", page: "2", page_size: "10" }) }));
    expect(mocks.listPackagesServer).toHaveBeenCalledWith({ q: "@scope/react%", version: "1.103.1", page: 2, page_size: 10 });
    expect(screen.getByText("Showing 11-12 of 30 package usages")).toBeInTheDocument();
    expect(screen.getByText("2 usages on this page")).toBeInTheDocument();
    expect(screen.getByText("Package groups may continue across pages.")).toBeInTheDocument();
    const input = screen.getByRole("searchbox", { name: "Search packages" });
    expect(screen.getByRole("textbox", { name: "Version" })).toHaveValue("1.103.1");
    expect(input).toHaveValue("@scope/react%");
    const form = input.closest("form");
    expect(form).toHaveAttribute("method", "get");
    expect(form).toHaveAttribute("action", "/app/packages");
    expect(form?.querySelector('[name="page"]')).toBeNull();
    expect(form?.querySelector('[name="page_size"]')).toHaveValue("10");
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute("href", "/app/packages?q=%40scope%2Freact%25&version=1.103.1&page_size=10");
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute("href", "/app/packages?q=%40scope%2Freact%25&version=1.103.1&page=3&page_size=10");
    expect(screen.getByRole("link", { name: "Clear" })).toHaveAttribute("href", "/app/packages");
  });

  it.each([
    { q: "missing", message: "No packages match this search." },
    { q: "", message: "No active packages found." }
  ])("shows an empty state for '$q'", async ({ q, message }) => {
    mocks.listPackagesServer.mockResolvedValue({ items: [], pagination: { page: 1, page_size: 25, total: 0, total_pages: 0, has_previous: false, has_next: false }, filters: { q, version: "" } });
    render(await PackagesPage({ searchParams: Promise.resolve({ q }) }));
    expect(screen.getByText(message)).toBeInTheDocument();
    expect(screen.getByText("Showing 0-0 of 0 package usages")).toBeInTheDocument();
  });

  it("allows clearing a version-only search and explains an empty result", async () => {
    mocks.listPackagesServer.mockResolvedValue({ ...response, items: [], pagination: { ...response.pagination, total: 0, total_pages: 0 }, filters: { q: "", version: "2.0.0" } });
    render(await PackagesPage({ searchParams: Promise.resolve({ version: "2.0.0" }) }));
    expect(mocks.listPackagesServer).toHaveBeenCalledWith({ q: "", version: "2.0.0", page: 1, page_size: 25 });
    expect(screen.getByText("No packages match this search.")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Version" })).toHaveValue("2.0.0");
    expect(screen.getByRole("link", { name: "Clear" })).toHaveAttribute("href", "/app/packages");
  });

  it("lets the user return from an empty page beyond the last result", async () => {
    mocks.listPackagesServer.mockResolvedValue({ ...response, items: [], pagination: { page: 9, page_size: 25, total: 2, total_pages: 1, has_previous: true, has_next: false } });
    render(await PackagesPage({ searchParams: Promise.resolve({ page: "9" }) }));
    expect(screen.getByText("No package usages on this page.")).toBeInTheDocument();
    expect(screen.getByText("Showing 0-0 of 2 package usages")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute("href", "/app/packages?page=8");
  });
});
