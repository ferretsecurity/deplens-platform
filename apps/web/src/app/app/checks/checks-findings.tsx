"use client";

import React, { useState } from "react";
import Link from "next/link";

import { ListPagination } from "@/components/list-pagination";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { findingSeveritySchema, type ScannerReport } from "@/lib/scanner-report";
import type { ProjectChecks } from "./checks-data";

type Finding = ScannerReport["findings"][number];
type FindingEntry = {
  finding: Finding;
  repository: ProjectChecks["repository"];
  scan: NonNullable<ProjectChecks["selected"]>["scan"];
};
type FindingFilters = { query: string; severity: "" | Finding["severity"]; checkId: string };
const emptyFilters: FindingFilters = { query: "", severity: "", checkId: "" };
const severityRank: Record<Finding["severity"], number> = { critical: 0, high: 1, medium: 2, low: 3, info: 4 };
const severities = findingSeveritySchema.options;
const pageSize = 10;

function collectFindings(projects: ProjectChecks[]): FindingEntry[] {
  return projects.flatMap(({ repository, selected }) => selected?.report.kind === "available"
    ? selected.report.report.findings.map((finding) => ({ finding, repository, scan: selected.scan })) : []
  ).sort((a, b) => severityRank[a.finding.severity] - severityRank[b.finding.severity]
    || a.repository.id.localeCompare(b.repository.id)
    || a.scan.id.localeCompare(b.scan.id)
    || a.finding.fingerprint.localeCompare(b.finding.fingerprint));
}

function filterFindings(entries: FindingEntry[], filters: FindingFilters) {
  const query = filters.query.trim().toLowerCase();
  return entries.filter(({ finding, repository }) => (!filters.severity || finding.severity === filters.severity)
    && (!filters.checkId || finding.check_id === filters.checkId)
    && [finding.summary, repository.name, finding.check_id, finding.subject.project_root, ...finding.locations.map((location) => location.path)]
      .join(" ").toLowerCase().includes(query));
}

