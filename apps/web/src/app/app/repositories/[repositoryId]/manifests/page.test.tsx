import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import RepositoryManifestsPage from "./page";

const mocks = vi.hoisted(() => ({
  listRepositoriesServer: vi.fn(),
  listRepositoryManifestsServer: vi.fn()
}));

vi.mock("@/lib/queries", () => ({
  listRepositoriesServer: mocks.listRepositoriesServer,
  listRepositoryManifestsServer: mocks.listRepositoryManifestsServer
}));

describe("RepositoryManifestsPage", () => {
  it("renders manifest lifecycle rows for the repository", async () => {
    mocks.listRepositoriesServer.mockResolvedValue([
      {
        id: "repo-1",
        name: "Repo One",
        url: "https://example.com/repo.git",
        default_branch: "main"
      }
    ]);
    mocks.listRepositoryManifestsServer.mockResolvedValue({
      items: [
        {
          id: "manifest-1",
          path: "package-lock.json",
          first_seen_at: "2026-05-08T10:00:00Z",
          last_seen_at: "2026-05-09T10:00:00Z",
          disappeared_at: null,
          is_active: true,
          labels: {}
        },
        {
          id: "manifest-2",
          path: "Cargo.lock",
          first_seen_at: "2026-05-08T10:00:00Z",
          last_seen_at: "2026-05-08T10:00:00Z",
          disappeared_at: "2026-05-09T10:00:00Z",
          is_active: false,
          labels: {}
        }
      ]
    });

    render(await RepositoryManifestsPage({ params: Promise.resolve({ repositoryId: "repo-1" }) }));

    expect(screen.getByRole("link", { name: /repositories/i })).toHaveAttribute("href", "/app/repositories");
    expect(screen.getByText("Repo One")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "https://example.com/repo.git" })).toHaveAttribute(
      "href",
      "https://example.com/repo.git"
    );
    expect(screen.getByText("package-lock.json")).toBeInTheDocument();
    expect(screen.getByText("Cargo.lock")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("Disappeared")).toBeInTheDocument();
  });

  it("renders an empty state when the repository has no manifest files", async () => {
    mocks.listRepositoriesServer.mockResolvedValue([
      {
        id: "repo-1",
        name: "Repo One",
        url: "https://example.com/repo.git",
        default_branch: "main"
      }
    ]);
    mocks.listRepositoryManifestsServer.mockResolvedValue({ items: [] });

    render(await RepositoryManifestsPage({ params: Promise.resolve({ repositoryId: "repo-1" }) }));

    expect(screen.getByText("No manifest files found for this repository.")).toBeInTheDocument();
  });
});
