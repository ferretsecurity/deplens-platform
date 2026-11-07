import React from "react";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ScansPage from "./page";

const mocks = vi.hoisted(() => ({
  listRepositoryOptionsServer: vi.fn(),
  listScansServer: vi.fn()
}));

vi.mock("@/lib/queries", () => mocks);
vi.mock("@/components/empty-inventory", () => ({
  EmptyInventory: () => <p>Scanner setup instructions</p>
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.listRepositoryOptionsServer.mockResolvedValue([{ id: "repo-1", name: "Example" }]);
  mocks.listScansServer.mockResolvedValue([]);
});

describe("scan recovery", () => {
  it("offers only selection guidance for an unknown repository", async () => {
    render(await ScansPage({ searchParams: Promise.resolve({ repository_id: "missing" }) }));
    expect(screen.getByText("This repository is unavailable. Select another repository above.")).toBeVisible();
    expect(screen.queryByRole("link", { name: "Show all scan dates" })).not.toBeInTheDocument();
    expect(screen.queryByText("Scanner setup instructions")).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Unavailable repository", selected: true })).toHaveValue("missing");
    expect(mocks.listScansServer).not.toHaveBeenCalled();
  });

  it("offers date reset before collapsed setup for a known repository", async () => {
    render(await ScansPage({ searchParams: Promise.resolve({ repository_id: "repo-1", from: "2020-01-01", to: "2020-01-02" }) }));
    expect(screen.getByRole("link", { name: "Show all scan dates" })).toHaveAttribute("href", expect.stringContaining("repository_id=repo-1&from=1970-01-01"));
    const guidance = screen.getByText("How to upload a scan").closest("details");
    expect(guidance).not.toHaveAttribute("open");
    expect(guidance).toHaveTextContent("Scanner setup instructions");
  });

  it("keeps initial empty inventory instructions visible", async () => {
    mocks.listRepositoryOptionsServer.mockResolvedValue([]);
    render(await ScansPage({}));
    expect(screen.getByText("Scanner setup instructions")).toBeVisible();
    expect(screen.queryByText("How to upload a scan")).not.toBeInTheDocument();
  });
});
