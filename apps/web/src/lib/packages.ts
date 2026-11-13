import { z } from "zod";

export const packageUsageSchema = z.object({
  assessment: z.object({
    status: z.enum(["issue", "warning", "unknown"]),
    scope: z.enum(["project", "workspace", "repository"]),
    root: z.string(),
    reasons: z.array(z.string()),
    lockfiles: z.array(z.string()),
    manifests: z.array(z.string())
  }).optional(),
  source: z.enum(["cli", "socket", "snyk"]),
  name: z.string(),
  origin_kind: z.string(),
  destination: z.string(),
  version: z.string(),
  constraint: z.string(),
  ecosystem: z.string(),
  repository_id: z.string().uuid(),
  repository_name: z.string(),
  manifest_path: z.string().nullable()
});

export const packageListResponseSchema = z.object({
  items: z.array(packageUsageSchema),
  pagination: z.object({
    page: z.number().int().positive(),
    page_size: z.number().int().min(1).max(100),
    total: z.number().int().nonnegative(),
    total_pages: z.number().int().nonnegative(),
    has_previous: z.boolean(),
    has_next: z.boolean()
  }),
  filters: z.object({ q: z.string(), version: z.string(), ecosystem: z.string().optional(), affected_range: z.string().optional() })
});

export type PackageUsage = z.infer<typeof packageUsageSchema>;
export type PackageListResponse = z.infer<typeof packageListResponseSchema>;
