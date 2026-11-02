"use client";

import React, { useState } from "react";
import type { RepositoryManifestItem } from "@/lib/types";
import type { SocketInventory } from "@/lib/socket";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const pageSize = 50;

export function ManifestComparison({ manifests, sources }: {
  manifests: RepositoryManifestItem[];
  sources: SocketInventory["sources"];
}) {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const deplensFiles = new Map(manifests.map((manifest) => [manifest.path, manifest]));
  const socketFiles = new Set(sources.flatMap((source) => source.snapshot?.files ?? []));
  const availableSocketFiles = new Set(sources.filter((source) => source.available)
    .flatMap((source) => source.snapshot?.files ?? []));
  const paths = [...new Set([...deplensFiles.keys(), ...socketFiles])]
    .sort()
    .filter((path) => path.toLowerCase().includes(search.toLowerCase()));
  const pageCount = Math.max(1, Math.ceil(paths.length / pageSize));
  const socketMissing = sources.length === 0
    ? "No Socket source linked"
    : sources.every((source) => source.snapshot === null)
      ? "Not imported yet"
      : "Not found in imported files";

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">Deplens vs Socket</h2>
      <p className="text-sm text-slate-600">
        Compare file paths from Deplens scans and the latest Socket imports. Each source may have scanned a different commit.
      </p>
      {manifests.length === 0 && (
        <p className="text-sm text-slate-600">No CLI manifest files found for this repository.</p>
      )}
      {sources.some((source) => !source.available || source.error || source.snapshot === null) && (
        <p className="text-sm text-amber-800">
          Some Socket sources are unavailable, have import errors, or have not been imported yet. See Socket inventory below for details.
        </p>
      )}
      <Input
        aria-label="Filter manifest paths"
        placeholder="Filter manifest paths"
        value={search}
        onChange={(event) => { setSearch(event.target.value); setPage(0); }}
      />
      <div className="overflow-x-auto">
        <Table aria-label="Manifest file comparison" className="min-w-[600px] table-fixed">
          <TableHeader>
            <TableRow>
              <TableHead className="w-1/2">Path</TableHead>
              <TableHead>Deplens</TableHead>
              <TableHead>Socket</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {paths.slice(page * pageSize, (page + 1) * pageSize).map((path) => {
              const manifest = deplensFiles.get(path);
              return (
                <TableRow key={path}>
                  <TableCell className="break-all align-top font-medium">{path}</TableCell>
                  <TableCell className="align-top">
                    {manifest ? (
                      <div className="space-y-1">
                        <p className={manifest.is_active ? "text-slate-900" : "text-slate-500"}>
                          {manifest.is_active ? "Active" : "Disappeared"}
                        </p>
                        <p className="text-xs text-slate-500">First seen: {new Date(manifest.first_seen_at).toLocaleString()}</p>
                        <p className="text-xs text-slate-500">Last seen: {new Date(manifest.last_seen_at).toLocaleString()}</p>
                      </div>
                    ) : <span className="text-slate-500">Not found</span>}
                  </TableCell>
                  <TableCell className="align-top">
                    {socketFiles.has(path)
                      ? availableSocketFiles.has(path) ? "Detected" : "Detected in unavailable source"
                      : <span className="text-slate-500">{socketMissing}</span>}
                  </TableCell>
                </TableRow>
              );
            })}
            {paths.length === 0 && (
              <TableRow><TableCell colSpan={3} className="text-slate-500">
                {search ? "No manifest paths match." : "No manifest files to compare."}
              </TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      {paths.length > pageSize && (
        <div className="flex items-center justify-between gap-2 text-sm">
          <span>{paths.length} files · Page {page + 1} of {pageCount}</span>
          <div className="flex gap-2">
            <Button variant="outline" disabled={page === 0} onClick={() => setPage(page - 1)}>Previous files</Button>
            <Button variant="outline" disabled={page + 1 >= pageCount} onClick={() => setPage(page + 1)}>Next files</Button>
          </div>
        </div>
      )}
    </div>
  );
}