export function ChecksFindings({ projects }: { projects: ProjectChecks[] }) {
  const [filters, setFilters] = useState(emptyFilters);
  const [requestedPage, setPage] = useState(0);
  const entries = collectFindings(projects);
  const checkIds = [...new Set(projects.flatMap(({ selected }) => selected?.report.kind === "available"
    ? [...selected.report.report.check_runs.map((check) => check.check_id), ...selected.report.report.findings.map((finding) => finding.check_id)] : []))].sort();
  const matches = filterFindings(entries, filters);
  const page = Math.min(requestedPage, Math.max(0, Math.ceil(matches.length / pageSize) - 1));
  const readableReports = projects.filter(({ selected }) => selected?.report.kind === "available").length;
  function updateFilters(next: FindingFilters) {
    setFilters(next);
    setPage(0);
  }

  return (
    <section aria-label="Findings" className="min-w-0 space-y-4">
      <div className="flex flex-wrap items-end gap-3">
        <div className="grid w-full min-w-0 gap-2 sm:max-w-80 sm:flex-1">
          <Label htmlFor="findings-search">Search findings</Label>
          <input id="findings-search" type="search" placeholder="Search findings…" value={filters.query} onChange={(event) => updateFilters({ ...filters, query: event.target.value })} className="h-9 min-w-0 rounded-md border border-slate-200 bg-white px-3 text-sm" />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="findings-severity">Severity</Label>
          <select id="findings-severity" value={filters.severity} onChange={(event) => {
            const value = event.target.value;
            const severity = severities.find((item) => item === value) ?? "";
            updateFilters({ ...filters, severity });
          }} className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm">
            <option value="">All severities</option>
            {[...severities].sort((a, b) => severityRank[a] - severityRank[b]).map((severity) => <option key={severity} value={severity}>{severity}</option>)}
          </select>
        </div>
        <div className="grid min-w-0 gap-2">
          <Label htmlFor="findings-check">Check type</Label>
          <select id="findings-check" value={filters.checkId} onChange={(event) => updateFilters({ ...filters, checkId: event.target.value })} className="h-9 w-full min-w-0 max-w-full rounded-md border border-slate-200 bg-white px-3 text-sm sm:max-w-72">
            <option value="">All check types</option>
            {checkIds.map((checkId) => <option key={checkId} value={checkId}>{checkId}</option>)}
          </select>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={() => updateFilters(emptyFilters)}>Reset filters</Button>
        <span className="ml-auto pb-2 text-xs text-slate-400">Severity first</span>
      </div>
      <p className="text-xs text-slate-500">Search and filters apply to the repositories on this page. Changing repository or scan selection resets these filters.</p>
      <p aria-live="polite" className="text-sm text-slate-500">{matches.length} matching of {entries.length} findings in this repository page</p>
      {matches.length === 0 ? (
        <p className="border-y border-slate-200 py-12 text-center text-sm text-slate-500">
          {entries.length ? "No findings match these filters." : readableReports ? "No findings reported in this repository page. Review scan coverage below." : "No findings available. Review scan coverage below."}
        </p>
      ) : (
        <div className="border-y border-slate-200 bg-white">
          {matches.slice(page * pageSize, (page + 1) * pageSize).map(({ finding, repository, scan }) => (
            <details key={`${scan.id}:${finding.fingerprint}`} className="group border-b border-slate-200 last:border-0">
              <summary className="grid cursor-pointer list-none items-start gap-3 px-4 py-4 sm:grid-cols-[78px_minmax(0,1fr)_140px_16px]">
                <span className={`inline-flex items-center gap-2 text-xs ${severityRank[finding.severity] < 2 ? "text-red-700" : finding.severity === "medium" ? "text-amber-700" : "text-slate-500"}`}><span className="h-1.5 w-1.5 shrink-0 rounded-full bg-current" />{finding.severity}</span>
                <span className="min-w-0 break-words text-sm font-medium">{finding.summary}<span className="mt-1 block truncate font-mono text-xs font-normal text-slate-400">{finding.locations[0]?.path ?? finding.subject.project_root}</span></span>
                <span className="truncate text-xs text-slate-500">{repository.name}</span>
                <span aria-hidden="true" className="hidden text-slate-400 group-open:rotate-90 sm:block">›</span>
              </summary>
              <div className="space-y-5 border-t border-slate-100 bg-slate-50/60 p-5 text-sm [overflow-wrap:anywhere] sm:pl-[106px]">
                <div><h3 className="mb-2 font-medium text-slate-900">Suggested fix</h3><p className="leading-relaxed text-slate-600">{finding.remediation}</p></div>
                <dl className="space-y-2 text-slate-500">
                  <div><dt className="font-medium">Repository</dt><dd>{repository.name}</dd></div>
                  <div><dt className="font-medium">Project root</dt><dd className="font-mono text-xs">{finding.subject.project_root}</dd></div>
                  <div><dt className="font-medium">Locations</dt><dd className="font-mono text-xs">{finding.locations.length ? <ul>{finding.locations.map((location, index) => <li key={`${location.path}:${index}`}>{location.path}</li>)}</ul> : finding.subject.project_root}</dd></div>
                  <div><dt className="font-medium">Check</dt><dd>{finding.check_id}</dd></div>
                </dl>
                <details className="text-slate-500">
                  <summary className="cursor-pointer">Evidence and scan</summary>
                  <dl className="mt-3 space-y-2">{Object.entries(finding.evidence).map(([key, value]) => <div key={key}><dt className="font-medium">{key}</dt><dd>{value}</dd></div>)}</dl>
                  <p className="mt-3">{scan.commit_sha} · <time dateTime={scan.scanned_at}>{new Date(scan.scanned_at).toLocaleString("en-GB", { timeZone: "UTC" })} UTC</time></p>
                  <div className="mt-2 flex flex-wrap gap-4">
                    <Link className="underline" href={`/app/scans/${scan.id}`}>Open scan</Link>
                    <Link className="underline" href={`/app/checks?${new URLSearchParams({ repository_id: repository.id, scan_id: scan.id })}`}>Permalink to these checks</Link>
                  </div>
                </details>
              </div>
            </details>
          ))}
        </div>
      )}
      {matches.length > 0 ? <p className="text-xs text-slate-500">Findings {page * pageSize + 1} to {Math.min((page + 1) * pageSize, matches.length)} of {matches.length}</p> : null}
      <ListPagination page={page} pageSize={pageSize} total={matches.length} onPageChange={setPage} />
    </section>
  );
}
