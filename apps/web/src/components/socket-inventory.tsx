"use client";

import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import type { SocketInventory } from "@/lib/socket";
import { inventoryRepositorySchema, referencePositionSchema, socketRequest } from "@/lib/socket";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

function referencePosition(position: z.infer<typeof referencePositionSchema>) {
  if (position == null) return "";
  if (typeof position === "number") return `Position ${position}`;
  return `Line ${position.line}${position.column === undefined ? "" : `, column ${position.column}`}`;
}

function RepositoryMapping({ sourceId }: { sourceId: string }) {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const candidates = useQuery({
    queryKey: ["socket-mapping-candidates", search],
    enabled: search.trim().length > 0,
    queryFn: async () => {
      const response = await fetch(
        `/api/v1/repositories?q=${encodeURIComponent(search)}&page_size=100`
      );
      if (!response.ok) throw new Error("Repository search failed");
      const result: unknown = await response.json();
      return z
        .object({ items: z.array(inventoryRepositorySchema) })
        .parse(result)
        .items.filter((repository) => repository.cli);
    }
  });
  async function save() {
    setBusy(true);
    setMessage("");
    try {
      await socketRequest(`/repositories/${sourceId}/mapping`, {
        method: "PUT",
        body: JSON.stringify({ cli_repository_id: selected || null })
      });
      window.location.href = selected
        ? `/app/repositories/${selected}/manifests`
        : `/app/repositories/${sourceId}/manifests`;
    } catch {
      setMessage("Could not save repository mapping.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <details className="rounded-lg border p-3 text-sm">
      <summary className="cursor-pointer font-medium">Manage CLI repository mapping</summary>
      <p className="my-3 text-slate-600">
        Link only when both sources describe the same SCM repository. The inventories remain
        separate.
      </p>
      <Input
        aria-label="Search CLI repositories"
        placeholder="Search CLI repositories"
        value={search}
        onChange={(event) => setSearch(event.target.value)}
      />
      <select
        aria-label="CLI repository mapping"
        className="my-3 w-full rounded border p-2"
        value={selected}
        onChange={(event) => setSelected(event.target.value)}
      >
        <option value="">Keep as a separate Socket repository</option>
        {candidates.data?.map((repository) => (
          <option key={repository.id} value={repository.id}>
            {repository.name} · {repository.url}
          </option>
        ))}
      </select>
      <Button disabled={busy} onClick={() => void save()} variant="outline">
        Save mapping
      </Button>
      {message && <p role="alert">{message}</p>}
    </details>
  );
}
export function SocketInventoryView({
  sources,
  admin = false
}: {
  sources: SocketInventory["sources"];
  admin?: boolean;
}) {
  return <>
    {sources.map((source) => <SocketSourceCard key={source.id} source={source} admin={admin} />)}
  </>;
}

function SocketSourceCard({ source, admin }: { source: SocketInventory["sources"][number]; admin: boolean }) {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const snapshot = source.snapshot;
  const filtered =
    snapshot?.artifacts.filter((artifact) =>
      `${artifact.namespace}/${artifact.name}@${artifact.version} ${artifact.type} ${artifact.manifestFiles.map((ref) => ref.file).join(" ")}`
        .toLowerCase()
        .includes(search.toLowerCase())
    ) ?? [];
  const files =
    snapshot?.files.filter((file) => file.toLowerCase().includes(search.toLowerCase())) ?? [];
  const total = Math.max(files.length, filtered.length);
  return (
    <Card key={source.id}>
      <CardHeader>
        <CardTitle>Socket inventory</CardTitle>
        <p className="text-sm text-slate-600">
          {source.synced_at
            ? `Last imported ${new Date(source.synced_at).toLocaleString()}`
            : "Not imported yet"}
          {!source.available && " · No longer available in Socket"}
        </p>
        {source.error && (
          <p role="alert" className="text-sm text-amber-800">
            {source.error}
          </p>
        )}
        {snapshot && (
          <p className="text-sm text-slate-600">
            {snapshot.files.length} source files · {snapshot.artifacts.length} dependencies ·
            Branch {snapshot.metadata.branch || "unknown"}
            {snapshot.metadata.commit_hash &&
              ` · Commit ${snapshot.metadata.commit_hash.slice(0, 12)}`}
          </p>
        )}
      </CardHeader>
      <CardContent className="space-y-5">
        {admin && <RepositoryMapping sourceId={source.id} />}
        {snapshot && (
          <>
            <Input
              aria-label="Filter Socket inventory"
              placeholder="Filter files and dependencies"
              value={search}
              onChange={(event) => {
                setSearch(event.target.value);
                setPage(0);
              }}
            />
            <div className="flex items-center justify-between text-sm">
              <span>
                Page {page + 1} of {Math.max(1, Math.ceil(total / 50))}
              </span>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  disabled={page === 0}
                  onClick={() => setPage(page - 1)}
                >
                  Previous results
                </Button>
                <Button
                  variant="outline"
                  disabled={(page + 1) * 50 >= total}
                  onClick={() => setPage(page + 1)}
                >
                  Next results
                </Button>
              </div>
            </div>
            <div>
              <h3 className="mb-2 font-semibold">Dependency source files</h3>
              <div className="divide-y rounded border">
                {files.slice(page * 50, (page + 1) * 50).map((file) => (
                  <div key={file} className="break-all p-3 text-sm">
                    {file}
                  </div>
                ))}
                {files.length === 0 && (
                  <p className="p-3 text-sm text-slate-500">No source files match.</p>
                )}
              </div>
            </div>
            <div>
              <h3 className="mb-2 font-semibold">Dependencies and references</h3>
              <div className="divide-y rounded border">
                {filtered.slice(page * 50, (page + 1) * 50).map((artifact) => (
                  <div key={artifact.id} className="space-y-2 p-3 text-sm">
                    <div className="break-all font-medium">
                      {artifact.namespace ? `${artifact.namespace}/` : ""}
                      {artifact.name}@{artifact.version}
                    </div>
                    <p className="text-xs text-slate-500">
                      {artifact.type}
                      {artifact.direct !== null &&
                        ` · ${artifact.direct ? "Direct" : "Transitive"}`}
                      {artifact.dev !== null &&
                        ` · ${artifact.dev ? "Development" : "Production"}`}
                    </p>
                    {artifact.manifestFiles.length ? (
                      artifact.manifestFiles.map((ref, index) => (
                        <p key={`${ref.file}:${index}`} className="break-all text-slate-600">
                          {ref.file}
                          {ref.start != null && ` · ${referencePosition(ref.start)}`}
                          {ref.end != null && ` to ${referencePosition(ref.end)}`}
                        </p>
                      ))
                    ) : (
                      <p className="text-slate-500">
                        Repository dependency · No source file reference provided by Socket
                      </p>
                    )}
                  </div>
                ))}
                {filtered.length === 0 && (
                  <p className="p-3 text-sm text-slate-500">No dependencies match.</p>
                )}
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
