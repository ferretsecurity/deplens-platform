import React from "react";

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

export default async function DependenciesPage() {
  const dependencies = await listDependenciesServer();

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
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Dependency</TableHead>
                  <TableHead className="text-right">Repositories</TableHead>
                  <TableHead className="text-right">Manifest files</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {dependencies.items.map((dependency, index) => (
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
          )}
        </CardContent>
      </Card>
    </section>
  );
}
