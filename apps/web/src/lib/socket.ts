import { z } from "zod";

export const referencePositionSchema = z
  .union([
    z.number().int(),
    z.object({ line: z.number().int(), column: z.number().int().optional() })
  ])
  .nullish();

export const socketRunSchema = z.object({
  id: z.string(),
  status: z.enum(["queued", "running", "completed", "partial", "failed", "cancelled"]),
  started_at: z.string(),
  finished_at: z.string().nullable(),
  total: z.number(),
  completed: z.number(),
  failed: z.number(),
  message: z.string()
});
export const socketConfigurationSchema = z.object({
  enabled: z.boolean(),
  organization: z.string(),
  token_configured: z.boolean(),
  interval_minutes: z.number(),
  next_run_at: z.string().nullable(),
  last_run: socketRunSchema.nullable()
});
export const inventoryRepositorySchema = z.object({
  id: z.string(),
  name: z.string(),
  url: z.string(),
  default_branch: z.string(),
  cli: z.boolean(),
  socket: z.boolean(),
  socket_url: z.string(),
  snyk: z.boolean().optional(),
  snyk_url: z.string().optional()
});
export const socketInventorySchema = z.object({
  repository: inventoryRepositorySchema,
  sources: z.array(
    z.object({
      id: z.string(),
      available: z.boolean(),
      error: z.string(),
      synced_at: z.string().nullable(),
      snapshot: z
        .object({
          files: z.array(z.string()),
          metadata: z.object({
            branch: z.string(),
            commit_hash: z.string(),
            integration_repo_url: z.string(),
            html_report_url: z.string()
          }),
          artifacts: z.array(
            z.object({
              id: z.string(),
              type: z.string(),
              name: z.string(),
              namespace: z.string(),
              version: z.string(),
              direct: z.boolean().nullable(),
              dev: z.boolean().nullable(),
              manifestFiles: z.array(
                z.object({
                  file: z.string(),
                  start: referencePositionSchema,
                  end: referencePositionSchema
                })
              )
            })
          )
        })
        .nullable()
    })
  )
});
export type SocketInventory = z.infer<typeof socketInventorySchema>;

export async function socketRequest(path = "", init?: RequestInit) {
  const response = await fetch(`/api/v1/integrations/socket${path}`, {
    ...init,
    credentials: "include",
    headers: { "Content-Type": "application/json" }
  });
  if (!response.ok) {
    const message =
      response.status === 403
        ? "Administrator access is required."
        : response.status === 409
          ? "A sync is already active. Wait for it to finish before saving."
          : "Socket request failed. Check the organization, token scopes and deployment settings.";
    throw new Error(message);
  }
  const result: unknown = await response.json();
  return result;
}
