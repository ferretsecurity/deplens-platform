"use client";

// Throwaway round 2: compare a flat list, finding detail pane, and check groups.
import React, { useState } from "react";
import Link from "next/link";
import type { ProjectChecks } from "./page";
import { PrototypeSwitcher } from "@/components/prototype-switcher";

const variants = ["B · Original inbox", "D · Compact list", "E · Detail pane", "F · Group by check"];
const ranks = { critical: 0, high: 1, medium: 2, low: 3, info: 4 };
function entries(projects: ProjectChecks[]) {
  return projects.flatMap(({ repository, selected }) => selected?.report.kind === "available"
    ? selected.report.report.findings.map(finding => ({ ...finding, repository, scan: selected.scan, key: `${selected.scan.id}:${finding.fingerprint}` })) : []
  ).sort((a, b) => ranks[a.severity] - ranks[b.severity]);
}
type Entry = ReturnType<typeof entries>[number];

function Severity({ value }: { value: Entry["severity"] }) {
  return <span className={`inline-flex items-center gap-2 text-xs ${ranks[value] < 2 ? "text-red-700" : ranks[value] === 2 ? "text-amber-700" : "text-slate-500"}`}><span className="h-1.5 w-1.5 shrink-0 rounded-full bg-current" />{value}</span>;
}
function Detail({ entry }: { entry: Entry }) {
  return <div className="space-y-5 text-sm">
    <div><h3 className="mb-2 font-medium text-slate-900">Suggested fix</h3><p className="leading-relaxed text-slate-600">{entry.remediation}</p></div>
    <dl className="space-y-2 text-slate-500"><div><dt className="inline">Repository </dt><dd className="inline text-slate-900">{entry.repository.name}</dd></div><div><dt className="inline">Location </dt><dd className="inline break-all font-mono text-xs">{entry.locations.map(location => location.path).join(", ") || entry.subject.project_root}</dd></div><div><dt className="inline">Check </dt><dd className="inline">{entry.check_id}</dd></div></dl>
    <details className="text-slate-500"><summary className="cursor-pointer">Evidence & scan</summary><dl className="mt-3 space-y-2">{Object.entries(entry.evidence).map(([key, value]) => <div key={key}><dt className="font-medium">{key}</dt><dd className="break-all">{value}</dd></div>)}</dl><p className="mt-3">{entry.scan.commit_sha.slice(0, 7)} · {new Date(entry.scan.scanned_at).toLocaleString("en-GB")}</p><Link className="mt-2 inline-block underline" href={`/app/scans/${entry.scan.id}`}>Open scan</Link></details>
  </div>;
}
export function VariantD({ items }: { items: Entry[] }) {
  return <div className="border-y border-slate-200 bg-white">{items.map(entry => <details key={entry.key} className="group border-b border-slate-200 last:border-0"><summary className="grid cursor-pointer list-none items-start gap-3 px-4 py-4 sm:grid-cols-[78px_minmax(0,1fr)_140px_16px]"><Severity value={entry.severity} /><span className="min-w-0 text-sm font-medium">{entry.summary}<span className="mt-1 block truncate font-mono text-xs font-normal text-slate-400">{entry.locations[0]?.path ?? entry.subject.project_root}</span></span><span className="truncate text-xs text-slate-500">{entry.repository.name}</span><span className="hidden text-slate-400 group-open:rotate-90 sm:block">›</span></summary><div className="border-t border-slate-100 bg-slate-50/60 p-5 sm:pl-[106px]"><Detail entry={entry} /></div></details>)}</div>;
}
export function VariantE({ items, selected, onSelect }: { items: Entry[]; selected: Entry | undefined; onSelect: (key: string) => void }) {
  return <div className="grid overflow-hidden rounded-lg border border-slate-200 bg-white lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]"><nav aria-label="Findings to review" className="max-h-[540px] overflow-auto border-b border-slate-200 lg:border-r lg:border-b-0">{items.map(entry => <button key={entry.key} onClick={() => onSelect(entry.key)} aria-pressed={selected?.key === entry.key} className={`block w-full border-b border-slate-100 border-l-2 px-5 py-4 text-left ${selected?.key === entry.key ? "border-l-slate-900 bg-slate-50" : "border-l-transparent hover:bg-slate-50"}`}><Severity value={entry.severity} /><span className="mt-2 block text-sm font-medium">{entry.summary}</span><span className="mt-1 block truncate text-xs text-slate-500">{entry.repository.name} · {entry.locations[0]?.path ?? entry.subject.project_root}</span></button>)}</nav><article aria-label="Selected finding" className="min-w-0 p-6">{selected && <><Severity value={selected.severity} /><h2 className="mt-3 mb-6 text-xl font-semibold tracking-tight">{selected.summary}</h2><Detail key={selected.key} entry={selected} /></>}</article></div>;
}
export function VariantF({ items }: { items: Entry[] }) {
  const groups = new Map<string, Entry[]>();
  for (const entry of items) groups.set(entry.check_id, [...(groups.get(entry.check_id) ?? []), entry]);
  return <div className="space-y-8">{Array.from(groups, ([checkId, group]) => <section key={checkId} className="grid gap-4 border-t border-slate-200 pt-5 md:grid-cols-[200px_minmax(0,1fr)]"><header><h2 className="break-words text-sm font-semibold">{checkId}</h2><p className="mt-2 text-xs text-slate-500">{group.length} findings · {new Set(group.map(entry => entry.repository.id)).size} repositories</p></header><div className="min-w-0">{group.map(entry => <details key={entry.key} className="group border-b border-slate-200 py-3 first:pt-0 last:border-0"><summary className="flex cursor-pointer list-none items-start gap-3"><span className="min-w-0 flex-1"><span className="block text-sm font-medium">{entry.summary}</span><span className="mt-1 block truncate text-xs text-slate-500">{entry.repository.name} · {entry.locations[0]?.path ?? entry.subject.project_root}</span></span><Severity value={entry.severity} /><span className="text-slate-400 group-open:rotate-90">›</span></summary><div className="py-5"><Detail entry={entry} /></div></details>)}</div></section>)}</div>;
}

