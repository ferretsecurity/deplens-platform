import React from "react";
import Link from "next/link";

import { listRepositoriesServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export default async function RepositoriesPage() {
  const repositories = await listRepositoriesServer();

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>Repositories</CardTitle>
          <CardDescription>Browse repositories that have uploaded scan snapshots.</CardDescription>
        </CardHeader>
        <CardContent>
          {repositories.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              No repositories have been uploaded yet.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>URL</TableHead>
                  <TableHead>Default branch</TableHead>
                  <TableHead className="text-right">Scans</TableHead>
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
                      <Link
                        className="font-medium text-slate-900 underline-offset-4 hover:underline"
                        href={`/app/scans?repository_id=${repository.id}`}
                      >
                        Open scans
                      </Link>
                    </TableCell>
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
