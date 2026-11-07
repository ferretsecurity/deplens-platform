import Link from "next/link";
import { redirect } from "next/navigation";

import { listRepositoryOptionsServer, listScansServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

function formatDateInput(date: Date) {
  return date.toISOString().slice(0, 10);
}

function startOfDayUtc(dateString: string) {
  return `${dateString}T00:00:00Z`;
}

function endOfDayUtc(dateString: string) {
  return `${dateString}T23:59:59Z`;
}

export default async function ScansPage({
  searchParams
}: {
  searchParams?: Promise<{
    repository_id?: string;
    from?: string;
    to?: string;
  }>;
}) {
  const params = (await searchParams) ?? {};
  const repositories = await listRepositoryOptionsServer();

  if (repositories.length === 0) {
    return (
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <h1 className="text-2xl font-semibold leading-none tracking-tight">Scans</h1>
          <CardDescription>No repositories have been uploaded yet.</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const repositoryId = params.repository_id ?? repositories[0].id;
  const fromDate = params.from ?? formatDateInput(new Date(Date.now() - 30 * 24 * 60 * 60 * 1000));
  const toDate = params.to ?? formatDateInput(new Date());

  if (!params.repository_id) {
    redirect(`/app/scans?repository_id=${repositoryId}&from=${fromDate}&to=${toDate}`);
  }

  const scans = await listScansServer(repositoryId, startOfDayUtc(fromDate), endOfDayUtc(toDate));
  const activeRepository = repositories.find((repository) => repository.id === repositoryId) ?? repositories[0];

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <h1 className="text-2xl font-semibold leading-none tracking-tight">Scans</h1>
          <CardDescription>
            Inspect scan runs for {activeRepository.name}. Filter by repository and date window.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid grid-cols-2 gap-3 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)_minmax(0,0.9fr)_auto] md:items-end" method="get">
            <div className="col-span-2 grid min-w-0 gap-2 xl:col-span-1">
              <Label htmlFor="repository_id">Repository</Label>
              <select
                className="h-10 w-full min-w-0 max-w-full rounded-md border border-input bg-background px-3 text-sm"
                id="repository_id"
                name="repository_id"
                defaultValue={repositoryId}
              >
                {repositories.map((repository) => (
                  <option key={repository.id} value={repository.id}>
                    {repository.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="grid min-w-0 gap-2">
              <Label htmlFor="from">From</Label>
              <Input className="w-full min-w-0 max-w-full" id="from" name="from" type="date" defaultValue={fromDate} />
            </div>
            <div className="grid min-w-0 gap-2">
              <Label htmlFor="to">To</Label>
              <Input className="w-full min-w-0 max-w-full" id="to" name="to" type="date" defaultValue={toDate} />
            </div>
            <Button className="col-span-2 xl:col-span-1" type="submit">Apply</Button>
          </form>
        </CardContent>
      </Card>

      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardContent className="pt-6">
          {scans.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
              No scans found for this window.
            </div>
          ) : (
            <Table layout="reflow" aria-label="Scans">
              <TableHeader>
                <TableRow>
                  <TableHead>Commit</TableHead>
                  <TableHead>Scanned at</TableHead>
                  <TableHead>Manifests</TableHead>
                  <TableHead>Dependencies</TableHead>
                  <TableHead className="text-right">Open</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {scans.map((scan) => (
                  <TableRow key={scan.id}>
                    <TableCell mobileLabel="Commit" className="table-cell-wide table-cell-identity font-medium">{scan.commit_sha}</TableCell>
                    <TableCell mobileLabel="Scanned at" className="table-cell-wide"><time dateTime={scan.scanned_at} title={new Date(scan.scanned_at).toISOString()}>{new Date(scan.scanned_at).toLocaleDateString("en-GB")} <span className="inline-block whitespace-nowrap">{new Date(scan.scanned_at).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" })}</span></time></TableCell>
                    <TableCell mobileLabel="Manifests">{scan.manifest_count}</TableCell>
                    <TableCell mobileLabel="Dependencies">{scan.dependency_count}</TableCell>
                    <TableCell mobileLabel="Open" className="table-cell-wide text-right">
                      <Link
                        className="font-medium text-slate-900 underline-offset-4 hover:underline"
                        aria-label={`Details for ${scan.commit_sha}`}
                        href={`/app/scans/${scan.id}`}
                      >
                        Details
                      </Link>
                    </TableCell>
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
