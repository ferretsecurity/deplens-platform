import React from "react";
import { LoginForm } from "@/components/login-form";

export const dynamic = "force-dynamic";

export default function LoginPage() {
  const prefill = process.env.NODE_ENV === "development" && process.env.DEV_LOGIN_PREFILL === "true";
  const defaultValues = prefill
    ? { email: process.env.BOOTSTRAP_OWNER_EMAIL ?? "", password: process.env.BOOTSTRAP_OWNER_PASSWORD ?? "" }
    : undefined;
  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-10">
      <LoginForm defaultValues={defaultValues} />
    </main>
  );
}
