import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import DependenciesPage from "./page";

vi.mock("@/lib/queries", () => ({
  listDependenciesServer: vi.fn().mockResolvedValue({
    items: [
      {
        raw: "",
        name: "react",
        occurrence_count: 5,
        repository_count: 2,
        manifest_file_count: 2,
        lock_file_count: 2
      },
      {
        raw: "",
        name: "internal-lib",
        occurrence_count: 1,
        repository_count: 1,
        manifest_file_count: 1,
        lock_file_count: 0
      },
      {
        raw: "raw-only-entry",
        name: "",
        occurrence_count: 1,
        repository_count: 1,
        manifest_file_count: 0,
        lock_file_count: 0
      }
    ]
  })
}));

describe("DependenciesPage", () => {
  it("renders the dependency table with display fallbacks", async () => {
    render(await DependenciesPage());

    expect(screen.getByRole("columnheader", { name: "Dependency" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Occurrences" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Manifest files" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Lock files" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Repositories" })).toBeInTheDocument();

    const reactRow = screen.getByRole("row", { name: /react 5 2 2 2/i });
    expect(within(reactRow).getByText("react")).toBeInTheDocument();

    expect(screen.getByText("internal-lib")).toBeInTheDocument();
    expect(screen.getByText("raw-only-entry")).toBeInTheDocument();
  });
});
