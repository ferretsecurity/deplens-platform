import React from "react";
import Link from "next/link";

import { listPackagesServer } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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

function packagesHref(params: { q: string; page: number; pageSize: number }) {
  const searchParams = new URLSearchParams();
  if (params.q) searchParams.set("q", params.q);
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
    page?: string | string[];
    page_size?: string | string[];
  }>;
}) {
  const params = (await searchParams) ?? {};
  const query = (firstParam(params.q) ?? "").trim();
  const page = positiveInt(params.page, 1);
  const pageSize = Math.min(100, positiveInt(params.page_size, DEFAULT_PAGE_SIZE));
  const { items, pagination } = await listPackagesServer({ q: query, page, page_size: pageSize });
  const firstResult = items.length ? (pagination.page - 1) * pagination.page_size + 1 : 0;
  const lastResult = items.length ? firstResult + items.length - 1 : 0;

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>Packages</CardTitle>
          <CardDescription>
            Find package usages across repositories and manifest files from the CLI, Socket and Snyk.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <form className="flex flex-col gap-3 sm:flex-row" method="get" action="/app/packages">
            <div className="grid flex-1 gap-2">
              <Label className="sr-only" htmlFor="q">
                Search packages
              </Label>
              <Input
                id="q"
                name="q"
                type="search"
                placeholder="Search by package name"
                defaultValue={query}
                maxLength={200}
              />
            </div>
            <input name="page_size" type="hidden" value={pagination.page_size} />
            <div className="flex gap-2">
              <Button type="submit">Search</Button>
              {query ? (
                <Button asChild variant="outline">
                  <Link href="/app/packages">Clear</Link>
                </Button>
              ) : null}
            </div>
          </form>
          <div className="text-sm text-slate-600">
            Showing {firstResult}-{lastResult} of {pagination.total} package usages
          </div>
          {items.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              {pagination.total > 0
                ? "No package usages on this page."
                : query
                  ? "No packages match this search."
                  : "No active packages found."}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table className="min-w-[720px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>Package</TableHead>
                    <TableHead>Version / constraint</TableHead>
                    <TableHead>Ecosystem</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead>Repository</TableHead>
                    <TableHead>Manifest file</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((item) => (
                    <TableRow key={JSON.stringify(item)}>
                      <TableCell className="font-medium">{item.name}</TableCell>
                      <TableCell>
                        {[item.version, item.constraint].filter(Boolean).join(" / ") || "Unspecified"}
                      </TableCell>
                      <TableCell>{item.ecosystem || "Unspecified"}</TableCell>
                      <TableCell>{sourceLabels[item.source]}</TableCell>
                      <TableCell>
                        <Link
                          className="font-medium text-slate-900 underline-offset-4 hover:underline"
                          href={`/app/repositories/${item.repository_id}/manifests`}
                        >
                          {item.repository_name}
                        </Link>
                      </TableCell>
                      <TableCell className="break-all">{item.manifest_path ?? "Not reported"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
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
