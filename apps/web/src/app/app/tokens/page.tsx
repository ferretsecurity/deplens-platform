import React from "react";

import { TokenManagementClient } from "./token-management-client";
import { listAPITokensServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default async function TokensPage() {
  const tokens = await listAPITokensServer();

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>API Tokens</CardTitle>
          <CardDescription>
            Issue scanner tokens, adjust token scopes, and remove old credentials after rotation.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <TokenManagementClient initialTokens={tokens} />
        </CardContent>
      </Card>
    </section>
  );
}
