"use client";

import React, { useMemo, useState } from "react";
import { Check, Pencil, Plus, Trash2, X } from "lucide-react";

import { clientApiFetch } from "@/lib/api";
import type { APITokenMetadata, CreatedAPIToken } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const allowedScopes = ["scan:read", "scan:write", "scan:metadata:write"] as const;

type TokenManagementClientProps = {
  initialTokens: APITokenMetadata[];
};

type FormMode =
  | { type: "create" }
  | { type: "edit"; token: APITokenMetadata };

export function TokenManagementClient({ initialTokens }: TokenManagementClientProps) {
  const [tokens, setTokens] = useState(initialTokens);
  const [mode, setMode] = useState<FormMode>({ type: "create" });
  const [label, setLabel] = useState("");
  const [scopes, setScopes] = useState<string[]>([]);
  const [created, setCreated] = useState<CreatedAPIToken | null>(null);
  const [confirmingDelete, setConfirmingDelete] = useState<APITokenMetadata | null>(null);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const submitLabel = mode.type === "edit" ? "Save token" : "Issue token";
  const sortedTokens = useMemo(
    () => [...tokens].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)),
    [tokens]
  );

  function resetForm() {
    setMode({ type: "create" });
    setLabel("");
    setScopes([]);
    setError("");
  }

  function editToken(token: APITokenMetadata) {
    setMode({ type: "edit", token });
    setLabel(token.label);
    setScopes(token.scopes);
    setError("");
  }

  function toggleScope(scope: string) {
    setScopes((current) =>
      current.includes(scope) ? current.filter((item) => item !== scope) : [...current, scope]
    );
  }

  async function submitToken(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedLabel = label.trim();
    if (!trimmedLabel) {
      setError("Label is required.");
      return;
    }
    if (scopes.length === 0) {
      setError("Choose at least one scope.");
      return;
    }

    setSubmitting(true);
    setError("");
    try {
      if (mode.type === "edit") {
        const updated = await clientApiFetch<APITokenMetadata>(`/api/v1/tokens/${mode.token.id}`, {
          method: "PATCH",
          body: JSON.stringify({ label: trimmedLabel, scopes })
        });
        setTokens((current) => current.map((token) => (token.id === updated.id ? updated : token)));
        resetForm();
      } else {
        const item = await clientApiFetch<CreatedAPIToken>("/api/v1/tokens", {
          method: "POST",
          body: JSON.stringify({ label: trimmedLabel, scopes })
        });
        setTokens((current) => [item, ...current]);
        setCreated(item);
        resetForm();
      }
    } catch {
      setError("Token change failed. Keep the current list visible and try again.");
    } finally {
      setSubmitting(false);
    }
  }

  async function deleteToken(token: APITokenMetadata) {
    setSubmitting(true);
    setError("");
    try {
      await clientApiFetch<void>(`/api/v1/tokens/${token.id}`, { method: "DELETE" });
      setTokens((current) => current.filter((item) => item.id !== token.id));
      setConfirmingDelete(null);
    } catch {
      setError("Delete failed. The token list has not changed.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-5">
      {created ? (
        <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-950">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="space-y-1">
              <div className="font-medium">One-time token secret</div>
              <p className="text-emerald-900">Store this value now. It cannot be shown again.</p>
              <code className="block break-all rounded-md border border-emerald-200 bg-white px-3 py-2 font-mono text-xs text-slate-900">
                {created.token}
              </code>
            </div>
            <Button type="button" variant="outline" size="sm" onClick={() => setCreated(null)}>
              <X className="h-4 w-4" />
              Dismiss
            </Button>
          </div>
        </div>
      ) : null}

      <form className="grid gap-4 rounded-lg border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(180px,1fr)_minmax(260px,2fr)_auto]" onSubmit={submitToken}>
        <div className="space-y-2">
          <Label htmlFor="token-label">Label</Label>
          <Input
            id="token-label"
            placeholder="CI scanner"
            value={label}
            onChange={(event) => setLabel(event.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label>Scopes</Label>
          <div className="flex flex-wrap gap-2">
            {allowedScopes.map((scope) => (
              <label
                key={scope}
                className="inline-flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700"
              >
                <input
                  checked={scopes.includes(scope)}
                  className="h-4 w-4"
                  onChange={() => toggleScope(scope)}
                  type="checkbox"
                />
                {scope}
              </label>
            ))}
          </div>
        </div>
        <div className="flex items-end gap-2">
          <Button disabled={submitting} type="submit">
            {mode.type === "edit" ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
            {submitLabel}
          </Button>
          {mode.type === "edit" ? (
            <Button type="button" variant="outline" onClick={resetForm}>
              Cancel
            </Button>
          ) : null}
        </div>
        {error ? <p className="text-sm text-red-600 lg:col-span-3">{error}</p> : null}
      </form>

      {tokens.length === 0 ? (
        <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
          No API tokens have been issued yet.
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Label</TableHead>
              <TableHead>Scopes</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedTokens.map((token) => (
              <TableRow key={token.id}>
                <TableCell className="font-medium">{token.label}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1.5">
                    {token.scopes.map((scope) => (
                      <span key={scope} className="rounded-md bg-slate-100 px-2 py-1 text-xs text-slate-700">
                        {scope}
                      </span>
                    ))}
                  </div>
                </TableCell>
                <TableCell className="text-slate-600">
                  {new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short" }).format(
                    new Date(token.created_at)
                  )}
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      aria-label={`Edit ${token.label}`}
                      onClick={() => editToken(token)}
                    >
                      <Pencil className="h-4 w-4" />
                      Edit
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      aria-label={`Delete ${token.label}`}
                      onClick={() => setConfirmingDelete(token)}
                    >
                      <Trash2 className="h-4 w-4" />
                      Delete
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {confirmingDelete ? (
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <p>
              Create a replacement, update CI or scanners, then delete{" "}
              <span className="font-medium">{confirmingDelete.label}</span>.
            </p>
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => setConfirmingDelete(null)}>
                Cancel
              </Button>
              <Button
                type="button"
                variant="destructive"
                size="sm"
                aria-label={`Confirm delete ${confirmingDelete.label}`}
                onClick={() => void deleteToken(confirmingDelete)}
              >
                <Trash2 className="h-4 w-4" />
                Confirm delete
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
