import Link from "next/link";

import { getScanReportServer, getScanServer, listScanManifestsServer, listRepositoryOptionsServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ScannerReportCard } from "@/components/scanner-report-card";
import { Separator } from "@/components/ui/separator";
import { ScanInventory } from "@/components/scan-inventory";

export default async function ScanDetailPage({
  params,
  searchParams
}: {
  searchParams?: Promise<{ from?: string; to?: string }>;
  params: Promise<{
    scanId: string;
  }>;
}) {
  const { scanId } = await params;
  const dates = (await searchParams) ?? {};
  const returnParams = new URLSearchParams();
  for (const [key, value] of Object.entries(dates)) {
    if ((key === "from" || key === "to") && value && /^\d{4}-\d{2}-\d{2}$/.test(value)) returnParams.set(key, value);
  }
  const [scan, manifests, report] = await Promise.all([
    getScanServer(scanId),
    listScanManifestsServer(scanId),
    getScanReportServer(scanId)
  ]);

  returnParams.set("repository_id", scan.repository_id);
  const repositories = await listRepositoryOptionsServer();
  const repository = repositories.find((item) => item.id === scan.repository_id);

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <div>
          <div className="text-sm text-slate-500 [overflow-wrap:anywhere]">
            <Link className="underline-offset-4 hover:underline" href={`/app/scans?${returnParams}`}>
              Scans
            </Link>
            <span className="mx-2">/</span>
            {scan.id}
          </div>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Scan details</h1>
        </div>
      </div>

      <div className="rounded-xl border border-slate-200 bg-white/80 p-4 text-sm">
        <Link className="font-semibold text-slate-950 underline-offset-4 hover:underline [overflow-wrap:anywhere]" href={`/app/repositories/${scan.repository_id}/manifests`}>
          {repository?.name ?? scan.repository_id}
        </Link>
        <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-4">
          <div className="col-span-2 min-w-0 sm:col-span-1"><dt className="text-slate-500">Commit</dt><dd className="break-all font-mono">{scan.commit_sha}</dd></div>
          <div><dt className="text-slate-500">Manifests</dt><dd className="font-medium">{scan.manifest_count}</dd></div>
          <div><dt className="text-slate-500">Dependencies</dt><dd className="font-medium">{scan.dependency_count}</dd></div>
          <div className="col-span-2 sm:col-span-1"><dt className="text-slate-500">Scanned at</dt><dd><time dateTime={scan.scanned_at} title={new Date(scan.scanned_at).toISOString()}>{new Date(scan.scanned_at).toLocaleString("en-GB")}</time></dd></div>
        </dl>
      </div>

      <ScanInventory manifests={manifests.items} />

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

    </section>
  );
}
