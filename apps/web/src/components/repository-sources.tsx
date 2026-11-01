import React from "react";

export function RepositorySources({
  repository
}: {
  repository: { cli?: boolean; socket?: boolean; socket_url?: string };
}) {
  return (
    <span className="inline-flex flex-wrap items-center gap-2 text-xs">
      {(repository.cli ?? !repository.socket) && (
        <span className="rounded border border-slate-200 bg-slate-50 px-2 py-1">CLI</span>
      )}
      {repository.socket && (
        <span className="rounded border border-indigo-200 bg-indigo-50 px-2 py-1">Socket</span>
      )}
      {repository.socket_url && (
        <a
          href={repository.socket_url}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1 underline"
          aria-label="Open Socket dashboard"
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src="/icons/socket.svg" alt="" width={16} height={16} /> Socket ↗
        </a>
      )}
    </span>
  );
}
