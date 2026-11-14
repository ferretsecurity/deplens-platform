import React from "react";
import Link from "next/link";

import { EmptyInventory } from "@/components/empty-inventory";
import { ScannerReportCard } from "@/components/scanner-report-card";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { getScanReportServer, listRepositoryOptionsServer, listScansServer } from "@/lib/queries";

export default async function ChecksPage({
  searchParams
}: {
  searchParams?: Promise<{ repository_id?: string; scan_id?: string }>;
}) {
  const params = (await searchParams) ?? {};
  const repositories = await listRepositoryOptionsServer();

  if (repositories.length === 0) {
    return (
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <h1 className="text-2xl font-semibold leading-none tracking-tight">Checks</h1>
          <CardDescription>No repositories have been uploaded yet.</CardDescription>
        </CardHeader>
        <CardContent><EmptyInventory /></CardContent>
      </Card>
    );
  }

  const repositoryId = params.repository_id ?? repositories[0].id;
  const repository = repositories.find((item) => item.id === repositoryId);
  const scans = repository
    ? await listScansServer(repository.id, "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z")
    : [];
  const scan = params.scan_id === undefined ? scans[0] : scans.find((item) => item.id === params.scan_id);
  const reportState = scan ? await getScanReportServer(scan.id) : null;

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <h1 className="text-2xl font-semibold leading-none tracking-tight">Checks</h1>
          <CardDescription>CLI policy checks and findings from an uploaded scan.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <form action="/app/checks" method="get" aria-label="Choose repository" className="grid min-w-0 content-start gap-3">
            <div className="grid min-w-0 gap-2">
              <Label htmlFor="repository_id">Repository</Label>
              <select id="repository_id" name="repository_id" defaultValue={repositoryId} className="h-10 w-full min-w-0 rounded-md border border-input bg-background px-3 text-sm">
                {!repository ? <option value={repositoryId}>Unavailable repository</option> : null}
                {repositories.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
              </select>
            </div>
            <Button type="submit">Apply repository</Button>
          </form>
          {repository && (scans.length > 0 || params.scan_id !== undefined) ? (
            <form action="/app/checks" method="get" aria-label="Choose scan" className="grid min-w-0 content-start gap-3">
              <input type="hidden" name="repository_id" value={repository.id} />
              <div className="grid min-w-0 gap-2">
                <Label htmlFor="scan_id">Scan</Label>
                <select id="scan_id" name="scan_id" defaultValue={params.scan_id ?? scan?.id} className="h-10 w-full min-w-0 rounded-md border border-input bg-background px-3 text-sm">
                  {!scan ? <option value={params.scan_id}>Unavailable scan</option> : null}
                  {scans.map((item, index) => (
                    <option key={item.id} value={item.id}>
                      {index === 0 ? "Latest · " : ""}{new Date(item.scanned_at).toLocaleString("en-GB")} · {item.commit_sha} · {item.id}
                    </option>
                  ))}
                </select>
              </div>
              <Button type="submit">Apply scan</Button>
            </form>
          ) : null}
        </CardContent>
      </Card>

      {!repository ? (
        <p className="text-sm text-slate-600">This repository is unavailable. Select another repository above.</p>
      ) : !scan ? (
        <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
          {params.scan_id !== undefined ? (
            <p>This scan is unavailable for the selected repository. Select another scan or repository above.</p>
          ) : (
            <>
              <p>No CLI scans have been uploaded for {repository.name}.</p>
              <details className="mt-3">
                <summary className="cursor-pointer font-medium text-slate-900">How to upload a scan</summary>
                <div className="mt-3"><EmptyInventory /></div>
              </details>
            </>
          )}
        </div>
      ) : (
        <div className="rounded-xl border border-slate-200 bg-white/80 p-4 text-sm">
          <Link className="font-semibold text-slate-950 underline-offset-4 hover:underline [overflow-wrap:anywhere]" href={`/app/repositories/${repository.id}/manifests`}>{repository.name}</Link>
          <dl className="mt-3 grid gap-3 sm:grid-cols-2">
            <div className="min-w-0"><dt className="text-slate-500">Commit</dt><dd className="break-all font-mono">{scan.commit_sha}</dd></div>
            <div><dt className="text-slate-500">Scanned at{scan.id === scans[0]?.id ? " · Latest scan" : ""}</dt><dd><time dateTime={scan.scanned_at} title={new Date(scan.scanned_at).toISOString()}>{new Date(scan.scanned_at).toLocaleString("en-GB")}</time></dd></div>
            <div className="min-w-0 sm:col-span-2"><dt className="text-slate-500">Scan ID</dt><dd className="break-all font-mono">{scan.id}</dd></div>
          </dl>
          <div className="mt-4 flex flex-wrap gap-4">
            <Link className="font-medium text-slate-900 underline-offset-4 hover:underline" href={`/app/scans/${scan.id}`}>Scan details</Link>
            <Link className="font-medium text-slate-900 underline-offset-4 hover:underline" href={`/app/checks?${new URLSearchParams({ repository_id: repository.id, scan_id: scan.id })}`}>Permalink to these checks</Link>
          </div>
        </div>
      )}

      {scan && reportState ? <ScannerReportCard key={scan.id} scanId={scan.id} reportState={reportState} /> : null}
    </section>
  );
}
