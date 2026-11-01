import React from "react";

import { fireEvent, render, screen } from "@testing-library/react";
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
});
