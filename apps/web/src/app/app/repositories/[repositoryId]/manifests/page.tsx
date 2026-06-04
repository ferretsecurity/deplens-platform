import React from "react";
import Link from "next/link";

import { listRepositoriesServer, listRepositoryManifestsServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

function formatDate(value: string) {
  return new Date(value).toLocaleString();
}

export default async function RepositoryManifestsPage({
  params
}: {
  params: Promise<{
    repositoryId: string;
  }>;
}) {
  const { repositoryId } = await params;
  const [repositories, manifests] = await Promise.all([
    listRepositoriesServer(),
    listRepositoryManifestsServer(repositoryId)
  ]);
  const repository = repositories.find((item) => item.id === repositoryId);

  return (
    <section className="space-y-6">
      <div>
        <div className="text-sm text-slate-500">
          <Link className="underline-offset-4 hover:underline" href="/app/repositories">
            Repositories
          </Link>
          <span className="mx-2">/</span>
          Manifest files
        </div>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Manifest files</h1>
      </div>

      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>{repository?.name ?? repositoryId}</CardTitle>
          <CardDescription>
            {repository ? (
              <a
                className="underline-offset-4 hover:text-slate-900 hover:underline"
                href={repository.url}
                rel="noreferrer"
                target="_blank"
              >
                {repository.url}
              </a>
            ) : (
              "Repository details are unavailable."
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {manifests.items.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              No manifest files found for this repository.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Path</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>First seen</TableHead>
                  <TableHead>Last seen</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {manifests.items.map((manifest) => (
                  <TableRow key={manifest.id}>
                    <TableCell className="font-medium">{manifest.path}</TableCell>
                    <TableCell className={manifest.is_active ? "text-slate-900" : "text-slate-500"}>
                      {manifest.is_active ? "Active" : "Disappeared"}
                    </TableCell>
                    <TableCell>{formatDate(manifest.first_seen_at)}</TableCell>
                    <TableCell>{formatDate(manifest.last_seen_at)}</TableCell>
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
