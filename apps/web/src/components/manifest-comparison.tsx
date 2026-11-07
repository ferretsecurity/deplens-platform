"use client";

import { FilePath } from "@/components/file-path";
import React, { useState } from "react";
import type { RepositoryManifestItem } from "@/lib/types";
import type { SnykInventory } from "@/lib/snyk";
import type { SocketInventory } from "@/lib/socket";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const pageSize = 50;

export function ManifestComparison({ manifests, sources, snykSources = [] }: {
  manifests: RepositoryManifestItem[];
  sources: SocketInventory["sources"];
  snykSources?: SnykInventory["sources"];
}) {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const deplensFiles = new Map(manifests.map((manifest) => [manifest.path, manifest]));
  const providers = [
    { name: "Socket", sources },
    { name: "Snyk", sources: snykSources }
  ].map((provider) => ({
    ...provider,
    files: new Set(provider.sources.flatMap((source) => source.snapshot?.files ?? [])),
    availableFiles: new Set(provider.sources.filter((source) => source.available)
      .flatMap((source) => source.snapshot?.files ?? [])),
    missing: provider.sources.length === 0
      ? `No ${provider.name} source linked`
      : provider.sources.every((source) => source.snapshot === null)
        ? "Not imported yet"
        : provider.name === "Snyk" && snykSources.some((source) => source.snapshot?.projects.some((project) => !project.target_file))
          ? "Manifest path unknown for some projects"
          : "Not found in imported files"
  }));
  const paths = [...new Set([...deplensFiles.keys(), ...providers.flatMap((provider) => [...provider.files])])]
    .sort()
    .filter((path) => path.toLowerCase().includes(search.toLowerCase()));
  const pageCount = Math.max(1, Math.ceil(paths.length / pageSize));

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">Deplens vs Socket vs Snyk</h2>
      <p className="text-sm text-slate-600">
        Compare file paths from Deplens scans and the latest Socket and Snyk imports. Each source may have scanned a different commit.
      </p>
      {manifests.length === 0 && (
        <p className="text-sm text-slate-600">No CLI manifest files found for this repository.</p>
      )}
      {providers.filter((provider) => provider.sources.some((source) => !source.available || source.error || source.snapshot === null)).map((provider) => (
        <p key={provider.name} className="text-sm text-amber-800">
          Some {provider.name} sources are unavailable, have import errors, or have not been imported yet. See {provider.name} inventory below for details.
        </p>
      ))}
      {snykSources.some((source) => source.snapshot?.metadata.incomplete) && (
        <p className="text-sm text-amber-800">Snyk includes projects without dependency graphs. Their manifest paths are shown, but their dependency inventory is incomplete.</p>
      )}
      {snykSources.some((source) => source.snapshot?.projects.some((project) => !project.target_file)) && (
        <p className="text-sm text-amber-800">Some Snyk projects do not report a manifest path. Their file coverage is unknown.</p>
      )}
      <Input
        aria-label="Filter manifest paths"
        placeholder="Filter manifest paths"
        value={search}
        onChange={(event) => { setSearch(event.target.value); setPage(0); }}
      />
      <div>
        <Table layout="reflow" aria-label="Manifest file comparison">
          <TableHeader>
            <TableRow>
              <TableHead className="w-[40%]">Path</TableHead>
              <TableHead>Deplens</TableHead>
              {providers.map((provider) => <TableHead key={provider.name}>{provider.name}</TableHead>)}
            </TableRow>
          </TableHeader>
          <TableBody>
            {paths.slice(page * pageSize, (page + 1) * pageSize).map((path) => {
              const manifest = deplensFiles.get(path);
              return (
                <TableRow key={path}>
                  <TableCell mobileLabel="Path" className="table-cell-wide align-top font-medium"><FilePath path={path} /></TableCell>
                  <TableCell mobileLabel="Deplens" className="table-cell-wide align-top">
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
                  {providers.map((provider) => (
                    <TableCell key={provider.name} mobileLabel={provider.name} className="table-cell-wide align-top">
                      {provider.files.has(path)
                        ? provider.availableFiles.has(path) ? "Detected" : "Detected in unavailable source"
                        : <span className="text-slate-500">{provider.missing}</span>}
                    </TableCell>
                  ))}
                </TableRow>
              );
            })}
            {paths.length === 0 && (
              <TableRow><TableCell colSpan={4} className="text-slate-500">
                {search ? "No manifest paths match." : "No manifest files to compare."}
              </TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      {paths.length > pageSize && (
        <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
          <span>{paths.length} files · Page {page + 1} of {pageCount}</span>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" disabled={page === 0} onClick={() => setPage(page - 1)}>Previous files</Button>
            <Button variant="outline" disabled={page + 1 >= pageCount} onClick={() => setPage(page + 1)}>Next files</Button>
          </div>
        </div>
      )}
    </div>
  );
}
