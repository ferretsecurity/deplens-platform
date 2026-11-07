import React from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AppShell } from "./app-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/tokens"
}));

vi.mock("@/lib/auth", () => ({
  logout: vi.fn()
}));

const user = {
  user_id: "user-1",
  display_name: "An account with a very long display name for navigation",
  memberships: [],
  active_tenant_id: "tenant-1",
  role: "owner"
};

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
    expect(screen.getAllByRole("link", { name: /^packages$/i })[0]).toHaveAttribute("href", "/app/packages");
  });

  it("names the menu and marks only the active destination", async () => {
    const keyboard = userEvent.setup();
    render(<AppShell user={user}>Content</AppShell>);

    expect(screen.getByRole("link", { name: "API Tokens" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Packages" })).not.toHaveAttribute("aria-current");
    await keyboard.click(screen.getByRole("button", { name: "Open navigation" }));
    const drawer = within(screen.getByRole("dialog"));
    expect(drawer.getByRole("link", { name: "API Tokens" })).toHaveAttribute("aria-current", "page");
    expect(drawer.getByRole("link", { name: "Packages" })).not.toHaveAttribute("aria-current");
  });

  it.each(["Packages", "API Tokens"])("closes after selecting %s and restores menu focus", async (destination) => {
    const keyboard = userEvent.setup();
    render(<AppShell user={user}>Content</AppShell>);
    const menu = screen.getByRole("button", { name: "Open navigation" });
    menu.focus();
    await keyboard.keyboard("{Enter}");
    const link = within(screen.getByRole("dialog")).getByRole("link", { name: destination });
    link.addEventListener("click", (event) => event.preventDefault());
    link.focus();
    await keyboard.keyboard("{Enter}");

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(menu).toHaveFocus();
    expect(menu).toHaveAttribute("aria-expanded", "false");
    await keyboard.keyboard("{Enter}");
    expect(screen.getByRole("dialog")).toBeVisible();
    await keyboard.keyboard("{Escape}");
    await waitFor(() => expect(menu).toHaveFocus());
  });

  it.each(["owner", "admin", "member", "viewer"])("preserves provider navigation for %s", async (role) => {
    const keyboard = userEvent.setup();
    render(<AppShell user={{ ...user, role }}>Content</AppShell>);
    const allowed = role === "owner" || role === "admin";
    expect(screen.queryAllByRole("link", { name: /integration$/ })).toHaveLength(allowed ? 2 : 0);
    await keyboard.click(screen.getByRole("button", { name: "Open navigation" }));
    expect(within(screen.getByRole("dialog")).queryAllByRole("link", { name: /integration$/ })).toHaveLength(allowed ? 2 : 0);
  });

});
