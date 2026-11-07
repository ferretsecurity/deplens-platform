"use client";

import React, { useMemo, useState, useRef, useEffect } from "react";
import { Check, Pencil, Plus, Trash2, X } from "lucide-react";

import { clientApiFetch } from "@/lib/api";
import type { APITokenMetadata, CreatedAPIToken } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const allowedScopes = [
  { id: "scan:read", help: "Read scan history and inventory" },
  { id: "scan:write", help: "Upload scanner reports" },
  { id: "scan:metadata:write", help: "Update scan labels and annotations" }
];

type Feedback =
  | { kind: "idle" }
  | { kind: "pending"; action: "save" | "delete" }
  | { kind: "invalid"; field: "label" | "scopes"; message: string }
  | { kind: "error" | "success"; message: string };

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
  const [feedback, setFeedback] = useState<Feedback>({ kind: "idle" });
  const labelRef = useRef<HTMLInputElement>(null);
  const scopeRef = useRef<HTMLInputElement>(null);
  const cancelDeleteRef = useRef<HTMLButtonElement>(null);
  const actionOrigin = useRef<HTMLButtonElement | null>(null);
  const focusAfterRender = useRef<HTMLElement | null>(null);
  useEffect(() => {
    focusAfterRender.current?.focus();
    focusAfterRender.current = null;
  });
  const submitting = feedback.kind === "pending";
  useEffect(() => {
    if (mode.type === "edit") labelRef.current?.focus();
  }, [mode]);
  useEffect(() => {
    if (confirmingDelete) cancelDeleteRef.current?.focus();
  }, [confirmingDelete]);

  const submitLabel = mode.type === "edit" ? "Save token" : "Issue token";
  const sortedTokens = useMemo(
    () => [...tokens].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)),
    [tokens]
  );

  function resetForm() {
    setMode({ type: "create" });
    setLabel("");
    setScopes([]);
    setFeedback({ kind: "idle" });
  }

  function editToken(token: APITokenMetadata) {
    setMode({ type: "edit", token });
    setLabel(token.label);
    setScopes(token.scopes);
    setFeedback({ kind: "idle" });
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
      setFeedback({ kind: "invalid", field: "label", message: "Label is required." });
      labelRef.current?.focus();
      return;
    }
    if (scopes.length === 0) {
      setFeedback({ kind: "invalid", field: "scopes", message: "Choose at least one scope." });
      scopeRef.current?.focus();
      return;
    }

    setFeedback({ kind: "pending", action: "save" });
    try {
      if (mode.type === "edit") {
        const updated = await clientApiFetch<APITokenMetadata>(`/api/v1/tokens/${mode.token.id}`, {
          method: "PATCH",
          body: JSON.stringify({ label: trimmedLabel, scopes })
        });
        setTokens((current) => current.map((token) => (token.id === updated.id ? updated : token)));
        resetForm();
        setFeedback({ kind: "success", message: "Token updated." });
        focusAfterRender.current = actionOrigin.current;
      } else {
        const item = await clientApiFetch<CreatedAPIToken>("/api/v1/tokens", {
          method: "POST",
          body: JSON.stringify({ label: trimmedLabel, scopes })
        });
        setTokens((current) => [item, ...current]);
        setCreated(item);
        resetForm();
        setFeedback({ kind: "success", message: "Token issued. Store the one-time secret before dismissing it." });
        focusAfterRender.current = labelRef.current;
      }
    } catch {
      setFeedback({ kind: "error", message: "Token change failed. Your label and scopes are still here. Check your connection and try again." });
    }
  }

  async function deleteToken(token: APITokenMetadata) {
    setFeedback({ kind: "pending", action: "delete" });
    try {
      await clientApiFetch<void>(`/api/v1/tokens/${token.id}`, { method: "DELETE" });
      setTokens((current) => current.filter((item) => item.id !== token.id));
      setConfirmingDelete(null);
      if (mode.type === "edit" && mode.token.id === token.id) resetForm();
      setFeedback({ kind: "success", message: "Token deleted. Scanners using it can no longer authenticate." });
      focusAfterRender.current = labelRef.current;
    } catch {
      setFeedback({ kind: "error", message: "Delete failed. The token list has not changed. Check your connection and try again." });
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

      {feedback.kind === "success" ? <p role="status" className="text-sm text-emerald-800">{feedback.message}</p> : null}
      {feedback.kind === "error" ? <p role="alert" className="text-sm text-red-700">{feedback.message}</p> : null}
      <form aria-label={mode.type === "edit" ? "Edit token" : "Issue token"} aria-busy={submitting} className="grid gap-4 rounded-lg border border-slate-200 bg-slate-50 p-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto]" onSubmit={submitToken}>
        <div className="space-y-2">
          <Label htmlFor="token-label">Label</Label>
          <Input
            ref={labelRef}
            disabled={submitting || !!confirmingDelete}
            aria-invalid={feedback.kind === "invalid" && feedback.field === "label"}
            aria-describedby={feedback.kind === "invalid" && feedback.field === "label" ? "token-label-error" : undefined}
            id="token-label"
            placeholder="CI scanner"
            value={label}
            onChange={(event) => setLabel(event.target.value)}
          />
        </div>
        <fieldset disabled={submitting || !!confirmingDelete} aria-describedby={feedback.kind === "invalid" && feedback.field === "scopes" ? "token-scopes-error" : undefined} className="min-w-0 space-y-2">
          <legend className="text-sm font-medium">Scopes</legend>
          <div className="flex flex-wrap gap-2">
            {allowedScopes.map((scope) => (
              <label
                key={scope.id}
                className="inline-flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700"
              >
                <input
                  ref={scope.id === "scan:read" ? scopeRef : undefined}
                  aria-label={scope.id}
                  aria-describedby={`${scope.id}-help`}
                  aria-invalid={feedback.kind === "invalid" && feedback.field === "scopes"}
                  checked={scopes.includes(scope.id)}
                  className="h-4 w-4"
                  onChange={() => toggleScope(scope.id)}
                  type="checkbox"
                />
                <span><code>{scope.id}</code><span id={`${scope.id}-help`} className="block text-xs text-slate-500">{scope.help}</span></span>
              </label>
            ))}
          </div>
        </fieldset>
        <div className="flex flex-wrap items-end gap-2">
          <Button disabled={submitting || !!confirmingDelete} type="submit">
            {mode.type === "edit" ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
            {submitting && feedback.action === "save" ? "Saving token…" : submitLabel}
          </Button>
          {mode.type === "edit" ? (
            <Button type="button" variant="outline" disabled={submitting || !!confirmingDelete} onClick={() => { resetForm(); focusAfterRender.current = actionOrigin.current; }}>
              Cancel
            </Button>
          ) : null}
        </div>
        {feedback.kind === "invalid" ? <p id={`token-${feedback.field}-error`} role="alert" className="text-sm text-red-600 xl:col-span-3">{feedback.message}</p> : null}
      </form>

      {tokens.length === 0 ? (
        <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-600">
          No API tokens have been issued yet.
        </div>
      ) : (
        <Table layout="reflow" aria-label="API tokens">
          <TableHeader>
            <TableRow>
              <TableHead>Label</TableHead>
              <TableHead>Scopes</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-56 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedTokens.map((token) => (
              <TableRow key={token.id}>
                <TableCell mobileLabel="Label" className="table-cell-wide table-cell-identity font-medium">{token.label}</TableCell>
                <TableCell mobileLabel="Scopes">
                  <div className="flex flex-wrap gap-1.5">
                    {token.scopes.map((scope) => (
                      <span key={scope} className="rounded-md bg-slate-100 px-2 py-1 text-xs text-slate-700">
                        {scope}
                      </span>
                    ))}
                  </div>
                </TableCell>
                <TableCell mobileLabel="Created" className="text-slate-600">
                  <time dateTime={token.created_at} title={new Date(token.created_at).toISOString()}>
                    {new Date(token.created_at).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" })}{" "}
                    <span className="inline-block whitespace-nowrap">{new Date(token.created_at).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" })}</span>
                  </time>
                </TableCell>
                <TableCell mobileLabel="Actions" className="table-cell-wide text-right">
                  <div className="table-actions flex flex-wrap justify-end gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      aria-label={`Edit ${token.label}`}
                      disabled={submitting || !!confirmingDelete}
                      onClick={(event) => { actionOrigin.current = event.currentTarget; editToken(token); }}
                    >
                      <Pencil className="h-4 w-4" />
                      Edit
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      aria-label={`Delete ${token.label}`}
                      disabled={submitting || !!confirmingDelete}
                      onClick={(event) => { actionOrigin.current = event.currentTarget; setFeedback({ kind: "idle" }); setConfirmingDelete(token); }}
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
        <div role="group" aria-label={`Delete ${confirmingDelete.label}?`} className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <p>
              Create a replacement, update CI or scanners, then delete{" "}
              <span className="font-medium">{confirmingDelete.label}</span>.
            </p>
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="outline" size="sm" ref={cancelDeleteRef} disabled={submitting} onClick={() => { setConfirmingDelete(null); setFeedback({ kind: "idle" }); focusAfterRender.current = actionOrigin.current; }}>
                Cancel
              </Button>
              <Button
                type="button"
                variant="destructive"
                size="sm"
                disabled={submitting}
                aria-label={`Confirm delete ${confirmingDelete.label}`}
                onClick={() => void deleteToken(confirmingDelete)}
              >
                <Trash2 className="h-4 w-4" />
                {submitting ? "Deleting token…" : "Confirm delete"}
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
