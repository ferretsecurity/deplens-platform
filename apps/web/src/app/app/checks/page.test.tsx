import React from "react";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api";
import type * as queries from "@/lib/queries";
import { parseScannerReportResponse } from "@/lib/scanner-report";
import ChecksPage from "./page";

const mocks = vi.hoisted(() => ({
  listRepositoryOptionsServer: vi.fn<typeof queries.listRepositoryOptionsServer>(),
  listScansServer: vi.fn<typeof queries.listScansServer>(),
  getScanServer: vi.fn<typeof queries.getScanServer>(),
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
  id: "11111111-1111-1111-1111-111111111111", repository_id: "repo-1", commit_sha: "latest-commit", scanned_at: "2020-03-02T03:04:05Z",
  manifest_count: 1, dependency_count: 2, labels: {}, annotation: ""
} satisfies Awaited<ReturnType<typeof queries.listScansServer>>[number];
const older = { ...latest, id: "22222222-2222-2222-2222-222222222222", commit_sha: "older-commit", scanned_at: "2019-01-01T01:02:03Z" };
const otherLatest = { ...latest, id: "33333333-3333-3333-3333-333333333333", repository_id: "repo-2", commit_sha: "other-latest-commit" };
const otherOlder = { ...otherLatest, id: "44444444-4444-4444-4444-444444444444", commit_sha: "other-older-commit", scanned_at: older.scanned_at };
const nativeReport = parseScannerReportResponse({ report: {
  schema_version: 1, sources: [], check_runs: [], findings: [{
    check_id: "missing-codeowners", severity: "medium", summary: "CODEOWNERS is missing",
    subject: { project_root: "." }, locations: [], remediation: "Add CODEOWNERS.", fingerprint: "codeowners"
  }]
} });

beforeEach(() => {
  vi.resetAllMocks();
  mocks.listRepositoryOptionsServer.mockResolvedValue(repositories);
  mocks.listScansServer.mockImplementation(async (repositoryId) => repositoryId === "repo-1" ? [latest, older] : [otherLatest, otherOlder]);
  mocks.getScanServer.mockImplementation(async (scanId) => {
    const scan = [latest, older, otherLatest, otherOlder].find((item) => item.id === scanId);
    if (!scan) throw new ApiError(404, "Scan not found");
    return scan;
  });
  mocks.getScanReportServer.mockResolvedValue(nativeReport);
});

describe("ChecksPage", () => {
  it.each([undefined, { repository_id: "", scan_id: "" }, { repository_id: "  ", scan_id: " \t " }])("shows each project's latest scan with default filters %j", async (params) => {
    render(await ChecksPage({ searchParams: Promise.resolve(params ?? {}) }));

    expect(mocks.listScansServer).toHaveBeenCalledTimes(2);
    for (const repository of repositories) {
      expect(mocks.listScansServer).toHaveBeenCalledWith(repository.id, "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z");
    }
    expect(mocks.getScanReportServer.mock.calls).toEqual([[latest.id], [otherLatest.id]]);
    expect(mocks.getScanServer).not.toHaveBeenCalled();
    expect(screen.getByRole("combobox", { name: "Repository" })).toHaveValue("");
    expect(screen.getByRole("combobox", { name: "Scan" })).toHaveValue("");
    for (const [name, scan] of [["Example", latest], ["Other", otherLatest]] as const) {
      const project = within(screen.getByRole("region", { name: `Checks for ${name}` }));
      expect(project.getByText("CODEOWNERS is missing")).toBeVisible();
      expect(project.getByText(scan.commit_sha)).toBeVisible();
      expect(project.getByText("Scanned at · Latest scan")).toBeVisible();
      expect(project.getByTitle("2020-03-02T03:04:05.000Z")).toHaveAttribute("datetime", scan.scanned_at);
      expect(project.getByRole("link", { name: "Scan details" })).toHaveAttribute("href", `/app/scans/${scan.id}`);
      expect(project.getByRole("link", { name: "Permalink to these checks" })).toHaveAttribute("href", `/app/checks?repository_id=${scan.repository_id}&scan_id=${scan.id}`);
    }
    expect(screen.queryByText(older.commit_sha)).not.toBeInTheDocument();
    expect(screen.queryByText(otherOlder.commit_sha)).not.toBeInTheDocument();
    const scanForm = within(screen.getByRole("form", { name: "Choose scan" }));
    expect(scanForm.getAllByRole("group").map((group) => group.getAttribute("label"))).toEqual(["Example", "Other"]);
    expect(scanForm.getByRole("option", { name: new RegExp(older.id) })).toHaveValue(older.id);
    expect(scanForm.getByRole("option", { name: new RegExp(otherLatest.id) })).toHaveValue(otherLatest.id);
  });

  it("limits a repository-only filter to that project's latest scan and offers its history", async () => {
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1" }) }));

    expect(mocks.listScansServer).toHaveBeenCalledExactlyOnceWith("repo-1", "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z");
    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith(latest.id);
    expect(screen.getByRole("combobox", { name: "Scan" })).toHaveValue("");
    expect(screen.getByRole("option", { name: new RegExp(older.id) })).toHaveValue(older.id);
    expect(screen.queryByRole("region", { name: "Checks for Other" })).not.toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Project pages" })).not.toBeInTheDocument();
  });

  it("loads a historical scan and lets repository switching and scan clearing reset independently", async () => {
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1", scan_id: older.id, page: "9" }) }));

    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith(older.id);
    expect(screen.getByText(older.commit_sha)).toBeVisible();
    expect(screen.queryByText("Scanned at · Latest scan")).not.toBeInTheDocument();
    const repositoryForm = screen.getByRole("form", { name: "Choose repository" });
    if (!(repositoryForm instanceof HTMLFormElement)) throw new Error("Missing repository form");
    expect(repositoryForm).toHaveAttribute("method", "get");
    expect(repositoryForm).toHaveAttribute("action", "/app/checks");
    expect(Array.from(new FormData(repositoryForm).entries())).toEqual([["repository_id", "repo-1"]]);
    const scanForm = screen.getByRole("form", { name: "Choose scan" });
    if (!(scanForm instanceof HTMLFormElement)) throw new Error("Missing scan form");
    const scanSelect = within(scanForm).getByRole("combobox", { name: "Scan" });
    expect(scanSelect).toHaveValue(older.id);
    expect(Array.from(new FormData(scanForm).entries())).toEqual([["repository_id", "repo-1"], ["scan_id", older.id]]);
    if (!(scanSelect instanceof HTMLSelectElement)) throw new Error("Missing scan select");
    scanSelect.value = "";
    expect(Array.from(new FormData(scanForm).entries())).toEqual([["repository_id", "repo-1"], ["scan_id", ""]]);
    expect(screen.getByRole("link", { name: "Clear filters" })).toHaveAttribute("href", "/app/checks");
  });

  it("resolves a scan-only historical URL to its owner while leaving the repository filter blank", async () => {
    render(await ChecksPage({ searchParams: Promise.resolve({ scan_id: otherOlder.id }) }));

    expect(mocks.getScanServer).toHaveBeenCalledExactlyOnceWith(otherOlder.id);
    expect(mocks.listScansServer).toHaveBeenCalledExactlyOnceWith("repo-2", "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z");
    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith(otherOlder.id);
    expect(screen.getByRole("combobox", { name: "Repository" })).toHaveValue("");
    expect(screen.getByRole("combobox", { name: "Scan" })).toHaveValue(otherOlder.id);
    expect(screen.getByText(otherOlder.commit_sha)).toBeVisible();
    const scanForm = screen.getByRole("form", { name: "Choose scan" });
    if (!(scanForm instanceof HTMLFormElement)) throw new Error("Missing scan form");
    expect(Array.from(new FormData(scanForm).entries())).toEqual([["scan_id", otherOlder.id]]);
  });

  it("resets both filter controls when navigation clears their URL values", async () => {
    const view = render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-2", scan_id: otherOlder.id }) }));
    expect(screen.getByRole("combobox", { name: "Repository" })).toHaveValue("repo-2");
    expect(screen.getByRole("combobox", { name: "Scan" })).toHaveValue(otherOlder.id);

    view.rerender(await ChecksPage({}));

    expect(screen.getByRole("combobox", { name: "Repository" })).toHaveValue("");
    expect(screen.getByRole("combobox", { name: "Scan" })).toHaveValue("");
    expect(screen.getByRole("region", { name: "Checks for Example" })).toBeVisible();
    expect(screen.getByRole("region", { name: "Checks for Other" })).toBeVisible();
  });

  it("does not fall back from an unavailable repository", async () => {
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "missing", scan_id: latest.id }) }));

    expect(screen.getByText("This repository is unavailable. Select another repository above.")).toBeVisible();
    expect(screen.getByRole("option", { name: "Unavailable repository", selected: true })).toHaveValue("missing");
    expect(mocks.listScansServer).not.toHaveBeenCalled();
    expect(mocks.getScanServer).not.toHaveBeenCalled();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
  });

  it.each(["missing", "../scans", "?scan=bad"])("rejects malformed scan ID %j before API access", async (scanId) => {
    render(await ChecksPage({ searchParams: Promise.resolve({ scan_id: scanId }) }));

    expect(screen.getByText("This scan is unavailable. Select another scan or clear the scan filter above.")).toBeVisible();
    expect(screen.getByRole("option", { name: "Unavailable scan", selected: true })).toHaveValue(scanId);
    expect(mocks.getScanServer).not.toHaveBeenCalled();
    expect(mocks.listScansServer).not.toHaveBeenCalled();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
  });

  it("keeps a missing scan explicit", async () => {
    const scanId = "99999999-9999-9999-9999-999999999999";
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1", scan_id: scanId }) }));

    expect(screen.getByRole("option", { name: "Unavailable scan", selected: true })).toHaveValue(scanId);
    expect(mocks.getScanServer).toHaveBeenCalledExactlyOnceWith(scanId);
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
    expect(screen.queryByText(latest.commit_sha)).not.toBeInTheDocument();
  });

  it("rejects a scan belonging to a different repository before fetching its report", async () => {
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1", scan_id: otherLatest.id }) }));

    expect(screen.getByText("This scan does not belong to the selected repository. Select another scan or repository above.")).toBeVisible();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
    expect(mocks.listScansServer).not.toHaveBeenCalled();
  });

  it("does not display a scan whose owning repository is unavailable in this tenant", async () => {
    mocks.getScanServer.mockResolvedValue({ ...older, repository_id: "unavailable" });
    render(await ChecksPage({ searchParams: Promise.resolve({ scan_id: older.id }) }));

    expect(screen.getByText("This scan is unavailable. Select another scan or clear the scan filter above.")).toBeVisible();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
    expect(mocks.listScansServer).not.toHaveBeenCalled();
  });

  it("keeps no scan history distinct from an absent report", async () => {
    mocks.listScansServer.mockResolvedValue([]);
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-2" }) }));

    expect(screen.getByText("No CLI scans have been uploaded for Other.")).toBeVisible();
    expect(screen.getByRole("combobox", { name: "Scan" })).toHaveValue("");
    expect(screen.queryByText(/This scan has no native scanner report/)).not.toBeInTheDocument();
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
  });

  it("includes projects with no scans alongside projects with reports", async () => {
    mocks.listScansServer.mockImplementation(async (id) => id === "repo-1" ? [latest] : []);
    render(await ChecksPage({}));

    expect(screen.getByRole("region", { name: "Checks for Example" })).toBeVisible();
    expect(screen.getByText("No CLI scans have been uploaded for Other.")).toBeVisible();
    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith(latest.id);
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
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1" }) }));

    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith(latest.id);
    expect(screen.getByText(/This scan has no native scanner report/)).toBeVisible();
    expect(screen.queryByRole("link", { name: "View complete JSON report" })).not.toBeInTheDocument();
    expect(screen.getByText(latest.commit_sha)).toBeVisible();
    expect(screen.queryByText(older.commit_sha)).not.toBeInTheDocument();
  });

  it("preserves the complete JSON link when the selected report cannot be displayed", async () => {
    mocks.getScanReportServer.mockResolvedValue({ kind: "unavailable" });
    render(await ChecksPage({ searchParams: Promise.resolve({ repository_id: "repo-1", scan_id: older.id }) }));

    expect(screen.getByText(/This report cannot be displayed here/)).toBeVisible();
    expect(screen.getByRole("link", { name: "View complete JSON report" })).toHaveAttribute("href", `/api/v1/scans/${older.id}/report`);
  });

  it("preserves report HTTP errors", async () => {
    const error = new ApiError(500, "Report request failed");
    mocks.getScanReportServer.mockRejectedValue(error);
    await expect(ChecksPage({})).rejects.toBe(error);
  });

  it("preserves scan lookup HTTP errors other than unavailable selections", async () => {
    const error = new ApiError(500, "Scan request failed");
    mocks.getScanServer.mockRejectedValue(error);
    await expect(ChecksPage({ searchParams: Promise.resolve({ scan_id: older.id }) })).rejects.toBe(error);
    expect(mocks.getScanReportServer).not.toHaveBeenCalled();
  });

  it.each([undefined, "garbage", "0", "-1", "1.5", "Infinity"])("bounds initial work to ten projects for page %j", async (page) => {
    const manyRepositories = Array.from({ length: 101 }, (_, index) => ({ ...repositories[0], id: `repo-${index + 1}`, name: `Project ${index + 1}` }));
    mocks.listRepositoryOptionsServer.mockResolvedValue(manyRepositories);
    mocks.listScansServer.mockImplementation(async (id) => [{ ...latest, id: `scan-${id}`, repository_id: id }]);
    render(await ChecksPage({ searchParams: Promise.resolve({ page }) }));

    expect(mocks.listScansServer.mock.calls.map(([id]) => id)).toEqual(manyRepositories.slice(0, 10).map(({ id }) => id));
    expect(mocks.getScanReportServer).toHaveBeenCalledTimes(10);
    expect(screen.getByText("Showing projects 1–10 of 101. Page 1 of 11.")).toBeVisible();
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute("href", "/app/checks?page=2");
    expect(screen.queryByRole("link", { name: "Previous" })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Project 101" })).toHaveValue("repo-101");
  });

  it.each(["11", "999999"])("makes project 101 reachable and clamps page %j to the last page", async (page) => {
    const manyRepositories = Array.from({ length: 101 }, (_, index) => ({ ...repositories[0], id: `repo-${index + 1}`, name: `Project ${index + 1}` }));
    mocks.listRepositoryOptionsServer.mockResolvedValue(manyRepositories);
    mocks.listScansServer.mockResolvedValue([{ ...latest, repository_id: "repo-101" }]);
    render(await ChecksPage({ searchParams: Promise.resolve({ page }) }));

    expect(mocks.listScansServer).toHaveBeenCalledExactlyOnceWith("repo-101", "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z");
    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith(latest.id);
    expect(screen.getByRole("region", { name: "Checks for Project 101" })).toBeVisible();
    expect(screen.getByText("Showing projects 101–101 of 101. Page 11 of 11.")).toBeVisible();
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute("href", "/app/checks?page=10");
    expect(screen.queryByRole("link", { name: "Next" })).not.toBeInTheDocument();
  });

  it("displays a scan-only historical URL from a project outside the first page", async () => {
    mocks.listRepositoryOptionsServer.mockResolvedValue(Array.from({ length: 101 }, (_, index) => ({ ...repositories[0], id: `repo-${index + 1}`, name: `Project ${index + 1}` })));
    mocks.getScanServer.mockResolvedValue({ ...older, repository_id: "repo-101" });
    mocks.listScansServer.mockResolvedValue([{ ...latest, repository_id: "repo-101" }]);
    render(await ChecksPage({ searchParams: Promise.resolve({ scan_id: older.id }) }));

    expect(mocks.getScanReportServer).toHaveBeenCalledExactlyOnceWith(older.id);
    expect(screen.getByRole("region", { name: "Checks for Project 101" })).toBeVisible();
    expect(screen.getByRole("combobox", { name: "Scan" })).toHaveValue(older.id);
    expect(screen.getByRole("option", { name: `${older.commit_sha} · ${older.id}` })).toHaveValue(older.id);
    expect(screen.getByRole("combobox", { name: "Repository" })).toHaveValue("");
    expect(screen.queryByRole("navigation", { name: "Project pages" })).not.toBeInTheDocument();
  });
});