export function FindingsPrototype({ projects, variant, page, totalPages }: { projects: ProjectChecks[]; variant: string; page: number; totalPages: number }) {
  const [query, setQuery] = useState("");
  const [severity, setSeverity] = useState("all");
  const [selectedKey, setSelectedKey] = useState("");
  const all = entries(projects);
  const items = all.filter(entry => (severity === "all" || entry.severity === severity) && `${entry.summary} ${entry.repository.name} ${entry.check_id} ${entry.locations.map(location => location.path).join(" ")}`.toLowerCase().includes(query.toLowerCase()));
  const selected = items.find(entry => entry.key === selectedKey) ?? items[0];
  const incomplete = projects.filter(project => !project.selected || project.selected.report.kind !== "available" || project.selected.report.report.check_runs.some(check => check.status !== "completed") || project.selected.report.report.sources.some(source => source.analysis.extraction !== "complete"));
  return <section className="pb-28 text-slate-900">
    <header className="mb-6"><h1 className="text-2xl font-semibold tracking-tight">Checks</h1><p className="mt-2 text-sm text-slate-500">{all.length} findings · Latest scan per repository</p></header>
    <div className="mb-6 flex flex-wrap items-center gap-3"><input aria-label="Search findings" placeholder="Search findings…" value={query} onChange={event => setQuery(event.target.value)} className="h-9 min-w-0 flex-1 rounded-md border border-slate-200 bg-white px-3 text-sm sm:max-w-80" /><select aria-label="Severity" value={severity} onChange={event => setSeverity(event.target.value)} className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm"><option value="all">All severities</option>{Object.keys(ranks).map(value => <option key={value} value={value}>{value}</option>)}</select><span className="ml-auto text-xs text-slate-400">Severity first</span></div>
    {items.length === 0 ? <p className="border-y border-slate-200 py-12 text-center text-sm text-slate-500">{all.length ? "No findings match these filters." : "No findings reported. See scan coverage below."}</p> : variant === "D" ? <VariantD items={items} /> : variant === "E" ? <VariantE items={items} selected={selected} onSelect={setSelectedKey} /> : <VariantF items={items} />}
    <details className="mt-6 text-xs text-slate-500"><summary className="cursor-pointer">{incomplete.length ? `${incomplete.length} repositories have incomplete coverage or no report` : "Scan coverage"}</summary><ul className="mt-3 space-y-2">{projects.map(project => <li key={project.repository.id}>{project.repository.name} · {!project.selected ? "Not scanned" : project.selected.report.kind !== "available" ? "Report unavailable" : `${project.selected.report.report.check_runs.filter(check => check.status !== "completed").length} skipped or failed checks · ${project.selected.report.report.sources.filter(source => source.analysis.extraction !== "complete").length} incomplete sources`}</li>)}</ul></details>
    {totalPages > 1 && <nav className="mt-4 flex gap-4 text-xs text-slate-500"><span>Repository batch {page} of {totalPages}. Findings are limited to this batch.</span>{page > 1 && <Link href={`?variant=${variant}&page=${page - 1}`}>Previous</Link>}{page < totalPages && <Link href={`?variant=${variant}&page=${page + 1}`}>Next</Link>}</nav>}
    <PrototypeSwitcher variants={variants} current={variant} state={`${items.length}/${all.length} findings · ${severity} · ${query ? `“${query}”` : "No search"}${variant === "E" ? ` · ${selected?.repository.name ?? "No selection"}` : ""}`} />
  </section>;
}
