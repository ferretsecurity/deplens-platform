import React from "react";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type * as queries from "@/lib/queries";
import { parseScannerReportResponse } from "@/lib/scanner-report";
import ChecksPage from "./page";

const mocks = vi.hoisted(() => ({
  listRepositoryOptionsServer: vi.fn<typeof queries.listRepositoryOptionsServer>(),
  listScansServer: vi.fn<typeof queries.listScansServer>(),
  getScanReportServer: vi.fn<typeof queries.getScanReportServer>()
}));

vi.mock("@/lib/queries", () => mocks);
vi.mock("@/components/empty-inventory", () => ({
  EmptyInventory: () => <p>Scanner setup instructions</p>
}));

const repositories = [
  { id: "repo-1", name: "Example", url: "https://example.com/repo", default_branch: "main" },
  { id: "repo-2", name: "Other", url: "https://example.com/other", default_branch: "main" }
] satisfies Awaited<ReturnType<typeof queries.listRepositoryOptionsServer>>;
const latest = {
  id: "latest-scan", repository_id: "repo-1", commit_sha: "latest-commit", scanned_at: "2020-03-02T03:04:05Z",
  manifest_count: 1, dependency_count: 2, labels: {}, annotation: ""
} satisfies Awaited<ReturnType<typeof queries.listScansServer>>[number];
const older = { ...latest, id: "older-scan", commit_sha: "older-commit", scanned_at: "2019-01-01T01:02:03Z" };
const nativeReport = parseScannerReportResponse({ report: {
  schema_version: 1, sources: [], check_runs: [], findings: [{
    check_id: "missing-codeowners", severity: "medium", summary: "CODEOWNERS is missing",
    subject: { project_root: "." }, locations: [], remediation: "Add CODEOWNERS.", fingerprint: "codeowners"
  }]
} });

beforeEach(() => {
  vi.clearAllMocks();
  mocks.listRepositoryOptionsServer.mockResolvedValue(repositories);
  mocks.listScansServer.mockResolvedValue([latest, older]);
  mocks.getScanReportServer.mockResolvedValue(nativeReport);
});

describe("ChecksPage", () => {
  it("shows the API's latest scan across all dates and fetches only its report", async () => {
    render(await ChecksPage({}));

    expect(mocks.listScansServer).toHaveBeenCalledExactlyOnceWith("repo-1", "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z");
    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith("latest-scan");
    expect(screen.getByText("CODEOWNERS is missing")).toBeVisible();
    expect(screen.getByText("latest-commit")).toBeVisible();
    expect(screen.getByText("Scanned at · Latest scan")).toBeVisible();
    expect(screen.getByTitle("2020-03-02T03:04:05.000Z")).toHaveAttribute("datetime", latest.scanned_at);
    expect(screen.getByRole("link", { name: "Scan details" })).toHaveAttribute("href", "/app/scans/latest-scan");
    expect(screen.getByRole("link", { name: "Permalink to these checks" })).toHaveAttribute("href", "/app/checks?repository_id=repo-1&scan_id=latest-scan");
  });

  it("loads an explicitly selected historical scan and keeps repository switching independent", async () => {
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1", scan_id: "older-scan" }) }));

    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith("older-scan");
    expect(screen.getByText("older-commit")).toBeVisible();
    expect(screen.queryByText("Scanned at · Latest scan")).not.toBeInTheDocument();
    const repositoryForm = screen.getByRole("form", { name: "Choose repository" });
    if (!(repositoryForm instanceof HTMLFormElement)) throw new Error("Missing repository form");
    expect(repositoryForm).toHaveAttribute("method", "get");
    expect(repositoryForm).toHaveAttribute("action", "/app/checks");
    expect(Array.from(new FormData(repositoryForm).entries())).toEqual([["repository_id", "repo-1"]]);
    const scanForm = screen.getByRole("form", { name: "Choose scan" });
    if (!(scanForm instanceof HTMLFormElement)) throw new Error("Missing scan form");
    expect(Array.from(new FormData(scanForm).entries())).toEqual([["repository_id", "repo-1"], ["scan_id", "older-scan"]]);
    expect(within(scanForm).getByRole("combobox", { name: "Scan" })).toHaveValue("older-scan");
  });

  it.each(["missing", ""])("does not fall back from explicit unavailable repository %j", async (repositoryId) => {
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: repositoryId, scan_id: "latest-scan" }) }));

    expect(screen.getByText("This repository is unavailable. Select another repository above.")).toBeVisible();
    expect(screen.getByRole("option", { name: "Unavailable repository", selected: true })).toHaveValue(repositoryId);
    expect(mocks.listScansServer).not.toHaveBeenCalled();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
    expect(screen.queryByText("Scanner report")).not.toBeInTheDocument();
  });

  it.each(["missing", "other-repository-scan", ""])("does not fall back from explicit unavailable scan %j", async (scanId) => {
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1", scan_id: scanId }) }));

    expect(screen.getByText("This scan is unavailable for the selected repository. Select another scan or repository above.")).toBeVisible();
    expect(screen.getByRole("option", { name: "Unavailable scan", selected: true })).toHaveValue(scanId);
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
    expect(screen.queryByText("latest-commit")).not.toBeInTheDocument();
  });

  it("keeps no scan history distinct from an absent report", async () => {
    mocks.listScansServer.mockResolvedValue([]);
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-2" }) }));

    expect(screen.getByText("No CLI scans have been uploaded for Other.")).toBeVisible();
    expect(screen.queryByText(/This scan has no native scanner report/)).not.toBeInTheDocument();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
  });

  it("shows initial upload instructions when there are no repositories", async () => {
    mocks.listRepositoryOptionsServer.mockResolvedValue([]);
    render(await ChecksPage({}));

    expect(screen.getByText("Scanner setup instructions")).toBeVisible();
    expect(mocks.listScansServer).not.toHaveBeenCalled();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
  });

  it("keeps the latest legacy scan selected when its report is absent", async () => {
    mocks.getScanReportServer.mockResolvedValue({ kind: "absent" });
    render(await ChecksPage({}));

    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith("latest-scan");
    expect(screen.getByText(/This scan has no native scanner report/)).toBeVisible();
    expect(screen.queryByRole("link", { name: "View complete JSON report" })).not.toBeInTheDocument();
    expect(screen.getByText("latest-commit")).toBeVisible();
  });

  it("preserves the complete JSON link when the selected report cannot be displayed", async () => {
    mocks.getScanReportServer.mockResolvedValue({ kind: "unavailable" });
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1", scan_id: "older-scan" }) }));

    expect(screen.getByText(/This report cannot be displayed here/)).toBeVisible();
    expect(screen.getByRole("link", { name: "View complete JSON report" })).toHaveAttribute("href", "/api/v1/scans/older-scan/report");
  });

  it("preserves report HTTP errors", async () => {
    const error = new Error("Report request failed");
    mocks.getScanReportServer.mockRejectedValue(error);
    await expect(ChecksPage({})).rejects.toBe(error);
  });
});
