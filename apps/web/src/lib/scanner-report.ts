import { z } from "zod";

const subjectSchema = z.object({ project_root: z.string() });

export const scannerReportResponseSchema = z.object({
  report: z.object({
    schema_version: z.number().int(),
    sources: z.array(z.object({
      detector: z.string(),
      path: z.string(),
      form: z.string(),
      analysis: z.object({
        presence: z.enum(["unknown", "absent", "present"]),
        extraction: z.enum(["unsupported", "complete", "partial", "failed"])
      }),
      diagnostics: z.array(z.object({
        severity: z.enum(["warning", "error"]),
        code: z.string(),
        message: z.string()
      })).optional().default([])
    })).nullable().transform((value) => value ?? []),
    check_runs: z.array(z.object({
      check_id: z.string(),
      subject: subjectSchema,
      status: z.enum(["completed", "skipped", "failed"]),
      reason_code: z.string().optional(),
      detail: z.string().optional()
    })).nullable().transform((value) => value ?? []),
    findings: z.array(z.object({
      check_id: z.string(),
      severity: z.enum(["info", "low", "medium", "high", "critical"]),
      summary: z.string(),
      subject: subjectSchema,
      locations: z.array(z.object({ path: z.string() })).nullable().transform((value) => value ?? []),
      evidence: z.record(z.string()).optional().default({}),
      remediation: z.string(),
      fingerprint: z.string()
    })).nullable().transform((value) => value ?? [])
  }).nullable()
});

export type ScannerReport = NonNullable<z.infer<typeof scannerReportResponseSchema>["report"]>;

export type ScannerReportState =
  | { kind: "available"; report: ScannerReport }
  | { kind: "absent" }
  | { kind: "unavailable" };

export function parseScannerReportResponse(input: unknown): ScannerReportState {
  const result = scannerReportResponseSchema.safeParse(input);
  if (!result.success) return { kind: "unavailable" };
  if (result.data.report === null) return { kind: "absent" };
  return { kind: "available", report: result.data.report };
}
