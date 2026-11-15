import React from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ScannerReport } from "@/lib/scanner-report";
import type { ProjectChecks } from "./checks-data";
import { ChecksFindings } from "./checks-findings";

type Finding = ScannerReport["findings"][number];
function finding(overrides: Partial<Finding> = {}): Finding {
  return {
    check_id: "missing-lockfile", severity: "high", summary: "Lockfile missing", subject: { project_root: "apps/api" },
    locations: [{ path: "package.json" }, { path: "nested/secret-path.json" }], evidence: { manifest: "package.json" },
    remediation: "Commit a lockfile.", fingerprint: "lockfile", ...overrides
  };
}
function project(findings: Finding[], id = "repo-1"): ProjectChecks {
  return {
    repository: { id, name: `Repository ${id}`, url: "https://example.com/repository", default_branch: "main" },
    scans: [],
    selected: {
      scan: { id: `scan-${id}`, repository_id: id, commit_sha: "abc123", scanned_at: "2020-01-01T00:00:00Z", manifest_count: 1, dependency_count: 1, labels: {}, annotation: "" },
      report: { kind: "available", report: { schema_version: 1, sources: [], findings,
        check_runs: [{ check_id: "coverage-only", status: "completed", subject: { project_root: "." } }] } }
    }
  };
}
function rows() {
  return [...screen.getByRole("region", { name: "Findings" }).querySelectorAll("details")].filter((details) => details.classList.contains("group"));
}

describe("ChecksFindings", () => {
  it("intersects search, exact severity, and exact check IDs, then resets them", () => {
    render(<ChecksFindings projects={[project([
      finding({ summary: "API needs lock", fingerprint: "a" }),
      finding({ summary: "API low finding", severity: "low", fingerprint: "b" }),
      finding({ check_id: "missing-codeowners", summary: "API needs owners", fingerprint: "c" }),
      finding({ summary: "Other project", subject: { project_root: "web" }, locations: [], fingerprint: "d" })
    ])]} />);
    fireEvent.change(screen.getByLabelText("Search findings"), { target: { value: "API" } });
    fireEvent.change(screen.getByLabelText("Severity"), { target: { value: "high" } });
    fireEvent.change(screen.getByLabelText("Check type"), { target: { value: "missing-lockfile" } });
    expect(rows()).toHaveLength(1);
    expect(within(rows()[0]).getByText("API needs lock")).toBeVisible();
    expect(screen.getByRole("option", { name: "missing-codeowners" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Reset filters" }));
    expect(rows()).toHaveLength(4);
    expect(screen.getByLabelText("Search findings")).toHaveValue("");
    expect(screen.getByLabelText("Severity")).toHaveValue("");
    expect(screen.getByLabelText("Check type")).toHaveValue("");
  });

  it("searches repository, check ID, project root and every location case insensitively", () => {
    render(<ChecksFindings projects={[project([finding()])]} />);
    for (const query of ["REPOSITORY REPO-1", "MISSING-LOCKFILE", "APPS/API", "SECRET-PATH"]) {
      fireEvent.change(screen.getByLabelText("Search findings"), { target: { value: query } });
      expect(rows()).toHaveLength(1);
    }
  });

  it("sorts by severity and deterministic context before paginating, and filters all pages", () => {
    const findings = Array.from({ length: 24 }, (_, index) => finding({ summary: `Low finding ${index}`, severity: "low", fingerprint: `low-${String(index).padStart(2, "0")}` }));
    findings.push(finding({ summary: "Critical finding", severity: "critical", fingerprint: "critical" }));
    findings.push(finding({ summary: "Target finding", check_id: "target-check", fingerprint: "target" }));
    render(<ChecksFindings projects={[project(findings)]} />);
    expect(rows()).toHaveLength(10);
    expect(within(rows()[0]).getByText("Critical finding")).toBeVisible();
    expect(within(rows()[1]).getByText("Target finding")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Findings 11 to 20 of 26")).toBeVisible();
    fireEvent.change(screen.getByLabelText("Check type"), { target: { value: "target-check" } });
    expect(rows()).toHaveLength(1);
    expect(screen.getByText("Target finding")).toBeVisible();
    expect(screen.getByText("Findings 1 to 1 of 1")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Reset filters" }));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    fireEvent.change(screen.getByLabelText("Severity"), { target: { value: "critical" } });
    expect(screen.getByText("Findings 1 to 1 of 1")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Reset filters" }));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    fireEvent.change(screen.getByLabelText("Search findings"), { target: { value: "Target" } });
    expect(screen.getByText("Findings 1 to 1 of 1")).toBeVisible();
  });

  it("offers checks with no findings and preserves their empty filtered result", () => {
    render(<ChecksFindings projects={[project([finding()])]} />);
    fireEvent.change(screen.getByLabelText("Check type"), { target: { value: "coverage-only" } });
    expect(screen.getByText("No findings match these filters.")).toBeVisible();
    expect(screen.getByLabelText("Check type")).toHaveValue("coverage-only");
    expect(rows()).toHaveLength(0);
  });

  it("keeps same-fingerprint findings from different scans independently expandable", () => {
    render(<ChecksFindings projects={[project([finding({ summary: "Second repository finding" })], "repo-2"), project([finding({ summary: "First repository finding" })])]} />);
    expect(within(rows()[0]).getByText("First repository finding")).toBeVisible();
    fireEvent.click(within(rows()[0]).getByText("First repository finding"));
    expect(rows()[0]).toHaveAttribute("open");
    expect(rows()[1]).not.toHaveAttribute("open");
    const first = within(rows()[0]);
    expect(first.getByText("Commit a lockfile.")).toBeVisible();
    expect(first.getByText("nested/secret-path.json")).toBeVisible();
    fireEvent.click(first.getByText("Evidence and scan"));
    expect(first.getByRole("link", { name: "Open scan" })).toHaveAttribute("href", "/app/scans/scan-repo-1");
    expect(first.getByText("manifest")).toBeVisible();
    fireEvent.change(screen.getByLabelText("Search findings"), { target: { value: "Second repository" } });
    expect(rows()).toHaveLength(1);
    expect(rows()[0]).not.toHaveAttribute("open");
  });

  it("clamps pagination when a report refresh shrinks findings under the same scan", () => {
    const many = Array.from({ length: 21 }, (_, index) => finding({ summary: `Finding ${index}`, fingerprint: String(index) }));
    const view = render(<ChecksFindings projects={[project(many)]} />);
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    view.rerender(<ChecksFindings projects={[project([finding()])]} />);
    expect(screen.getByText("Findings 1 to 1 of 1")).toBeVisible();
    expect(rows()).toHaveLength(1);
  });

  it("distinguishes a readable report with zero findings from no readable reports", () => {
    const view = render(<ChecksFindings projects={[project([])]} />);
    expect(screen.getByText("No findings reported in this repository page. Review scan coverage below.")).toBeVisible();
    view.rerender(<ChecksFindings projects={[{ ...project([]), selected: null }]} />);
    expect(screen.getByText("No findings available. Review scan coverage below.")).toBeVisible();
    expect(screen.queryByText(/No findings reported/)).not.toBeInTheDocument();
  });
});
