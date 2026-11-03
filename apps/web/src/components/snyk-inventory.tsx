"use client";

import React, { useState } from "react";
import type { SnykInventory } from "@/lib/snyk";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export function SnykInventoryView({ sources }: { sources: SnykInventory["sources"] }) {
  return <>{sources.map((source) => <SnykSourceCard key={source.id} source={source} />)}</>;
}

function SnykSourceCard({ source }: { source: SnykInventory["sources"][number] }) {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const snapshot = source.snapshot;
  const projects = snapshot?.projects.filter((project) =>
    `${project.name} ${project.type} ${project.target_file} ${project.branch}`.toLowerCase().includes(search.toLowerCase())) ?? [];
  const artifacts = snapshot?.artifacts.filter((artifact) =>
    `${artifact.namespace}/${artifact.name}@${artifact.version} ${artifact.type} ${artifact.manifestFiles.map((ref) => ref.file).join(" ")}`.toLowerCase().includes(search.toLowerCase())) ?? [];
  const total = Math.max(projects.length, artifacts.length);
  return (
    <Card>
      <CardHeader>
        <CardTitle>Snyk inventory</CardTitle>
        <p className="text-sm text-slate-600">
          {source.synced_at ? `Last imported ${new Date(source.synced_at).toLocaleString()}` : "Not imported yet"}
          {!source.available && " · No longer available in Snyk"}
        </p>
        {source.error && <p role="alert" className="text-sm text-amber-800">{source.error}</p>}
        {snapshot && <p className="text-sm text-slate-600">{snapshot.projects.length} projects · {snapshot.files.length} manifest files · {snapshot.artifacts.length} dependencies</p>}
        {snapshot?.metadata.incomplete && <p className="text-sm text-amber-800">Some projects do not provide dependency graphs. Dependency totals are incomplete.</p>}
      </CardHeader>
      {snapshot && (
        <CardContent className="space-y-5">
          <Input aria-label="Filter Snyk inventory" placeholder="Filter projects and dependencies" value={search} onChange={(event) => { setSearch(event.target.value); setPage(0); }} />
          <div className="flex items-center justify-between text-sm">
            <span>Page {page + 1} of {Math.max(1, Math.ceil(total / 50))}</span>
            <div className="flex gap-2">
              <Button variant="outline" disabled={page === 0} onClick={() => setPage(page - 1)}>Previous Snyk results</Button>
              <Button variant="outline" disabled={(page + 1) * 50 >= total} onClick={() => setPage(page + 1)}>Next Snyk results</Button>
            </div>
          </div>
          <div>
            <h3 className="mb-2 font-semibold">Projects and manifest files</h3>
            <div className="divide-y rounded border">
              {projects.slice(page * 50, (page + 1) * 50).map((project) => (
                <div key={project.id} className="space-y-1 break-all p-3 text-sm">
                  <a className="font-medium underline" href={project.url} target="_blank" rel="noreferrer">{project.name} ↗</a>
                  <p>{project.target_file || "No manifest path provided"}</p>
                  <p className="text-slate-500">{project.type} · Branch {project.branch || "unknown"}</p>
                </div>
              ))}
              {projects.length === 0 && <p className="p-3 text-sm text-slate-500">No projects match.</p>}
            </div>
          </div>
          <div>
            <h3 className="mb-2 font-semibold">Dependencies and references</h3>
            <div className="divide-y rounded border">
              {artifacts.slice(page * 50, (page + 1) * 50).map((artifact) => (
                <div key={artifact.id} className="space-y-1 break-all p-3 text-sm">
                  <p className="font-medium">{artifact.namespace ? `${artifact.namespace}/` : ""}{artifact.name}@{artifact.version}</p>
                  <p className="text-slate-500">{artifact.type}{artifact.direct !== null && ` · ${artifact.direct ? "Direct" : "Transitive"}`}</p>
                  {artifact.manifestFiles.map((ref, index) => <p key={`${ref.file}:${index}`} className="text-slate-600">{ref.file}</p>)}
                </div>
              ))}
              {artifacts.length === 0 && <p className="p-3 text-sm text-slate-500">No dependencies match.</p>}
            </div>
          </div>
        </CardContent>
      )}
    </Card>
  );
}
