import Link from "next/link";

import { getScanServer, listScanManifestsServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from "@/components/ui/table";

export default async function ScanDetailPage({
  params
}: {
  params: Promise<{
    scanId: string;
  }>;
}) {
  const { scanId } = await params;
  const scan = await getScanServer(scanId);
  const manifests = await listScanManifestsServer(scanId);

  return (
    <section className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="text-sm text-slate-500">
            <Link className="underline-offset-4 hover:underline" href="/app/scans">
              Scans
            </Link>
            <span className="mx-2">/</span>
            {scan.id}
          </div>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Scan details</h1>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-4">
        {[
          ["Commit", scan.commit_sha],
          ["Manifests", String(scan.manifest_count)],
          ["Dependencies", String(scan.dependency_count)],
          ["Scanned at", new Date(scan.scanned_at).toLocaleString()]
        ].map(([label, value]) => (
          <Card key={label} className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
            <CardHeader className="pb-3">
              <CardDescription>{label}</CardDescription>
              <CardTitle className="text-lg">{value}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>Metadata</CardTitle>
          <CardDescription>Labels and notes captured with the scan upload.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2">
            <div>
              <div className="text-xs uppercase tracking-[0.18em] text-slate-500">Repository</div>
              <div className="mt-1 font-medium text-slate-900">{scan.repository_id}</div>
            </div>
            <div>
              <div className="text-xs uppercase tracking-[0.18em] text-slate-500">Annotation</div>
              <div className="mt-1 font-medium text-slate-900">{scan.annotation || "None"}</div>
            </div>
          </div>
          <Separator />
          <div className="space-y-2">
            <div className="text-xs uppercase tracking-[0.18em] text-slate-500">Labels</div>
            <div className="flex flex-wrap gap-2">
              {Object.entries(scan.labels ?? {}).length === 0 ? (
                <span className="text-sm text-slate-600">No labels recorded.</span>
              ) : (
                Object.entries(scan.labels).map(([key, value]) => (
                  <span
                    key={key}
                    className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-sm text-slate-700"
                  >
                    {key}: {value}
                  </span>
                ))
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>Manifests</CardTitle>
          <CardDescription>Nested manifest entries and dependency details.</CardDescription>
        </CardHeader>
        <CardContent>
          {manifests.items.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              No manifests found for this scan.
            </div>
          ) : (
            manifests.items.map((manifest) => (
              <div key={manifest.id} className="mb-5 last:mb-0">
                <div className="flex flex-wrap items-center gap-3">
                  <div className="text-base font-semibold text-slate-950">{manifest.path}</div>
                  <div className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium uppercase tracking-[0.18em] text-slate-600">
                    {manifest.type}
                  </div>
                </div>
                <div className="mt-2 text-sm text-slate-600">
                  {manifest.warnings.length > 0 ? manifest.warnings.join(" · ") : "No warnings"}
                </div>
                <div className="mt-4 overflow-hidden rounded-xl border border-slate-200">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Name</TableHead>
                        <TableHead>Version</TableHead>
                        <TableHead>Constraint</TableHead>
                        <TableHead>Source</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {manifest.dependencies.map((dependency) => (
                        <TableRow key={dependency.id}>
                          <TableCell className="font-medium">{dependency.name || dependency.raw}</TableCell>
                          <TableCell>{dependency.version || "n/a"}</TableCell>
                          <TableCell>{dependency.constraint || "n/a"}</TableCell>
                          <TableCell>{dependency.source || "n/a"}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </section>
  );
}
