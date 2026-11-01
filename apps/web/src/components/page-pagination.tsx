"use client";

import React from "react";
import { usePathname, useRouter } from "next/navigation";

import { ListPagination } from "@/components/list-pagination";

export function PagePagination({ page, pageSize, total }: { page: number; pageSize: number; total: number }) {
  const pathname = usePathname();
  const router = useRouter();
  return <ListPagination page={page} pageSize={pageSize} total={total} onPageChange={(nextPage) => router.push(`${pathname}?page=${nextPage + 1}`)} />;
}
