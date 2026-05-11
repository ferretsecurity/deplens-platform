import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AppShell } from "./app-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/tokens"
}));

vi.mock("@/lib/auth", () => ({
  logout: vi.fn()
}));

describe("AppShell", () => {
  it("links to API token management", () => {
    render(
      <AppShell
        user={{
          user_id: "user-1",
          display_name: "Admin",
          memberships: [],
          active_tenant_id: "tenant-1",
          role: "owner"
        }}
      >
        <div>Content</div>
      </AppShell>
    );

    expect(screen.getAllByRole("link", { name: /api tokens/i })[0]).toHaveAttribute("href", "/app/tokens");
  });
});
