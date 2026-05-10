import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import RepositoriesPage from "./page";

vi.mock("@/lib/queries", () => ({
  listRepositoriesServer: vi.fn().mockResolvedValue([
    {
      id: "repo-1",
      name: "Repo One",
      url: "https://example.com/repo.git",
      default_branch: "main"
    }
  ])
}));

describe("RepositoriesPage", () => {
  it("renders the repository table", async () => {
    render(await RepositoriesPage());

    expect(screen.getByText("Repo One")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "https://example.com/repo.git" })).toHaveAttribute(
      "href",
      "https://example.com/repo.git"
    );
    expect(screen.getByRole("link", { name: /open scans/i })).toHaveAttribute(
      "href",
      "/app/scans?repository_id=repo-1"
    );
  });
});
