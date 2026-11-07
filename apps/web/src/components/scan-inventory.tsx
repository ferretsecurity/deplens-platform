"use client";

import { FilePath } from "@/components/file-path";
import React, { useState } from "react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ListPagination } from "@/components/list-pagination";
import type { ScanManifestItem } from "@/lib/types";

type ManifestView = { kind: "collapsed" } | { kind: "expanded"; page: number };
const dependenciesPerPage = 100;

function ManifestEntry({ manifest }: { manifest: ScanManifestItem }) {
  const [view, setView] = useState<ManifestView>({ kind: "collapsed" });

  return (
    <details
      className="rounded-xl border border-slate-200"
      onToggle={(event) => {
        if (event.currentTarget.open) {
          setView((current) => current.kind === "expanded" ? current : { kind: "expanded", page: 0 });
        } else {
          setView({ kind: "collapsed" });
        }
      }}
    >
      <summary className="cursor-pointer px-4 py-3 text-sm">
        <span className="[overflow-wrap:anywhere] font-semibold text-slate-950"><FilePath path={manifest.path} /></span>
        <span className="ml-3 text-slate-500">{manifest.type} · {manifest.dependencies.length} dependencies</span>
      </summary>
      {view.kind === "expanded" && (
        <div className="px-4 pb-3">
          <p className="mb-3 text-sm text-slate-600">{manifest.warnings.length > 0 ? manifest.warnings.join(" · ") : "No warnings"}</p>
          {manifest.dependencies.length === 0 ? <p className="text-sm text-slate-600">No dependencies recorded.</p> : (
            <>
              <ListPagination page={view.page} pageSize={dependenciesPerPage} total={manifest.dependencies.length} onPageChange={(page) => setView({ kind: "expanded", page })} />
              <Table layout="reflow" aria-label="Manifest dependencies">
                <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Version</TableHead><TableHead>Constraint</TableHead><TableHead>Source</TableHead></TableRow></TableHeader>
                <TableBody>
                  {manifest.dependencies.slice(view.page * dependenciesPerPage, (view.page + 1) * dependenciesPerPage).map((dependency) => (
                    <TableRow key={dependency.id}>
                      <TableCell mobileLabel="Name" className="table-cell-wide font-medium">{dependency.name || dependency.raw}</TableCell>
                      <TableCell mobileLabel="Version">{dependency.version || "n/a"}</TableCell>
                      <TableCell mobileLabel="Constraint">{dependency.constraint || "n/a"}</TableCell>
                      <TableCell mobileLabel="Source">{dependency.source || "n/a"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </>
          )}
        </div>
      )}
    </details>
  );
}

export function ScanInventory({ manifests }: { manifests: ScanManifestItem[] }) {
  return (
    <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
      <CardHeader>
        <CardTitle>Manifests</CardTitle>
        <CardDescription>Open a manifest to browse its dependencies, 100 at a time.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {manifests.length === 0 ? (
          <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">No manifests found for this scan.</div>
        ) : manifests.map((manifest) => <ManifestEntry key={manifest.id} manifest={manifest} />)}
      </CardContent>
    </Card>
  );
}
