import React from "react";
import Link from "next/link";
import { z } from "zod";

import { EmptyInventory } from "@/components/empty-inventory";
import { ScannerReportCard } from "@/components/scanner-report-card";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { getScanReportServer, getScanServer, listRepositoryOptionsServer, listScansServer } from "@/lib/queries";

type ChecksParams = { repository_id?: string; scan_id?: string; page?: string };
type Repository = Awaited<ReturnType<typeof listRepositoryOptionsServer>>[number];
type Scan = Awaited<ReturnType<typeof getScanServer>>;
type ProjectChecks = {
  repository: Repository;
  scans: Scan[];
  selected: { scan: Scan; report: Awaited<ReturnType<typeof getScanReportServer>> } | null;
};
type Selection =
  | { kind: "ready"; projects: ProjectChecks[]; page: number; totalPages: number; total: number }
  | { kind: "missing-repository" | "missing-scan" | "scan-repository-mismatch" };

async function loadChecks(params: ChecksParams) {
  const repositoryId = params.repository_id?.trim() ?? "";
  const scanId = params.scan_id?.trim() ?? "";
  const repositories = await listRepositoryOptionsServer();
  const repository = repositories.find((item) => item.id === repositoryId);
  const filters = { repositories, repositoryId, scanId };
  if (repositoryId && !repository) {
    return { ...filters, selection: { kind: "missing-repository" } satisfies Selection };
  }

  let exactScan: Scan | undefined;
  if (scanId) {
    if (!z.string().uuid().safeParse(scanId).success) {
      return { ...filters, selection: { kind: "missing-scan" } satisfies Selection };
    }
    try {
      exactScan = await getScanServer(scanId);
    } catch (error) {
      if (!(error instanceof ApiError) || (error.status !== 404 && error.status !== 400)) throw error;
      return { ...filters, selection: { kind: "missing-scan" } satisfies Selection };
    }
    if (!repositories.some((item) => item.id === exactScan?.repository_id)) {
      return { ...filters, selection: { kind: "missing-scan" } satisfies Selection };
    }
    if (repository && exactScan.repository_id !== repository.id) {
      return { ...filters, selection: { kind: "scan-repository-mismatch" } satisfies Selection };
    }
  }

  const scopedRepositories = exactScan
    ? repositories.filter((item) => item.id === exactScan.repository_id)
    : repository ? [repository] : repositories;
  const totalPages = Math.max(1, Math.ceil(scopedRepositories.length / 10));
  const requestedPage = Number(params.page);
  const page = Math.min(totalPages, Number.isSafeInteger(requestedPage) && requestedPage > 0 ? requestedPage : 1);
  const visibleRepositories = scopedRepositories.slice((page - 1) * 10, page * 10);
  const projects = await Promise.all(visibleRepositories.map(async (item): Promise<ProjectChecks> => {
    const scans = await listScansServer(item.id, "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z");
    const scan = exactScan ?? scans[0];
    return {
      repository: item,
      scans,
      selected: scan ? { scan, report: await getScanReportServer(scan.id) } : null
    };
  }));
  return { ...filters, selection: { kind: "ready", projects, page, totalPages, total: scopedRepositories.length } satisfies Selection };
}

