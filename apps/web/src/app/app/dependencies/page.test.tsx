import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import DependenciesPage from "./page";

vi.mock("@/lib/queries", () => ({
  listDependenciesServer: vi.fn().mockResolvedValue({
    items: [
      {
        raw: "react@19.1.0",
        name: "react",
        version: "19.1.0",
        constraint: "",
        repository_count: 2,
        manifest_file_count: 3
      },
      {
        raw: "internal-lib ^2",
        name: "internal-lib",
        version: "",
        constraint: "^2",
        repository_count: 1,
        manifest_file_count: 1
      },
      {
        raw: "raw-only-entry",
        name: "",
        version: "",
        constraint: "",
        repository_count: 1,
        manifest_file_count: 1
      }
    ]
  })
}));

describe("DependenciesPage", () => {
  it("renders the dependency table with display fallbacks", async () => {
    render(await DependenciesPage());

    expect(screen.getByRole("columnheader", { name: "Dependency" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Repositories" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Manifest files" })).toBeInTheDocument();

    const reactRow = screen.getByRole("row", { name: /react@19\.1\.0 2 3/i });
    expect(within(reactRow).getByText("react@19.1.0")).toBeInTheDocument();

    expect(screen.getByText("internal-lib ^2")).toBeInTheDocument();
    expect(screen.getByText("raw-only-entry")).toBeInTheDocument();
  });
});
