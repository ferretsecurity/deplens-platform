"use client";

import React, { useState } from "react";

import { ListPagination } from "@/components/list-pagination";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { ScannerReportState } from "@/lib/scanner-report";

const severityClasses = {
  critical: "bg-red-100 text-red-900",
  high: "bg-orange-100 text-orange-900",
  medium: "bg-amber-100 text-amber-900",
  low: "bg-blue-100 text-blue-900",
  info: "bg-slate-100 text-slate-700"
};

type SectionView = { kind: "collapsed" } | { kind: "expanded"; page: number };

export function ScannerReportCard({ reportState, scanId }: { reportState: ScannerReportState; scanId: string }) {
  const [findingsPage, setFindingsPage] = useState(0);
  const [checksView, setChecksView] = useState<SectionView>({ kind: "collapsed" });
  const [sourcesView, setSourcesView] = useState<SectionView>({ kind: "collapsed" });
  const report = reportState.kind === "available" ? reportState.report : null;
  return (
    <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <CardTitle>Scanner report</CardTitle>
          {reportState.kind !== "absent" && (
            <a
              href={`/api/v1/scans/${encodeURIComponent(scanId)}/report`}
              target="_blank"
              rel="noreferrer"
              className="text-sm font-medium text-blue-700 underline-offset-4 hover:underline"
            >
              View complete JSON report
            </a>
          )}
        </div>
        <CardDescription>Policy checks, findings, and dependency extraction results.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {!report ? (
          <p className="text-sm text-slate-600">
            {reportState.kind === "absent"
              ? "This scan has no native scanner report. Its uploaded inventory is shown below."
              : "This report cannot be displayed here. Open the complete JSON report to inspect it. Its uploaded inventory is shown below."}
          </p>
        ) : (
          <>
            <div className="flex flex-wrap gap-3 text-sm text-slate-700">
              <span className="rounded-lg bg-slate-100 px-3 py-2">{report.sources.length} sources</span>
              <span className="rounded-lg bg-slate-100 px-3 py-2">{report.check_runs.length} check runs</span>
              <span className="rounded-lg bg-slate-100 px-3 py-2">{report.findings.length} findings</span>
              <span className="rounded-lg bg-slate-100 px-3 py-2">
                {report.sources.filter((source) => source.analysis.extraction === "complete").length} sources fully extracted
              </span>
            </div>

            <div className="space-y-3">
              <h3 className="font-semibold text-slate-950">Findings</h3>
              <ListPagination page={findingsPage} pageSize={10} total={report.findings.length} onPageChange={setFindingsPage} />
              {report.findings.length === 0 ? (
                <p className="text-sm text-slate-600">No findings reported. Review skipped checks and extraction status below for coverage.</p>
              ) : report.findings.slice(findingsPage * 10, (findingsPage + 1) * 10).map((finding) => (
                <div key={finding.fingerprint} className="space-y-2 rounded-xl border border-slate-200 p-4 text-sm">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`rounded-full px-2.5 py-1 text-xs font-medium uppercase ${severityClasses[finding.severity]}`}>
                      {finding.severity}
                    </span>
                    <span className="font-semibold text-slate-950">{finding.summary}</span>
                  </div>
                  <p className="break-all text-slate-500">{finding.check_id} · {finding.subject.project_root}</p>
                  {finding.locations.length > 0 && (
                    <p className="break-all font-mono text-xs text-slate-600">{finding.locations.map((location) => location.path).join(", ")}</p>
                  )}
                  <p className="text-slate-700">{finding.remediation}</p>
                  {Object.entries(finding.evidence).length > 0 && (
                    <details className="text-slate-600">
                      <summary className="cursor-pointer">Evidence</summary>
                      <dl className="mt-2 space-y-1">
                        {Object.entries(finding.evidence).map(([key, value]) => (
                          <div key={key} className="break-words"><dt className="inline font-medium">{key}: </dt><dd className="inline">{value}</dd></div>
                        ))}
                      </dl>
                    </details>
                  )}
                </div>
              ))}
            </div>

            <details className="rounded-xl border border-slate-200" onToggle={(event) => setChecksView(event.currentTarget.open ? { kind: "expanded", page: 0 } : { kind: "collapsed" })}>
              <summary className="cursor-pointer px-4 py-3 font-semibold text-slate-950">Check runs · {report.check_runs.length}</summary>
              {checksView.kind === "expanded" && (
                <div className="px-4"><ListPagination page={checksView.page} pageSize={100} total={report.check_runs.length} onPageChange={(page) => setChecksView({ kind: "expanded", page })} /></div>
              )}
              {report.check_runs.length === 0 ? <p className="px-4 pb-4 text-sm text-slate-600">No checks were run.</p> : (
                <Table>
                  <TableHeader><TableRow><TableHead>Check</TableHead><TableHead>Project</TableHead><TableHead>Status</TableHead><TableHead>Detail</TableHead></TableRow></TableHeader>
                  <TableBody>
                    {checksView.kind === "expanded" && report.check_runs.slice(checksView.page * 100, (checksView.page + 1) * 100).map((check) => (
                      <TableRow key={`${check.check_id}:${check.subject.project_root}`}>
                        <TableCell className="font-medium">{check.check_id}</TableCell>
                        <TableCell>{check.subject.project_root}</TableCell>
                        <TableCell className={check.status === "failed" ? "text-red-700" : "text-slate-600"}>{check.status}</TableCell>
                        <TableCell className="whitespace-normal">{[check.reason_code, check.detail].filter(Boolean).join(" · ") || "Completed"}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </details>

            <details className="rounded-xl border border-slate-200" onToggle={(event) => setSourcesView(event.currentTarget.open ? { kind: "expanded", page: 0 } : { kind: "collapsed" })}>
              <summary className="cursor-pointer px-4 py-3 font-semibold text-slate-950">Extraction status · {report.sources.length} sources</summary>
              {sourcesView.kind === "expanded" && (
                <div className="px-4"><ListPagination page={sourcesView.page} pageSize={100} total={report.sources.length} onPageChange={(page) => setSourcesView({ kind: "expanded", page })} /></div>
              )}
              {report.sources.length === 0 ? <p className="px-4 pb-4 text-sm text-slate-600">No dependency sources detected.</p> : (
                <Table>
                  <TableHeader><TableRow><TableHead>Source</TableHead><TableHead>Detector</TableHead><TableHead>Presence</TableHead><TableHead>Extraction</TableHead><TableHead>Diagnostics</TableHead></TableRow></TableHeader>
                  <TableBody>
                    {sourcesView.kind === "expanded" && report.sources.slice(sourcesView.page * 100, (sourcesView.page + 1) * 100).map((source) => (
                      <TableRow key={`${source.detector}:${source.path}`}>
                        <TableCell className="font-medium">{source.path}<span className="mt-1 block text-xs font-normal text-slate-500">{source.form}</span></TableCell>
                        <TableCell>{source.detector}</TableCell>
                        <TableCell>{source.analysis.presence}</TableCell>
                        <TableCell className={source.analysis.extraction === "failed" ? "text-red-700" : "text-slate-600"}>{source.analysis.extraction}</TableCell>
                        <TableCell className="min-w-64 whitespace-normal">
                          {source.diagnostics.length === 0 ? "None" : source.diagnostics.map((diagnostic, index) => (
                            <p key={`${diagnostic.code}:${index}`}><span className="font-medium">{diagnostic.severity} · {diagnostic.code}</span> {diagnostic.message}</p>
                          ))}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </details>
          </>
        )}
      </CardContent>
    </Card>
  );
}
