import React from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import type { SocketInventory } from "@/lib/socket";
import { SocketInventoryView } from "./socket-inventory";

function source(id: string, count: number): SocketInventory["sources"][number] {
  return {
    id, available: true, error: "", synced_at: null,
    snapshot: {
      files: [],
      metadata: { branch: "main", commit_hash: "", integration_repo_url: "", html_report_url: "" },
      artifacts: Array.from({ length: count }, (_, i) => ({
        id: `${id}-${i}`, name: `${id}-${i}`, type: "npm", namespace: "", version: "1",
        direct: null, dev: null, manifestFiles: []
      }))
    }
  };
}

it("keeps each mapped source on its own page", () => {
  render(<SocketInventoryView sources={[source("large", 100), source("small", 10)]} />);
  fireEvent.click(screen.getAllByRole("button", { name: "Next results" })[0]);
  expect(screen.getByText("large-50@1")).toBeVisible();
  expect(screen.getByText("small-0@1")).toBeVisible();
  expect(screen.getByText("Page 2 of 2")).toBeVisible();
  expect(screen.getByText("Page 1 of 1")).toBeVisible();
  expect(screen.queryByText("Page 2 of 1")).not.toBeInTheDocument();
});
