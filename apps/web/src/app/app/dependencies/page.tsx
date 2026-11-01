import React from "react";

import { PagePagination } from "@/components/page-pagination";
import { listDependenciesServer } from "@/lib/queries";
import type { DependencyListItem } from "@/lib/types";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

function dependencyDisplayName(dependency: DependencyListItem) {
  if (dependency.name && dependency.version) {
    return `${dependency.name}@${dependency.version}`;
  }
  if (dependency.name && dependency.constraint) {
    return `${dependency.name} ${dependency.constraint}`;
  }
  return dependency.raw || "Unknown dependency";
}

export default async function DependenciesPage({ searchParams = Promise.resolve({}) }: {
  searchParams?: Promise<{ page?: string }>;
} = {}) {
  const [dependencies, params] = await Promise.all([listDependenciesServer(), searchParams]);
  const pageSize = 100;
  const requestedPage = Number(params.page ?? "1");
  const lastPage = Math.max(0, Math.ceil(dependencies.items.length / pageSize) - 1);
  const page = Number.isSafeInteger(requestedPage) && requestedPage > 0 ? Math.min(requestedPage - 1, lastPage) : 0;

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>Dependencies</CardTitle>
          <CardDescription>
            Active dependency usage across repositories and manifest files.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {dependencies.items.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              No active dependencies found.
            </div>
          ) : (
            <>
              <PagePagination page={page} pageSize={pageSize} total={dependencies.items.length} />
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Dependency</TableHead>
                    <TableHead className="text-right">Repositories</TableHead>
                    <TableHead className="text-right">Manifest files</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {dependencies.items.slice(page * pageSize, (page + 1) * pageSize).map((dependency, index) => (
                    <TableRow
                      key={`${dependency.name}:${dependency.version}:${dependency.raw}:${index}`}
                    >
                      <TableCell className="font-medium">{dependencyDisplayName(dependency)}</TableCell>
                      <TableCell className="text-right">{dependency.repository_count}</TableCell>
                      <TableCell className="text-right">{dependency.manifest_file_count}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </>
          )}
        </CardContent>
      </Card>
    </section>
  );
}
