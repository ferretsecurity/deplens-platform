import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { EmptyInventory } from "./empty-inventory";

const mocks = vi.hoisted(() => ({ fetchCurrentUserServer: vi.fn() }));
vi.mock("@/lib/auth.server", () => mocks);

describe("EmptyInventory", () => {
  it("offers token and provider entry points to administrators", async () => {
    mocks.fetchCurrentUserServer.mockResolvedValue({ role: "admin" });
    render(await EmptyInventory());
    expect(screen.getByRole("link", { name: "Issue an API token" })).toHaveAttribute("href", "/app/tokens");
    expect(screen.getByRole("link", { name: "Socket" })).toHaveAttribute("href", "/app/integrations/socket");
    expect(screen.getByText(/deplens -json/)).toBeInTheDocument();
  });
  it("guides viewers through an administrator without restricted links", async () => {
    mocks.fetchCurrentUserServer.mockResolvedValue({ role: "viewer" });
    render(await EmptyInventory());
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText(/Ask an owner or admin/)).toBeInTheDocument();
  });
});
