import React from "react";
import Link from "next/link";
import { ApiError } from "@/lib/api";
import { listPackagesServer } from "@/lib/queries";
import { FilePath } from "@/components/file-path";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardDescription } from "@/components/ui/card";

type LookupOutcome = { kind: "success"; data: Awaited<ReturnType<typeof listPackagesServer>> } | { kind: "invalid" };

const ecosystems = ["npm", "pypi", "cargo", "go", "maven", "nuget", "gem", "composer"];
const statusLabels = { issue: "Issue", warning: "Warning", unknown: "Unknown" };
const statusStyles = { issue: "border-red-300 bg-red-50", warning: "border-amber-300 bg-amber-50", unknown: "border-slate-300 bg-slate-50" };
const sourceLabels = { cli: "CLI", socket: "Socket", snyk: "Snyk" };

function first(value: string | string[] | undefined) {
  return (Array.isArray(value) ? value[0] : value)?.trim() ?? "";
}

export default async function AssessmentPage({ searchParams }: {
  searchParams?: Promise<{ q?: string | string[]; ecosystem?: string | string[]; affected_range?: string | string[]; page?: string | string[] }>;
}) {
  const params = await searchParams ?? {};
  const q = first(params.q);
  const ecosystem = first(params.ecosystem) || "npm";
  const range = first(params.affected_range);
  const parsedPage = Number(first(params.page));
  const page = Number.isSafeInteger(parsedPage) && parsedPage > 0 ? parsedPage : 1;
  const submitted = params.q !== undefined || params.affected_range !== undefined;
  const outcome = submitted ? await listPackagesServer({ q, ecosystem, affected_range: range, version: "", page, page_size: 25 })
    .then<LookupOutcome>(data => ({ kind: "success", data }))
    .catch<LookupOutcome>((error: unknown) => {
      if (error instanceof ApiError && error.status === 400) return { kind: "invalid" };
      throw error;
    }) : null;
  function pageHref(target: number) {
    return `/app/packages/assess?${new URLSearchParams({ q, ecosystem, affected_range: range, page: String(target) })}`;
  }

  return <section className="space-y-6">
    <Link className="text-blue-700 underline" href="/app/packages">Back to packages</Link>
    <Card>
      <CardHeader>
        <h1 className="text-2xl font-semibold">Assess a vulnerable version range</h1>
        <CardDescription>Find affected recorded versions and declarations that permit vulnerable versions. Results describe scanned evidence, not what is deployed.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <form action="/app/packages/assess" method="get" className="grid items-end gap-4 md:grid-cols-[1fr_1fr_2fr_auto]">
          <div className="space-y-2">
            <Label htmlFor="ecosystem">Ecosystem</Label>
            <select id="ecosystem" name="ecosystem" defaultValue={ecosystem} className="h-10 w-full rounded-md border border-slate-300 bg-white px-3">
              {ecosystems.map(value => <option key={value} value={value}>{value}</option>)}
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="q">Exact package name</Label>
            <Input id="q" name="q" defaultValue={q} maxLength={200} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="affected_range">Affected range</Label>
            <Input id="affected_range" name="affected_range" defaultValue={range} placeholder=">0.0.0 <=2.3.4" maxLength={200} required aria-describedby="range-help" />
          </div>
          <Button type="submit">Assess</Button>
        </form>
        <p id="range-help" className="text-sm text-slate-600">Use the ecosystem’s range syntax. For example, npm: &gt;0.0.0 &lt;=2.3.4; PyPI: &gt;0,&lt;=2.3.4; Maven: (0,2.3.4].</p>
        <p className="text-sm text-slate-600">Every corresponding lockfile is checked. Missing or incomplete lockfile evidence keeps a manifest warning. Unknown results need further investigation.</p>
        {outcome?.kind === "invalid" ? <p role="alert" className="text-red-700">Enter an exact package name and a valid, nonempty affected range for the selected ecosystem.</p> : null}
        {outcome?.kind === "success" ? <>
          <p role="status">{outcome.data.pagination.total} findings</p>
          {outcome.data.items.length === 0 ? <p>{outcome.data.pagination.total > 0 ? "No findings on this page." : "No issues, warnings, or unknown matches in the available inventory. This does not prove coverage of unscanned dependencies."}</p> : null}
          {outcome.data.items.map((item, index) => {
            const evidence = item.assessment;
            if (!evidence) return null;
            return <article key={index} className={`space-y-2 rounded-lg border p-4 ${statusStyles[evidence.status]}`}>
              <h2 className="font-semibold">{statusLabels[evidence.status]}: {item.name}{item.version ? `@${item.version}` : ""}</h2>
              <p><Link className="text-blue-700 underline" href={`/app/repositories/${item.repository_id}/manifests`}>{item.repository_name}</Link> · {sourceLabels[item.source]} · {evidence.scope}{evidence.root ? `: ${evidence.root}` : ""}</p>
              {item.manifest_path ? <FilePath path={item.manifest_path} /> : <p>File path not reported</p>}
              {item.constraint ? <p>Declared constraint: <code>{item.constraint}</code></p> : null}
              <ul className="list-inside list-disc">{evidence.reasons.map(reason => <li key={reason}>{reason}</li>)}</ul>
              {evidence.lockfiles.length > 1 ? <p>Competing lockfiles: {evidence.lockfiles.join(", ")}</p> : null}
              {evidence.scope === "workspace" ? <p>The shared lockfile contains this version. Affected member projects have not been established.</p> : null}
            </article>;
          })}
          <nav aria-label="Assessment pages" className="flex items-center gap-4">
            {outcome.data.pagination.has_previous ? <Link href={pageHref(page - 1)}>Previous</Link> : null}
            {outcome.data.pagination.total_pages > 0 ? <span>Page {page} of {outcome.data.pagination.total_pages}</span> : null}
            {outcome.data.pagination.has_next ? <Link href={pageHref(page + 1)}>Next</Link> : null}
          </nav>
        </> : null}
      </CardContent>
    </Card>
  </section>;
}
