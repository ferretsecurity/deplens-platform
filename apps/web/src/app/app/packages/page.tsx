import { EmptyInventory } from "@/components/empty-inventory";
import { FilePath } from "@/components/file-path";
import React from "react";
import Link from "next/link";

import { ApiError } from "@/lib/api";
import { listPackagesServer } from "@/lib/queries";
import type { PackageUsage } from "@/lib/packages";
import { dependencyOriginLabel } from "@/lib/dependency-origin";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const DEFAULT_PAGE_SIZE = 25;
const sourceLabels = { cli: "CLI", socket: "Socket", snyk: "Snyk" };

function firstParam(value: string | string[] | undefined) {
  return Array.isArray(value) ? value[0] : value;
}

function positiveInt(value: string | string[] | undefined, fallback: number) {
  const parsed = Number(firstParam(value));
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function packagesHref(params: { q: string; version: string; page: number; pageSize: number }) {
  const searchParams = new URLSearchParams();
  if (params.q) searchParams.set("q", params.q);
  if (params.version) searchParams.set("version", params.version);
  if (params.page > 1) searchParams.set("page", String(params.page));
  if (params.pageSize !== DEFAULT_PAGE_SIZE) searchParams.set("page_size", String(params.pageSize));
  const query = searchParams.toString();
  return query ? `/app/packages?${query}` : "/app/packages";
}

export default async function PackagesPage({
  searchParams
}: {
  searchParams?: Promise<{
    q?: string | string[];
    version?: string | string[];
    page?: string | string[];
    page_size?: string | string[];
  }>;
}) {
  const params = (await searchParams) ?? {};
  const query = (firstParam(params.q) ?? "").trim();
  const version = (firstParam(params.version) ?? "").trim();
  const page = positiveInt(params.page, 1);
  const pageSize = Math.min(100, positiveInt(params.page_size, DEFAULT_PAGE_SIZE));
  const result = await listPackagesServer({ q: query, version, page, page_size: pageSize }).catch((error: unknown) => {
    if (error instanceof ApiError && error.status === 400) return null;
    throw error;
  });
  const items = result?.items ?? [];
  const pagination = result?.pagination ?? { page, page_size: pageSize, total: 0, total_pages: 0, has_previous: false, has_next: false };
  const firstResult = items.length ? (pagination.page - 1) * pagination.page_size + 1 : 0;
  const lastResult = items.length ? firstResult + items.length - 1 : 0;
  const packageGroups = new Map<string, PackageUsage[]>();
  for (const item of items) {
    const identity = item.name.trim() || item.destination;
    const usages = packageGroups.get(identity);
    if (usages) usages.push(item);
    else packageGroups.set(identity, [item]);
  }

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <h1 className="text-2xl font-semibold leading-none tracking-tight">Packages</h1>
          <CardDescription>
            Find packages and direct URL or Git dependencies by repository, manifest and version. Local paths and workspace references remain in manifest details.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Link className="text-blue-700 underline" href="/app/packages/assess">Assess a vulnerable version range</Link>
          <form className="flex flex-col gap-3 sm:flex-row" method="get" action="/app/packages">
            <div className="grid flex-1 gap-2">
              <Label className="sr-only" htmlFor="q">
                Search packages
              </Label>
              <Input
                id="q"
                name="q"
                type="search"
                placeholder="Search by package name or destination"
                defaultValue={query}
                maxLength={200}
              />
            </div>
            <div className="grid gap-2 sm:w-56">
              <Label className="sr-only" htmlFor="version">Version</Label>
              <Input id="version" name="version" placeholder="Version, e.g. 1.103.1" defaultValue={version} maxLength={200} aria-invalid={!result} />
            </div>
            <input name="page_size" type="hidden" value={pagination.page_size} />
            <div className="flex gap-2">
              <Button type="submit">Search</Button>
              {query || version ? (
                <Button asChild variant="outline">
                  <Link href="/app/packages">Clear</Link>
                </Button>
              ) : null}
            </div>
          </form>
          {result ? <div className="text-sm text-slate-600">
            Showing {firstResult}-{lastResult} of {pagination.total} package usages
            {pagination.total_pages > 1 ? (
              <p className="mt-1">Package groups may continue across pages.</p>
            ) : null}
          </div> : null}
          {!result ? (
            <p role="alert" className="text-sm text-red-700">Enter a concrete version, such as 1.103.1, instead of a range or tag.</p>
          ) : items.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              {pagination.total > 0
                ? "No package usages on this page."
                : query || version
                  ? "No packages match this search."
                  : <p>No active packages found.</p>}
              {query || version || page > 1 ? <Link className="mt-3 inline-block underline" href="/app/packages">Reset search and page</Link> : <div className="mt-3"><EmptyInventory /></div>}
            </div>
          ) : (
            <div className="space-y-6">
              {Array.from(packageGroups, ([name, usages]) => (
                <section key={name} aria-label={name} className="space-y-3">
                  <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                    <h2 className="[overflow-wrap:anywhere] text-lg font-semibold text-slate-900">{name}</h2>
                    <p className="text-sm text-slate-600">
                      {usages.length} {usages.length === 1 ? "usage" : "usages"} on this page
                    </p>
                  </div>
                  <div>
                    <Table layout="reflow" className="package-usages" aria-label={`${name} usages`}>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-[20%]">Repository</TableHead>
                          <TableHead className="w-[20%]">Manifest file</TableHead>
                          <TableHead>Version / constraint</TableHead>
                          <TableHead>Ecosystem</TableHead>
                          <TableHead className="w-[25%]">Origin</TableHead>
                          <TableHead>Provider</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {usages.map((item) => (
                          <TableRow key={JSON.stringify(item)}>
                            <TableCell mobileLabel="Repository" className="table-cell-wide table-cell-identity">
                              <Link
                                className="font-medium text-blue-700 underline decoration-blue-300 underline-offset-4 hover:decoration-blue-700"
                                href={`/app/repositories/${item.repository_id}/manifests`}
                              >
                                {item.repository_name}
                              </Link>
                            </TableCell>
                            <TableCell mobileLabel="Manifest file" className="table-cell-wide">{item.manifest_path ? <FilePath path={item.manifest_path} /> : "Not reported"}</TableCell>
                            <TableCell mobileLabel="Version / constraint">
                              {[item.version, item.constraint].filter(Boolean).join(" / ") || "Unspecified"}
                            </TableCell>
                            <TableCell mobileLabel="Ecosystem">{item.ecosystem || "Unspecified"}</TableCell>
                            <TableCell mobileLabel="Origin" className="table-cell-wide">
                              <div>{dependencyOriginLabel(item.origin_kind)}</div>
                              {(item.origin_kind === "url" || item.origin_kind === "git") && item.destination ? (
                                <div className="[overflow-wrap:anywhere] mt-1 text-sm text-slate-600">{item.destination}</div>
                              ) : null}
                            </TableCell>
                            <TableCell mobileLabel="Provider">{sourceLabels[item.source]}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </section>
              ))}
            </div>
          )}
          {pagination.total > 0 ? (
            <div className="flex flex-col gap-3 border-t border-slate-200 pt-4 text-sm text-slate-600 sm:flex-row sm:items-center sm:justify-between">
              <div>
                Page {pagination.page} of {pagination.total_pages}
              </div>
              <div className="flex gap-2">
                {pagination.has_previous ? (
                  <Button asChild size="sm" variant="outline">
                    <Link href={packagesHref({
                      q: query,
                      version,
                      page: Math.max(1, pagination.page - 1),
                      pageSize: pagination.page_size
                    })}>Previous</Link>
                  </Button>
                ) : (
                  <Button disabled size="sm" variant="outline">Previous</Button>
                )}
                {pagination.has_next ? (
                  <Button asChild size="sm" variant="outline">
                    <Link href={packagesHref({
                      q: query,
                      version,
                      page: pagination.page + 1,
                      pageSize: pagination.page_size
                    })}>Next</Link>
                  </Button>
                ) : (
                  <Button disabled size="sm" variant="outline">Next</Button>
                )}
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </section>
  );
}
