import { z } from "zod";

export const packageUsageSchema = z.object({
  source: z.enum(["cli", "socket", "snyk"]),
  name: z.string().min(1),
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
  filters: z.object({ q: z.string(), version: z.string() })
});

export type PackageUsage = z.infer<typeof packageUsageSchema>;
export type PackageListResponse = z.infer<typeof packageListResponseSchema>;
