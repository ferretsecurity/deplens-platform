"use client";

import React from "react";

import { Button } from "@/components/ui/button";

export function ListPagination({ page, pageSize, total, onPageChange }: {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}) {
  if (total <= pageSize) return null;
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 py-3 text-sm text-slate-600">
      <span>{page * pageSize + 1} to {Math.min((page + 1) * pageSize, total)} of {total}</span>
      <div className="flex gap-2">
        <Button type="button" variant="outline" size="sm" disabled={page === 0} onClick={() => onPageChange(page - 1)}>Previous</Button>
        <Button type="button" variant="outline" size="sm" disabled={(page + 1) * pageSize >= total} onClick={() => onPageChange(page + 1)}>Next</Button>
      </div>
    </div>
  );
}
