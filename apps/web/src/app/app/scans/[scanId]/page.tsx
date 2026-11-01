import Link from "next/link";

import { getScanReportServer, getScanServer, listScanManifestsServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ScannerReportCard } from "@/components/scanner-report-card";
import { Separator } from "@/components/ui/separator";
import { ScanInventory } from "@/components/scan-inventory";

export default async function ScanDetailPage({
  params
}: {
  params: Promise<{
    scanId: string;
  }>;
}) {
  const { scanId } = await params;
  const [scan, manifests, report] = await Promise.all([
    getScanServer(scanId),
    listScanManifestsServer(scanId),
    getScanReportServer(scanId)
  ]);

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
              <CardTitle className={label === "Commit" ? "break-all font-mono text-sm leading-6" : "text-lg"}>{value}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      <ScannerReportCard reportState={report} scanId={scanId} />

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

      <ScanInventory manifests={manifests.items} />
    </section>
  );
}
