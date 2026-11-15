"use client";

// Throwaway: three simpler Checks layouts on /app/checks?variant=A|B|C.
import React, { useState } from "react";
import Link from "next/link";
import type { ProjectChecks } from "./page";
import type { ScannerReport } from "@/lib/scanner-report";
import { PrototypeSwitcher } from "@/components/prototype-switcher";

const variants = ["B · Original inbox", "D · Compact list", "E · Detail pane", "F · Group by check"];

function findings(project: ProjectChecks) {
  return project.selected?.report.kind === "available" ? project.selected.report.report.findings : [];
}

function status(project: ProjectChecks) {
  if (!project.selected) return "Not scanned";
  if (project.selected.report.kind !== "available") return "Report unavailable";
  const report = project.selected.report.report;
  const gaps = report.check_runs.filter(check => check.status !== "completed").length + report.sources.filter(source => source.analysis.extraction !== "complete").length;
  return `${report.findings.length} ${report.findings.length === 1 ? "finding" : "findings"}${gaps ? ` · ${gaps} coverage gaps` : ""}`;
}

function Finding({ finding }: { finding: ScannerReport["findings"][number] }) {
  return <details className="border-b border-slate-200 py-4 last:border-0">
    <summary className="flex cursor-pointer items-start gap-3 list-none">
      <span className={`mt-0.5 w-16 shrink-0 text-xs font-medium ${["critical", "high"].includes(finding.severity) ? "text-red-700" : "text-slate-500"}`}>{finding.severity}</span>
      <span className="min-w-0 flex-1 font-medium">{finding.summary}<span className="mt-1 block break-all text-xs font-normal text-slate-500">{finding.locations.map(location => location.path).join(", ") || finding.subject.project_root}</span></span>
      <span className="text-slate-400">+</span>
    </summary>
    <div className="mt-4 space-y-3 pl-[76px] text-sm text-slate-600"><p>{finding.remediation}</p><p className="text-xs">Check: {finding.check_id}</p><dl>{Object.entries(finding.evidence).map(([key, value]) => <div key={key}><dt className="inline font-medium">{key}: </dt><dd className="inline break-all">{value}</dd></div>)}</dl></div>
  </details>;
}

function ProjectDetail({ project }: { project: ProjectChecks }) {
  const selected = project.selected;
  return <div className="min-w-0">
    <div className="mb-4 flex flex-wrap items-center justify-between gap-2"><h2 className="text-lg font-semibold">{project.repository.name}</h2><span className="text-xs text-slate-500">{status(project)}</span></div>
    {findings(project).length ? findings(project).map(finding => <Finding key={finding.fingerprint} finding={finding} />) : <p className="py-6 text-sm text-slate-500">{selected?.report.kind === "available" ? "No findings reported. Check coverage before treating this as clear." : status(project)}</p>}
    {selected && <details className="mt-6 border-t border-slate-200 pt-4 text-sm text-slate-500"><summary className="cursor-pointer">Scan details & coverage</summary><div className="mt-4 space-y-3"><p className="break-all">Commit {selected.scan.commit_sha} · {new Date(selected.scan.scanned_at).toLocaleString("en-GB")}</p><p className="break-all">Scan {selected.scan.id}</p>{selected.report.kind === "available" && <><ul>{selected.report.report.check_runs.map((check, index) => <li key={index}>{check.check_id} · {check.subject.project_root} · {check.status}{check.detail ? ` · ${check.detail}` : ""}</li>)}</ul><ul>{selected.report.report.sources.map((source, index) => <li key={index}>{source.path} · {source.analysis.extraction}</li>)}</ul></>}<Link className="block underline" href={`/app/scans/${selected.scan.id}`}>Full scan details</Link></div></details>}
  </div>;
}

