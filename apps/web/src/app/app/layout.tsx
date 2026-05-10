import { redirect } from "next/navigation";

import { AppShell } from "@/components/app-shell";
import { fetchCurrentUserServer } from "@/lib/auth.server";

export const dynamic = "force-dynamic";

export default async function AppLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  try {
    const user = await fetchCurrentUserServer();
    return <AppShell user={user}>{children}</AppShell>;
  } catch {
    redirect("/login");
  }
}
