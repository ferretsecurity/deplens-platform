import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import TokensPage from "./page";

vi.mock("@/lib/queries", () => ({
  listAPITokensServer: vi.fn().mockResolvedValue([
    {
      id: "token-1",
      label: "CI scanner",
      scopes: ["scan:read", "scan:write"],
      created_at: "2026-05-11T10:00:00Z"
    }
  ])
}));

describe("TokensPage", () => {
  it("renders token metadata without a secret", async () => {
    render(await TokensPage());

    expect(screen.getByText("API Tokens")).toBeInTheDocument();
    expect(screen.getByText("CI scanner")).toBeInTheDocument();
    expect(screen.getAllByText("scan:read").length).toBeGreaterThan(0);
    expect(screen.queryByText("plain-token")).not.toBeInTheDocument();
  });
});
