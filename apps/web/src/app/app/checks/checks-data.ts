import type { getScanReportServer, getScanServer, listRepositoryOptionsServer } from "@/lib/queries";

type Scan = Awaited<ReturnType<typeof getScanServer>>;

export type ProjectChecks = {
  repository: Awaited<ReturnType<typeof listRepositoryOptionsServer>>[number];
  scans: Scan[];
  selected: { scan: Scan; report: Awaited<ReturnType<typeof getScanReportServer>> } | null;
};
