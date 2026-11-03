import { z } from "zod";
import { inventoryRepositorySchema, socketConfigurationSchema, socketInventorySchema } from "./socket";

export const snykConfigurationSchema = socketConfigurationSchema.extend({
  region: z.enum(["us", "us2", "eu", "au"]),
  authentication: z.enum(["token", "bearer"]).default("token")
});

const importedSnapshotSchema = socketInventorySchema.shape.sources.element.shape.snapshot.unwrap();

export const snykInventorySchema = z.object({
  repository: inventoryRepositorySchema,
  sources: z.array(socketInventorySchema.shape.sources.element.extend({
    snapshot: importedSnapshotSchema.extend({
      metadata: importedSnapshotSchema.shape.metadata.extend({
        incomplete: z.boolean()
      }),
      projects: z.array(z.object({
        id: z.string(), name: z.string(), type: z.string(), target_file: z.string(),
        branch: z.string(), url: z.string()
      }))
    }).nullable()
  }))
});

export type SnykInventory = z.infer<typeof snykInventorySchema>;

export async function snykRequest(path = "", init?: RequestInit) {
  const response = await fetch(`/api/v1/integrations/snyk${path}`, {
    ...init,
    credentials: "include",
    headers: { "Content-Type": "application/json" }
  });
  if (!response.ok) {
    throw new Error(response.status === 403
      ? "Administrator access is required."
      : response.status === 409
        ? "A sync is active or the organization changed. Wait for the sync or disconnect first."
        : "Snyk request failed. Check the organization ID, region, token permissions and deployment settings.");
  }
  const result: unknown = await response.json();
  return result;
}
