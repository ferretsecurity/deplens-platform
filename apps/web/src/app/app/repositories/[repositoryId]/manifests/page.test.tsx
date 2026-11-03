import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import RepositoryManifestsPage from "./page";

const mocks = vi.hoisted(() => ({
  getRepositoryInventoryServer: vi.fn(),
  listRepositoryManifestsServer: vi.fn()
}));

vi.mock("@/lib/queries", () => ({
  getRepositoryInventoryServer: mocks.getRepositoryInventoryServer,
  getRepositorySnykInventoryServer: vi.fn().mockResolvedValue({ repository: { id: "repo-1", name: "Repo One", url: "https://example.com/repo.git", default_branch: "main", cli: true, socket: false, socket_url: "", snyk: true, snyk_url: "https://app.snyk.io/org/test" }, sources: [] }),
  listRepositoryManifestsServer: mocks.listRepositoryManifestsServer
}));

vi.mock("@/lib/auth.server", () => ({
  fetchCurrentUserServer: vi.fn().mockResolvedValue({ role: "viewer" })
}));

describe("RepositoryManifestsPage", () => {
  it("renders manifest lifecycle rows for the repository", async () => {
    mocks.getRepositoryInventoryServer.mockResolvedValue({
      repository: {
        id: "repo-1",
        name: "Repo One",
        url: "https://example.com/repo.git",
        default_branch: "main"
      },
      sources: []
    });
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

    expect(screen.getByRole("link", { name: /repositories/i })).toHaveAttribute(
      "href",
      "/app/repositories"
    );
    expect(screen.getByText("Repo One")).toBeInTheDocument();
    expect(screen.getByLabelText("Open Snyk dashboard")).toBeInTheDocument();
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
    mocks.getRepositoryInventoryServer.mockResolvedValue({
      repository: {
        id: "repo-1",
        name: "Repo One",
        url: "https://example.com/repo.git",
        default_branch: "main"
      },
      sources: []
    });
    mocks.listRepositoryManifestsServer.mockResolvedValue({ items: [] });

    render(await RepositoryManifestsPage({ params: Promise.resolve({ repositoryId: "repo-1" }) }));

    expect(
      screen.getByText("No CLI manifest files found for this repository.")
    ).toBeInTheDocument();
  });
});
