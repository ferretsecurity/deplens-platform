import React from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import type { SnykInventory } from "@/lib/snyk";
import { SnykInventoryView } from "./snyk-inventory";

function source(id: string, count: number): SnykInventory["sources"][number] {
  return {
    id, available: true, error: "", synced_at: null,
    snapshot: {
      files: ["package.json"],
      metadata: { branch: "main", commit_hash: "", integration_repo_url: "", html_report_url: "", incomplete: true },
      projects: [{ id: `${id}-project`, name: `${id}-project`, type: "npm", target_file: "package.json", branch: "main", url: "https://app.snyk.io/org/test/project/example" }],
      artifacts: Array.from({ length: count }, (_, i) => ({
        id: `${id}-${i}`, name: `${id}-${i}`, type: "npm", namespace: "", version: "1",
        direct: i === 0, dev: null, manifestFiles: [{ file: "package.json" }]
      }))
    }
  };
}

it("shows Snyk projects, links, manifest references and incomplete graph notices", () => {
  render(<SnykInventoryView sources={[source("app", 1)]} />);
  expect(screen.getByRole("link", { name: "app-project ↗" })).toHaveAttribute("href", "https://app.snyk.io/org/test/project/example");
  expect(screen.getByText("app-0@1")).toBeVisible();
  expect(screen.getByText("npm · Direct")).toBeVisible();
  expect(screen.getAllByText("package.json")).toHaveLength(2);
  expect(screen.getByText(/Dependency totals are incomplete/)).toBeVisible();
});

it("paginates sources independently and resets filtering to the first page", () => {
  render(<SnykInventoryView sources={[source("large", 51), source("small", 1)]} />);
  fireEvent.click(screen.getAllByRole("button", { name: "Next Snyk results" })[0]);
  expect(screen.getByText("large-50@1")).toBeVisible();
  expect(screen.getByText("small-0@1")).toBeVisible();
  fireEvent.change(screen.getAllByRole("textbox", { name: "Filter Snyk inventory" })[0], { target: { value: "large-0" } });
  expect(screen.getByText("large-0@1")).toBeVisible();
  expect(screen.queryByText("large-50@1")).not.toBeInTheDocument();
});
