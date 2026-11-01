import React from "react";

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { parseScannerReportResponse } from "@/lib/scanner-report";
import { ScannerReportCard } from "./scanner-report-card";

const report = parseScannerReportResponse({
  report: {
    schema_version: 1,
    sources: [{
      detector: "npm-package-json",
      path: "package.json",
      form: "manifest",
      analysis: { presence: "present", extraction: "partial" },
      diagnostics: [{ severity: "warning", code: "dynamic-version", message: "A version could not be resolved." }]
    }],
    check_runs: [{
      check_id: "missing-lockfile",
      subject: { project_root: "." },
      status: "skipped",
      reason_code: "unsupported-format",
      detail: "This source format is not checked."
    }],
    findings: [{
      check_id: "missing-lockfile",
      severity: "high",
      summary: "Dependency versions are not locked",
      subject: { project_root: "." },
      locations: [{ path: "package.json" }],
      evidence: { manifest: "package.json" },
      remediation: "Commit a lockfile.",
      fingerprint: "finding-1"
    }]
  }
});

describe("ScannerReportCard", () => {
  it("shows findings, check coverage, and incomplete extraction", () => {
    render(<ScannerReportCard reportState={report} scanId="scan-1" />);

    expect(screen.getByText("Dependency versions are not locked")).toBeInTheDocument();
    expect(screen.getByText("Commit a lockfile.")).toBeInTheDocument();
    expect(screen.getByText("high")).toBeInTheDocument();
    expect(screen.getByText("0 sources fully extracted")).toBeInTheDocument();
    const checksSummary = screen.getByText("Check runs · 1");
    const checksDetails = checksSummary.closest("details");
    if (!checksDetails) throw new Error("Missing check disclosure");
    checksDetails.open = true;
    fireEvent(checksDetails, new Event("toggle"));
    const sourcesSummary = screen.getByText("Extraction status · 1 sources");
    const sourcesDetails = sourcesSummary.closest("details");
    if (!sourcesDetails) throw new Error("Missing source disclosure");
    sourcesDetails.open = true;
    fireEvent(sourcesDetails, new Event("toggle"));
    expect(screen.getByText("skipped")).toBeInTheDocument();
    expect(screen.getByText("unsupported-format · This source format is not checked.")).toBeInTheDocument();
    expect(screen.getByText("partial")).toBeInTheDocument();
    expect(screen.getByText(/A version could not be resolved/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View complete JSON report" })).toHaveAttribute("href", "/api/v1/scans/scan-1/report");
  });

  it("limits finding cards to ten at a time", () => {
    if (report.kind !== "available") throw new Error("Missing fixture report");
    const first = report.report.findings[0];
    const findings = Array.from({ length: 11 }, (_, index) => ({ ...first, summary: `Finding ${index}`, fingerprint: `fingerprint-${index}` }));
    render(<ScannerReportCard reportState={{ kind: "available", report: { ...report.report, findings } }} scanId="many-findings" />);

    expect(screen.getByText("Finding 0")).toBeInTheDocument();
    expect(screen.queryByText("Finding 10")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("Finding 10")).toBeInTheDocument();
    expect(screen.queryByText("Finding 0")).not.toBeInTheDocument();
  });

  it("keeps malformed native reports accessible as complete JSON", () => {
    render(<ScannerReportCard reportState={parseScannerReportResponse({ report: { schema_version: 1, sources: [] } })} scanId="malformed" />);

    expect(screen.getByText(/This report cannot be displayed here/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View complete JSON report" })).toHaveAttribute("href", "/api/v1/scans/malformed/report");
  });

  it("keeps legacy scans readable without a native report", () => {
    render(<ScannerReportCard reportState={{ kind: "absent" }} scanId="legacy" />);

    expect(screen.getByText(/This scan has no native scanner report/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "View complete JSON report" })).not.toBeInTheDocument();
  });

  it("does not describe an empty findings list as complete coverage", () => {
    render(<ScannerReportCard reportState={parseScannerReportResponse({ report: { schema_version: 1, findings: null, sources: null, check_runs: null } })} scanId="empty" />);

    expect(screen.getByText(/Review skipped checks and extraction status below for coverage/)).toBeInTheDocument();
    expect(screen.getByText("No checks were run.")).toBeInTheDocument();
    expect(screen.getByText("No dependency sources detected.")).toBeInTheDocument();
  });
});
