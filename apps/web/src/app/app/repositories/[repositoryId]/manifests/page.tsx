import React from "react";
import { SocketInventoryView } from "@/components/socket-inventory";
import { RepositorySources } from "@/components/repository-sources";
import { ManifestComparison } from "@/components/manifest-comparison";
import { fetchCurrentUserServer } from "@/lib/auth.server";
import Link from "next/link";

import { getRepositoryInventoryServer, listRepositoryManifestsServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default async function RepositoryManifestsPage({
  params
}: {
  params: Promise<{
    repositoryId: string;
  }>;
}) {
  const { repositoryId } = await params;
  const [inventory, manifests, user] = await Promise.all([
    getRepositoryInventoryServer(repositoryId),
    listRepositoryManifestsServer(repositoryId),
    fetchCurrentUserServer()
  ]);
  const repository = inventory.repository;

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
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">
          Manifest files
        </h1>
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
          <div className="mb-4">
            <RepositorySources repository={repository} />
          </div>
          <ManifestComparison manifests={manifests.items} sources={inventory.sources} />
        </CardContent>
      </Card>
      <SocketInventoryView
        sources={inventory.sources}
        admin={user.role === "owner" || user.role === "admin"}
      />
    </section>
  );
}
