import React from "react";
import Link from "next/link";

import { listRepositoriesServer } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const DEFAULT_PAGE_SIZE = 25;

function firstParam(value: string | string[] | undefined) {
  return Array.isArray(value) ? value[0] : value;
}

function parsePositiveInt(value: string | string[] | undefined, fallback: number) {
  const parsed = Number.parseInt(firstParam(value) ?? "", 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function repositoriesHref(params: { q: string; page: number; pageSize: number }) {
  const searchParams = new URLSearchParams();
  if (params.q) {
    searchParams.set("q", params.q);
  }
  if (params.page > 1) {
    searchParams.set("page", String(params.page));
  }
  if (params.pageSize !== DEFAULT_PAGE_SIZE) {
    searchParams.set("page_size", String(params.pageSize));
  }
  const query = searchParams.toString();
  return query ? `/app/repositories?${query}` : "/app/repositories";
}

export default async function RepositoriesPage({
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
  const page = parsePositiveInt(params.page, 1);
  const pageSize = parsePositiveInt(params.page_size, DEFAULT_PAGE_SIZE);
  const response = await listRepositoriesServer({
    q: query,
    page,
    page_size: pageSize
  });
  const { items: repositories, pagination } = response;
  const firstResult = repositories.length === 0 ? 0 : (pagination.page - 1) * pagination.page_size + 1;
  const lastResult = firstResult + repositories.length - 1;
  const previousHref = repositoriesHref({
    q: query,
    page: Math.max(1, pagination.page - 1),
    pageSize: pagination.page_size
  });
  const nextHref = repositoriesHref({
    q: query,
    page: pagination.page + 1,
    pageSize: pagination.page_size
  });

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>Repositories</CardTitle>
          <CardDescription>Browse repositories that have uploaded scan snapshots.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <form className="flex flex-col gap-3 sm:flex-row" method="get">
            <div className="grid flex-1 gap-2">
              <Label className="sr-only" htmlFor="q">
                Search repositories
              </Label>
              <Input
                id="q"
                name="q"
                placeholder="Search by name or URL"
                type="search"
                defaultValue={query}
              />
            </div>
            <input name="page_size" type="hidden" value={pagination.page_size} />
            <div className="flex gap-2">
              <Button type="submit">Search</Button>
              {query ? (
                <Button asChild variant="outline">
                  <Link href="/app/repositories">Clear</Link>
                </Button>
              ) : null}
            </div>
          </form>

          {repositories.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              {query ? "No repositories match this search." : "No repositories have been uploaded yet."}
            </div>
          ) : (
            <>
              <div className="text-sm text-slate-600">
                Showing {firstResult}-{lastResult} of {pagination.total} repositories
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>URL</TableHead>
                    <TableHead>Default branch</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {repositories.map((repository) => (
                    <TableRow key={repository.id}>
                      <TableCell className="font-medium">{repository.name}</TableCell>
                      <TableCell className="max-w-[380px] truncate">
                        <a
                          className="text-slate-600 underline-offset-4 hover:text-slate-900 hover:underline"
                          href={repository.url}
                          rel="noreferrer"
                          target="_blank"
                        >
                          {repository.url}
                        </a>
                      </TableCell>
                      <TableCell>{repository.default_branch}</TableCell>
                      <TableCell className="text-right">
                        <div className="flex flex-col items-end gap-2 sm:flex-row sm:justify-end">
                          <Link
                            className="font-medium text-slate-900 underline-offset-4 hover:underline"
                            href={`/app/repositories/${repository.id}/manifests`}
                          >
                            Manifest files
                          </Link>
                          <Link
                            className="font-medium text-slate-900 underline-offset-4 hover:underline"
                            href={`/app/scans?repository_id=${repository.id}`}
                          >
                            Open scans
                          </Link>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <div className="flex flex-col gap-3 border-t border-slate-200 pt-4 text-sm text-slate-600 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  Page {pagination.page} of {pagination.total_pages}
                </div>
                <div className="flex gap-2">
                  {pagination.has_previous ? (
                    <Button asChild size="sm" variant="outline">
                      <Link href={previousHref}>Previous</Link>
                    </Button>
                  ) : (
                    <Button disabled size="sm" variant="outline">
                      Previous
                    </Button>
                  )}
                  {pagination.has_next ? (
                    <Button asChild size="sm" variant="outline">
                      <Link href={nextHref}>Next</Link>
                    </Button>
                  ) : (
                    <Button disabled size="sm" variant="outline">
                      Next
                    </Button>
                  )}
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </section>
  );
}