export default async function ChecksPage({ searchParams }: { searchParams?: Promise<ChecksParams> }) {
  const { repositories, repositoryId, scanId, selection } = await loadChecks((await searchParams) ?? {});

  if (repositories.length === 0 && !repositoryId && !scanId) {
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

  const projects = selection.kind === "ready" ? selection.projects : [];
  const scanIsListed = projects.some((project) => project.scans.some((scan) => scan.id === scanId));
  const selectedScan = projects.find((project) => project.selected?.scan.id === scanId)?.selected?.scan;

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <h1 className="text-2xl font-semibold leading-none tracking-tight">Checks</h1>
          <CardDescription>CLI policy checks and findings. By default, the latest scan for each project.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <form action="/app/checks" method="get" aria-label="Choose repository" className="grid min-w-0 content-start gap-3">
            <div className="grid min-w-0 gap-2">
              <Label htmlFor="repository_id">Repository</Label>
              <select key={repositoryId} id="repository_id" name="repository_id" defaultValue={repositoryId} className="h-10 w-full min-w-0 rounded-md border border-input bg-background px-3 text-sm">
                <option value="">All projects</option>
                {repositoryId && !repositories.some((item) => item.id === repositoryId) ? <option value={repositoryId}>Unavailable repository</option> : null}
                {repositories.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
              </select>
            </div>
            <Button type="submit">Apply repository</Button>
          </form>
          <form action="/app/checks" method="get" aria-label="Choose scan" className="grid min-w-0 content-start gap-3">
            {repositoryId ? <input type="hidden" name="repository_id" value={repositoryId} /> : null}
            <div className="grid min-w-0 gap-2">
              <Label htmlFor="scan_id">Scan</Label>
              <select key={scanId} id="scan_id" name="scan_id" defaultValue={scanId} aria-describedby="scan-help" className="h-10 w-full min-w-0 rounded-md border border-input bg-background px-3 text-sm">
                <option value="">Latest scan per project</option>
                {scanId && !scanIsListed ? <option value={scanId}>{selectedScan ? `${selectedScan.commit_sha} · ${scanId}` : "Unavailable scan"}</option> : null}
                {projects.map((project) => (
                  <optgroup key={project.repository.id} label={project.repository.name}>
                    {project.scans.map((scan, index) => (
                      <option key={scan.id} value={scan.id}>
                        {index === 0 ? "Latest · " : ""}{new Date(scan.scanned_at).toLocaleString("en-GB")} · {scan.commit_sha} · {scan.id}
                      </option>
                    ))}
                  </optgroup>
                ))}
              </select>
              <p id="scan-help" className="text-sm text-slate-500">Choose a repository to browse its scan history. All projects shows scans from the current page.</p>
            </div>
            <Button type="submit">Apply scan</Button>
          </form>
          <Link className="text-sm font-medium text-slate-900 underline-offset-4 hover:underline" href="/app/checks">Clear filters</Link>
        </CardContent>
      </Card>

      {selection.kind === "missing-repository" ? (
        <p className="text-sm text-slate-600">This repository is unavailable. Select another repository above.</p>
      ) : selection.kind === "missing-scan" ? (
        <p className="text-sm text-slate-600">This scan is unavailable. Select another scan or clear the scan filter above.</p>
      ) : selection.kind === "scan-repository-mismatch" ? (
        <p className="text-sm text-slate-600">This scan does not belong to the selected repository. Select another scan or repository above.</p>
      ) : (
        <>
          {!repositoryId && !scanId ? (
            <nav aria-label="Project pages" className="flex flex-wrap items-center gap-4 text-sm text-slate-600">
              <p>Showing projects {(selection.page - 1) * 10 + 1}–{Math.min(selection.page * 10, selection.total)} of {selection.total}. Page {selection.page} of {selection.totalPages}.</p>
              {selection.page > 1 ? <Link className="font-medium text-slate-900 hover:underline" href={`/app/checks?page=${selection.page - 1}`}>Previous</Link> : null}
              {selection.page < selection.totalPages ? <Link className="font-medium text-slate-900 hover:underline" href={`/app/checks?page=${selection.page + 1}`}>Next</Link> : null}
            </nav>
          ) : null}
          {selection.projects.map(({ repository, scans, selected }) => (
            <section key={repository.id} aria-label={`Checks for ${repository.name}`} className="space-y-4 rounded-xl border border-slate-200 bg-white/80 p-4 text-sm">
              <h2 className="text-lg font-semibold text-slate-950">
                <Link className="underline-offset-4 hover:underline [overflow-wrap:anywhere]" href={`/app/repositories/${repository.id}/manifests`}>{repository.name}</Link>
              </h2>
              {!selected ? (
                <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-slate-600">
                  <p>No CLI scans have been uploaded for {repository.name}.</p>
                  <details className="mt-3">
                    <summary className="cursor-pointer font-medium text-slate-900">How to upload a scan</summary>
                    <div className="mt-3"><EmptyInventory /></div>
                  </details>
                </div>
              ) : (
                <>
                  <dl className="grid gap-3 sm:grid-cols-2">
                    <div className="min-w-0"><dt className="text-slate-500">Commit</dt><dd className="break-all font-mono">{selected.scan.commit_sha}</dd></div>
                    <div><dt className="text-slate-500">Scanned at{selected.scan.id === scans[0]?.id ? " · Latest scan" : ""}</dt><dd><time dateTime={selected.scan.scanned_at} title={new Date(selected.scan.scanned_at).toISOString()}>{new Date(selected.scan.scanned_at).toLocaleString("en-GB")}</time></dd></div>
                    <div className="min-w-0 sm:col-span-2"><dt className="text-slate-500">Scan ID</dt><dd className="break-all font-mono">{selected.scan.id}</dd></div>
                  </dl>
                  <div className="flex flex-wrap gap-4">
                    <Link className="font-medium text-slate-900 underline-offset-4 hover:underline" href={`/app/scans/${selected.scan.id}`}>Scan details</Link>
                    <Link className="font-medium text-slate-900 underline-offset-4 hover:underline" href={`/app/checks?${new URLSearchParams({ repository_id: repository.id, scan_id: selected.scan.id })}`}>Permalink to these checks</Link>
                  </div>
                  <ScannerReportCard key={selected.scan.id} scanId={selected.scan.id} reportState={selected.report} />
                </>
              )}
            </section>
          ))}
        </>
      )}
    </section>
  );
}
