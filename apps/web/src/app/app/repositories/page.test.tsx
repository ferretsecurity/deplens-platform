import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import RepositoriesPage from "./page";

const mocks = vi.hoisted(() => ({
  listRepositoriesServer: vi.fn()
}));

vi.mock("@/lib/queries", () => ({
  listRepositoriesServer: mocks.listRepositoriesServer
}));

describe("RepositoriesPage", () => {
  it("renders the repository table", async () => {
    mocks.listRepositoriesServer.mockResolvedValue({
      items: [
        {
          id: "repo-1",
          name: "Repo One",
          url: "https://example.com/repo.git",
          default_branch: "main"
        }
      ],
      pagination: {
        page: 1,
        page_size: 25,
        total: 1,
        total_pages: 1,
        has_previous: false,
        has_next: false
      },
      filters: { q: "" }
    });

    render(await RepositoriesPage({ searchParams: Promise.resolve({}) }));

    expect(screen.getByText("Repo One")).toBeInTheDocument();
    expect(screen.getByText("Showing 1-1 of 1 repositories")).toBeInTheDocument();
    expect(screen.getByRole("searchbox", { name: /search repositories/i })).toHaveValue("");
    expect(screen.getByRole("link", { name: "https://example.com/repo.git" })).toHaveAttribute(
      "href",
      "https://example.com/repo.git"
    );
    expect(screen.getByRole("link", { name: /open scans/i })).toHaveAttribute(
      "href",
      "/app/scans?repository_id=repo-1"
    );
    expect(screen.getByRole("link", { name: /manifest files/i })).toHaveAttribute(
      "href",
      "/app/repositories/repo-1/manifests"
    );
    expect(mocks.listRepositoriesServer).toHaveBeenCalledWith({ q: "", page: 1, page_size: 25 });
  });

  it("preserves search parameters in pagination links", async () => {
    mocks.listRepositoriesServer.mockResolvedValue({
      items: [
        {
          id: "repo-2",
          name: "Repo Two",
          url: "https://example.com/repo-two.git",
          default_branch: "main"
        }
      ],
      pagination: {
        page: 2,
        page_size: 25,
        total: 60,
        total_pages: 3,
        has_previous: true,
        has_next: true
      },
      filters: { q: "Repo" }
    });

    render(await RepositoriesPage({ searchParams: Promise.resolve({ q: "Repo", page: "2" }) }));

    expect(screen.getByRole("searchbox", { name: /search repositories/i })).toHaveValue("Repo");
    expect(screen.getByRole("link", { name: /previous/i })).toHaveAttribute("href", "/app/repositories?q=Repo");
    expect(screen.getByRole("link", { name: /next/i })).toHaveAttribute(
      "href",
      "/app/repositories?q=Repo&page=3"
    );
  });

  it("renders a search-specific empty state", async () => {
    mocks.listRepositoriesServer.mockResolvedValue({
      items: [],
      pagination: {
        page: 1,
        page_size: 25,
        total: 0,
        total_pages: 0,
        has_previous: false,
        has_next: false
      },
      filters: { q: "missing" }
    });

    render(await RepositoriesPage({ searchParams: Promise.resolve({ q: "missing" }) }));

    expect(screen.getByText("No repositories match this search.")).toBeInTheDocument();
  });
});