export function ChecksPrototype({ projects, variant, total, page, totalPages }: { projects: ProjectChecks[]; variant: string; total: number; page: number; totalPages: number }) {
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState(projects[0]?.repository.id ?? "");
  const visible = projects.filter(project => project.repository.name.toLowerCase().includes(query.toLowerCase()));
  const selected = visible.find(project => project.repository.id === selectedId) ?? visible[0];
  const count = visible.reduce((sum, project) => sum + findings(project).length, 0);
  return <section className="pb-24 text-slate-900">
    <header className="mb-8 flex flex-wrap items-end justify-between gap-4"><div><h1 className="text-2xl font-semibold tracking-tight">Checks</h1><p className="mt-2 text-sm text-slate-500">{count} findings across {visible.length} repositories on this page</p></div><input aria-label="Filter repositories on this page" placeholder="Find a repository…" value={query} onChange={event => setQuery(event.target.value)} className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm" /></header>
    {variant === "A" && <div className="border-y border-slate-200 bg-white">{visible.map(project => <details key={project.repository.id} className="group border-b border-slate-200 last:border-0"><summary className="flex cursor-pointer list-none flex-wrap items-center gap-4 px-5 py-5"><span className="text-slate-400 group-open:rotate-90">›</span><span className="min-w-40 flex-1 font-medium">{project.repository.name}</span><span className="text-sm text-slate-600">{status(project)}</span><span className="w-24 text-right font-mono text-xs text-slate-400">{project.selected?.scan.commit_sha.slice(0, 7) ?? "No scan"}</span></summary><div className="border-t border-slate-100 px-8 py-6"><ProjectDetail project={project} /></div></details>)}</div>}
    {variant === "B" && <div className="mx-auto max-w-3xl"><p className="mb-5 text-xs font-medium uppercase tracking-widest text-slate-400">Findings to review</p>{visible.filter(project => findings(project).length > 0).map(project => <div key={project.repository.id} className="mb-8"><h2 className="border-b border-slate-200 pb-2 text-sm font-semibold">{project.repository.name}</h2>{findings(project).map(finding => <Finding key={finding.fingerprint} finding={finding} />)}</div>)}{count === 0 && <p className="py-8 text-sm text-slate-500">No findings in this selection.</p>}<details className="mt-8 border-t border-slate-200 pt-4 text-sm text-slate-500"><summary className="cursor-pointer">Coverage & scan details · {visible.length} repositories</summary><div className="space-y-8 pt-6">{visible.map(project => <ProjectDetail key={project.repository.id} project={project} />)}</div></details></div>}
    {variant === "C" && <div className="grid min-h-96 overflow-hidden rounded-lg border border-slate-200 bg-white md:grid-cols-[240px_minmax(0,1fr)]"><nav aria-label="Repositories to review" className="border-b border-slate-200 bg-slate-50 p-2 md:border-r md:border-b-0">{visible.map(project => <button key={project.repository.id} onClick={() => setSelectedId(project.repository.id)} aria-pressed={selected?.repository.id === project.repository.id} className={`mb-1 block w-full rounded-md p-3 text-left text-sm ${selected?.repository.id === project.repository.id ? "bg-white shadow-sm ring-1 ring-slate-200" : "hover:bg-slate-100"}`}><span className="block font-medium">{project.repository.name}</span><span className="mt-1 block text-xs text-slate-500">{status(project)}</span></button>)}</nav><div className="min-w-0 p-6">{selected && <ProjectDetail project={selected} />}</div></div>}
    {visible.length === 0 && <p className="py-8 text-sm text-slate-500">No repositories match.</p>}
    {totalPages > 1 && <nav className="mt-6 flex gap-4 text-sm"><span>Page {page} of {totalPages} · {total} repositories</span>{page > 1 && <Link href={`?variant=${variant}&page=${page - 1}`}>Previous</Link>}{page < totalPages && <Link href={`?variant=${variant}&page=${page + 1}`}>Next</Link>}</nav>}
    <PrototypeSwitcher variants={variant === "A" || variant === "C" ? ["A · Repository list", "B · Original inbox", "C · Review workspace"] : variants} current={variant} state={`${visible.length}/${projects.length} visible · ${count} findings · ${variant === "C" ? selected?.repository.name ?? "None selected" : "Click rows for details"}`} />
  </section>;
}
