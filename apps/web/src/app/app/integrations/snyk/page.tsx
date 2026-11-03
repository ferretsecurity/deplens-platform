"use client";

import React, { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { snykConfigurationSchema, snykRequest } from "@/lib/snyk";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function SnykPage() {
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: ["snyk-configuration"],
    queryFn: async () => snykConfigurationSchema.parse(await snykRequest()),
    refetchInterval: (query) =>
      ["running", "queued"].includes(query.state.data?.last_run?.status ?? "") ? 2000 : 10000
  });
  const [organization, setOrganization] = useState("");
  const [region, setRegion] = useState("us");
  const [authentication, setAuthentication] = useState("token");
  const [token, setToken] = useState("");
  const [interval, setInterval] = useState(0);
  const [initialized, setInitialized] = useState(false);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    if (query.data && !initialized) {
      setOrganization(query.data.organization);
      setInterval(query.data.interval_minutes);
      setRegion(query.data.region);
      setAuthentication(query.data.authentication);
      setInitialized(true);
    }
  }, [query.data, initialized]);
  const action = useMutation({
    mutationFn: async (kind: "save" | "test" | "sync" | "disconnect") => {
      setNotice("");
      if (kind === "save")
        await snykRequest("", {
          method: "PUT",
          body: JSON.stringify({
            organization,
            region,
            authentication,
            interval_minutes: interval,
            ...(token ? { token } : {})
          })
        });
      if (kind === "test") await snykRequest("/test", { method: "POST" });
      if (kind === "sync") await snykRequest("/sync", { method: "POST" });
      if (kind === "disconnect") await snykRequest("", { method: "DELETE" });
      return kind;
    },
    onSuccess: async (kind) => {
      setToken("");
      if (kind === "disconnect") setInitialized(false);
      setNotice(
        {
          save: "Configuration saved.",
          test: "Connection verified. Target and project listing access are working.",
          sync: "Sync queued. You can leave this page while it runs.",
          disconnect: "Snyk disconnected. Imported Snyk inventory removed."
        }[kind]
      );
      await cache.invalidateQueries({ queryKey: ["snyk-configuration"] });
    }
  });
  const config = query.data;
  const run = config?.last_run;
  const active = run?.status === "queued" || run?.status === "running";
  const busy = action.isPending;
  return (
    <section className="max-w-4xl space-y-6">
      <div>
        <h1 className="text-3xl font-semibold tracking-tight">Snyk integration</h1>
        <p className="mt-2 text-slate-600">
          Import repositories, projects, manifest paths and dependencies from Snyk.
        </p>
      </div>
      {(query.error || action.error) && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
          {(query.error ?? action.error)?.message}
        </p>
      )}
      {notice && (
        <p role="status" className="rounded-lg border border-indigo-200 bg-indigo-50 p-4">
          {notice}
        </p>
      )}
      {config && !config.enabled && (
        <p role="alert" className="rounded-lg border bg-amber-50 p-4">
          Snyk is unavailable. Ask your deployment administrator to configure the Snyk
          encryption key.
        </p>
      )}
      <Card>
        <CardHeader>
          <CardTitle>Connection</CardTitle>
          <CardDescription>
            Use an organization API token with read access. Only administrators can manage this
            connection.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-5"
            onSubmit={(event) => {
              event.preventDefault();
              action.mutate("save");
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="snyk-org">Snyk organization ID</Label>
              <Input
                id="snyk-org"
                value={organization}
                onChange={(event) => setOrganization(event.target.value)}
                required
                disabled={busy || active || !!config?.token_configured}
                placeholder="Organization UUID"
              />
              {config?.token_configured && (
                <p className="text-xs text-slate-500">Disconnect to change the organization.</p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="snyk-region">Snyk region</Label>
              <select id="snyk-region" value={region} onChange={(event) => setRegion(event.target.value)} disabled={busy || active || !!config?.token_configured} className="block w-full rounded-md border border-slate-200 bg-white p-2 text-sm">
                <option value="us">US (api.snyk.io)</option>
                <option value="us2">US 2 (api.us.snyk.io)</option>
                <option value="eu">EU (api.eu.snyk.io)</option>
                <option value="au">AU (api.au.snyk.io)</option>
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="snyk-authentication">Authentication</Label>
              <select id="snyk-authentication" value={authentication} onChange={(event) => setAuthentication(event.target.value)} disabled={busy || active} className="block w-full rounded-md border border-slate-200 bg-white p-2 text-sm">
                <option value="token">API token</option>
                <option value="bearer">OAuth bearer token</option>
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="snyk-token">API token</Label>
              <Input
                id="snyk-token"
                type="password"
                autoComplete="new-password"
                value={token}
                onChange={(event) => setToken(event.target.value)}
                disabled={busy || active}
                placeholder={
                  config?.token_configured
                    ? "Token saved. Leave blank to keep it."
                    : "Paste a Snyk API token"
                }
              />
              <p className="text-sm text-slate-600">
                Use a Snyk API token with read access to projects and dependency graphs in this organization.{" "}
                <a
                  href="https://docs.snyk.io/snyk-api/authentication-for-api"
                  className="underline"
                  target="_blank"
                  rel="noreferrer"
                >
                  Snyk API authentication ↗
                </a>
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="snyk-schedule">Synchronization schedule</Label>
              <select
                id="snyk-schedule"
                className="block w-full rounded-md border border-slate-200 bg-white p-2 text-sm"
                value={interval}
                onChange={(event) => setInterval(Number(event.target.value))}
                disabled={busy || active}
              >
                <option value={0}>Manual only</option>
                <option value={60}>Every hour</option>
                <option value={360}>Every 6 hours</option>
                <option value={720}>Every 12 hours</option>
                <option value={1440}>Every day</option>
              </select>
            </div>
            <div className="flex flex-wrap gap-3">
              <Button type="submit" disabled={busy || active || !config?.enabled}>
                Save configuration
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={busy || active || !config?.token_configured || !config.enabled}
                onClick={() => action.mutate("test")}
              >
                Test connection
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={busy || !config?.token_configured}
                onClick={() => {
                  if (
                    window.confirm(
                      "Disconnect Snyk and remove its imported inventory? CLI scans will remain."
                    )
                  )
                    action.mutate("disconnect");
                }}
              >
                Disconnect
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Synchronization</CardTitle>
          <CardDescription>
            Each repository keeps its previous inventory until its new import succeeds.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <span className="font-medium capitalize">{run?.status ?? "Not synced yet"}</span>
            <Button
              disabled={busy || active || !config?.token_configured || !config?.enabled}
              onClick={() => action.mutate("sync")}
            >
              {active ? "Sync in progress…" : "Sync now"}
            </Button>
          </div>
          {run && (
            <div className="space-y-2" aria-live="polite">
              <progress
                className="w-full accent-indigo-600"
                max={Math.max(run.total, 1)}
                value={run.completed}
              />
              <p>
                {run.completed} of {run.total} repositories processed · {run.failed} failed
              </p>
              {run.failed > 0 && (
                <p className="text-amber-800">
                  Some repositories could not be imported. Their previous inventory was retained.
                  Open a repository to see its status, then sync again to retry.
                </p>
              )}
              {run.message && <p>{run.message}</p>}
              <p className="text-sm text-slate-500">
                Started {new Date(run.started_at).toLocaleString()}
                {run.finished_at && ` · Finished ${new Date(run.finished_at).toLocaleString()}`}
              </p>
            </div>
          )}
          <p className="text-sm text-slate-600">
            Next sync{" "}
            {config?.next_run_at ? new Date(config.next_run_at).toLocaleString() : "Manual only"}
          </p>
        </CardContent>
      </Card>
    </section>
  );
}
