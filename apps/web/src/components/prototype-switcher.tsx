"use client";

import React, { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

export function PrototypeSwitcher({ variants, current, state }: { variants: string[]; current: string; state: string }) {
  const router = useRouter();
  const params = useSearchParams();
  const index = Math.max(0, variants.findIndex(variant => variant.startsWith(current)));
  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.target instanceof Element && event.target.closest('input, textarea, select, [contenteditable], [role="slider"]')) return;
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      const next = variants[(index + (event.key === "ArrowRight" ? 1 : -1) + variants.length) % variants.length];
      const search = new URLSearchParams(params.toString());
      search.set("variant", next.charAt(0));
      router.replace(`?${search}`, { scroll: false });
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [index, params, router, variants]);
  if (process.env.NODE_ENV === "production") return null;
  function move(direction: number) {
    const search = new URLSearchParams(params.toString());
    search.set("variant", variants[(index + direction + variants.length) % variants.length].charAt(0));
    router.replace(`?${search}`, { scroll: false });
  }
  return <aside aria-label="Throwaway prototype controls" className="fixed bottom-5 left-1/2 z-50 w-max max-w-[95vw] -translate-x-1/2 rounded-2xl bg-slate-900 px-4 py-3 text-white shadow-xl"><div className="flex items-center justify-between gap-5"><button aria-label="Previous variant" onClick={() => move(-1)}>←</button><span className="text-sm"><span className="mr-3 text-xs text-slate-400">PROTOTYPE</span>{variants[index]}</span><button aria-label="Next variant" onClick={() => move(1)}>→</button></div><p className="mt-2 text-center text-[11px] text-slate-400">{state}</p></aside>;
}
