import React from "react";

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ScanManifestItem } from "@/lib/types";
import { ScanInventory } from "./scan-inventory";

const manifest: ScanManifestItem = {
  id: "manifest-1",
  type: "npm",
  path: "package-lock.json",
  warnings: [],
  dependencies: Array.from({ length: 101 }, (_, index) => ({
    id: `dependency-${index}`,
    name: `package-${index}`,
    raw: `package-${index}@1.0.0`,
    version: "1.0.0",
    constraint: "",
    section: "",
    source: "registry",
    extras: {}
  }))
};

describe("ScanInventory", () => {
  it("mounts at most 100 dependencies only after opening a manifest", () => {
    render(<ScanInventory manifests={[manifest]} />);

    expect(screen.getByText("npm · 101 dependencies")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.queryByText("package-0")).not.toBeInTheDocument();

    const details = screen.getByText("package-lock.json").closest("details");
    if (!details) throw new Error("Missing manifest disclosure");
    details.open = true;
    fireEvent(details, new Event("toggle"));

    expect(screen.getAllByRole("row")).toHaveLength(101);
    expect(screen.getByText("package-0")).toBeInTheDocument();
    expect(screen.queryByText("package-100")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getAllByRole("row")).toHaveLength(2);
    expect(screen.getByText("package-100")).toBeInTheDocument();
    expect(screen.queryByText("package-0")).not.toBeInTheDocument();

    details.open = false;
    fireEvent(details, new Event("toggle"));
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows the empty scan state", () => {
    render(<ScanInventory manifests={[]} />);
    expect(screen.getByText("No manifests found for this scan.")).toBeInTheDocument();
  });

  it("shows local paths, editable installs, remote destinations and original declarations", () => {
    const dependency = {
      id: "python-path", name: "python-local", raw: "-e ./python", version: "", constraint: "", section: "", source: "path", extras: { source_path: "./python", path: "ignored-path", editable: "true" }
    };
    render(<ScanInventory manifests={[{
      ...manifest,
      dependencies: [
        dependency,
        { ...dependency, id: "deno-path", name: "deno-local", raw: "local@./deno.ts", extras: { path: "./deno.ts", editable: "false" } },
        { ...dependency, id: "git", name: "git-alias", raw: "git-alias @ git+ssh://git@example.test/repo.git@main", source: "git", extras: { source_url: "git+ssh://git@example.test/repo.git", editable: "true" } },
        { ...dependency, id: "url", name: "url-alias", raw: "url-alias @ https://example.test/package.whl", source: "url", extras: { source_url: "https://example.test/package.whl" } },
        { ...dependency, id: "unknown", name: "unknown-origin", raw: "unknown-origin declaration", source: "", extras: null },
        { ...dependency, id: "future", name: "future", raw: "future declaration", source: "future-origin", extras: {} }
      ]
    }]} />);
    const details = screen.getByText("package-lock.json").closest("details");
    if (!details) throw new Error("Missing manifest disclosure");
    details.open = true;
    fireEvent(details, new Event("toggle"));

    expect(screen.getByRole("columnheader", { name: "Origin" })).toBeInTheDocument();
    const python = within(screen.getByRole("row", { name: /python-local/ }));
    expect(python.getByText("Local path")).toBeInTheDocument();
    expect(python.getByText("Path: ./python")).toBeInTheDocument();
    expect(python.getByText("Editable")).toBeInTheDocument();
    expect(python.getByText("Raw: -e ./python")).toBeInTheDocument();
    expect(screen.queryByText(/ignored-path/)).not.toBeInTheDocument();
    const deno = within(screen.getByRole("row", { name: /deno-local/ }));
    expect(deno.getByText("Path: ./deno.ts")).toBeInTheDocument();
    expect(deno.queryByText("Editable")).not.toBeInTheDocument();
    const git = within(screen.getByRole("row", { name: /git-alias/ }));
    expect(git.getByText("Git")).toBeInTheDocument();
    expect(git.getByText("Destination: git+ssh://git@example.test/repo.git")).toBeInTheDocument();
    expect(git.getByText("Raw: git-alias @ git+ssh://git@example.test/repo.git@main")).toBeInTheDocument();
    expect(screen.getByText("Destination: https://example.test/package.whl")).toBeInTheDocument();
    expect(screen.getByText("Unknown")).toBeInTheDocument();
    expect(screen.getByText("Raw: unknown-origin declaration")).toBeInTheDocument();
    expect(screen.getByText("future-origin")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /example.test/ })).not.toBeInTheDocument();
  });
});
