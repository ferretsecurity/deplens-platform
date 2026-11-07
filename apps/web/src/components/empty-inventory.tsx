import React from "react";
import Link from "next/link";

import { fetchCurrentUserServer } from "@/lib/auth.server";

export async function EmptyInventory() {
  const user = await fetchCurrentUserServer();
  const canManage = user.role === "owner" || user.role === "admin";
  return (
    <div className="space-y-3 text-sm text-slate-600">
      <p>Upload a scan to populate repositories, manifest files, packages and dependencies.</p>
      <ol className="list-decimal space-y-2 pl-5">
        <li>{canManage ? <Link className="font-medium underline" href="/app/tokens">Issue an API token</Link> : "Ask an owner or admin for a scanner token"} with <code>scan:write</code>. Save its one-time secret for your scanner or CI.</li>
        <li>Run <code className="break-all">deplens -json /path/to/repo &gt; /tmp/deplens.json</code>.</li>
        <li>Upload that report to <code>POST /api/v1/scans</code> with the token as a Bearer credential and repository/source headers. Then open Scans to inspect the run. An upload can contain no dependencies; check its source analysis and diagnostics.</li>
      </ol>
      <details>
        <summary className="cursor-pointer font-medium text-slate-900">Upload command</summary>
        <p className="my-2">Replace the API address, token, repository and commit values, and scan timestamp with your own.</p>
        <pre className="overflow-x-auto rounded-md bg-slate-100 p-3 text-xs"><code>{`curl -X POST https://YOUR-API/api/v1/scans \\
  -H 'Authorization: Bearer YOUR-TOKEN' \\
  -H 'Content-Type: application/json' \\
  -H 'X-Deplens-Repository-Name: YOUR-REPO' \\
  -H 'X-Deplens-Repository-URL: https://example.com/repo.git' \\
  -H 'X-Deplens-Default-Branch: main' \\
  -H 'X-Deplens-Commit-SHA: YOUR-COMMIT' \\
  -H 'X-Deplens-Ref: refs/heads/main' \\
  -H 'X-Deplens-Scanned-At: YYYY-MM-DDTHH:MM:SSZ' \\
  --data-binary @/tmp/deplens.json`}</code></pre>
      </details>
      {canManage ? <p>Or import inventory through <Link className="underline" href="/app/integrations/socket">Socket</Link> or <Link className="underline" href="/app/integrations/snyk">Snyk</Link>. Provider inventory appears in repositories and packages; scan history comes from scanner uploads.</p> : <p>An owner or admin can also configure Socket or Snyk imports.</p>}
    </div>
  );
}
