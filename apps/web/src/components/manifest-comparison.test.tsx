import React from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { RepositoryManifestItem } from "@/lib/types";
import type { SnykInventory } from "@/lib/snyk";
import type { SocketInventory } from "@/lib/socket";
import { ManifestComparison } from "./manifest-comparison";

function manifest(path: string, active = true): RepositoryManifestItem {
  return {
    id: path, path, is_active: active, labels: {},
    first_seen_at: "2026-05-08T10:00:00Z", last_seen_at: "2026-05-09T10:00:00Z",
    disappeared_at: active ? null : "2026-05-09T10:00:00Z"
  };
}

function source(files: string[]): SocketInventory["sources"][number] {
  return {
    id: "socket-1", available: true, error: "", synced_at: "2026-05-09T10:00:00Z",
    snapshot: {
      files, artifacts: [],
      metadata: { branch: "main", commit_hash: "", integration_repo_url: "", html_report_url: "" }
    }
  };
}

describe("ManifestComparison", () => {
  it("aligns shared files and shows files found by only one source, including disappeared manifests", () => {
    render(<ManifestComparison
      manifests={[manifest("shared/package.json"), manifest("Cargo.lock"), manifest("old.json", false)]}
      sources={[source(["shared/package.json", "socket-only.json", "shared/package.json"])]}
    />);
    const table = screen.getByRole("table", { name: "Manifest file comparison" });
    expect(within(table).getAllByRole("columnheader").map((cell) => cell.textContent))
      .toEqual(["Path", "Deplens", "Socket", "Snyk"]);
    const shared = within(table).getByRole("row", { name: /shared\/\s*package.json/ });
    expect(within(shared).getByText("Active")).toBeVisible();
    expect(within(shared).getByText("Detected")).toBeVisible();
    expect(within(table).getAllByText("shared/package.json")).toHaveLength(1);
    const cliOnly = within(table).getByRole("row", { name: /Cargo.lock/ });
    expect(within(cliOnly).getByText("Not found in imported files")).toBeVisible();
    expect(within(cliOnly).getByText(/First seen:/)).toBeVisible();
    expect(within(cliOnly).getByText(/Last seen:/)).toBeVisible();
    const socketOnly = within(table).getByRole("row", { name: /socket-only.json/ });
    expect(within(socketOnly).getByText("Not found")).toBeVisible();
    expect(within(socketOnly).getByText("Detected")).toBeVisible();
    expect(within(table).getByRole("row", { name: /old.json/ })).toHaveTextContent("Disappeared");
  });

  it("distinguishes missing Socket data from files absent in an import", () => {
    const { rerender } = render(<ManifestComparison manifests={[manifest("package.json")]} sources={[]} />);
    expect(screen.getByText("No Socket source linked")).toBeVisible();
    rerender(<ManifestComparison manifests={[manifest("package.json")]}
      sources={[{ ...source([]), snapshot: null }]} />);
    expect(screen.getByText("Not imported yet")).toBeVisible();
    rerender(<ManifestComparison manifests={[manifest("package.json")]} sources={[source([])]} />);
    expect(screen.getByText("Not found in imported files")).toBeVisible();
  });

  it("keeps files from unavailable sources visible and flags their status", () => {
    render(<ManifestComparison manifests={[]} sources={[{ ...source(["package.json"]), available: false }]} />);
    expect(screen.getByText("package.json")).toBeVisible();
    expect(screen.getByText("Detected in unavailable source")).toBeVisible();
    expect(screen.getByText(/Some Socket sources are unavailable/)).toBeVisible();
  });

  it("paginates the combined paths and resets the page when filtering", () => {
    render(<ManifestComparison manifests={[]} sources={[source(
      Array.from({ length: 51 }, (_, i) => `packages/${String(i).padStart(2, "0")}/package.json`)
    )]} />);
    expect(screen.getByText("51 files · Page 1 of 2")).toBeVisible();
    expect(screen.queryByText("packages/50/package.json")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Next files" }));
    expect(screen.getByText("packages/50/package.json")).toBeVisible();
    fireEvent.change(screen.getByRole("textbox", { name: "Filter manifest paths" }), { target: { value: "PACKAGES/00" } });
    expect(screen.getByText("packages/00/package.json")).toBeVisible();
    expect(screen.queryByText("packages/50/package.json")).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "missing" } });
    expect(screen.getByText("No manifest paths match.")).toBeVisible();
  });
});

function snykSource(files: string[]): SnykInventory["sources"][number] {
  return {
    id: "snyk-1", available: true, error: "", synced_at: null,
    snapshot: {
      files, artifacts: [], projects: [],
      metadata: { branch: "main", commit_hash: "", integration_repo_url: "", html_report_url: "", incomplete: false }
    }
  };
}

it("aligns all three providers, retains Snyk-only paths, and flags incomplete inventories", () => {
  render(<ManifestComparison manifests={[manifest("shared.json")]}
    sources={[source(["shared.json", "socket.json"])]}
    snykSources={[{ ...snykSource(["shared.json", "snyk.json"]), available: false }]} />);
  const shared = screen.getByRole("row", { name: /shared.json/ });
  const sharedCells = within(shared).getAllByRole("cell");
  expect(sharedCells).toHaveLength(4);
  ["shared.json", /^Active/, "Detected", "Detected in unavailable source"].forEach((name, index) => {
    expect(sharedCells[index]).toHaveAccessibleName(name);
  });
  const snykOnly = screen.getByRole("row", { name: /snyk.json/ });
  const snykCells = within(snykOnly).getAllByRole("cell");
  expect(snykCells).toHaveLength(4);
  ["snyk.json", "Not found", "Not found in imported files", "Detected in unavailable source"].forEach((name, index) => {
    expect(snykCells[index]).toHaveAccessibleName(name);
  });
  expect(screen.getByText(/Some Snyk sources are unavailable/)).toBeVisible();
});

it("shows that Snyk dependency graphs may be incomplete", () => {
  const input = snykSource(["package.json"]);
  render(<ManifestComparison manifests={[]} sources={[]} snykSources={[{
    ...input, snapshot: input.snapshot && { ...input.snapshot, metadata: { ...input.snapshot.metadata, incomplete: true } }
  }]} />);
  expect(screen.getByText(/Snyk includes projects without dependency graphs/)).toBeVisible();
});
