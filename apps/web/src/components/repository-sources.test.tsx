import React from "react";
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { RepositorySources } from "./repository-sources";

it("shows all linked providers and their dashboards", () => {
  render(<RepositorySources repository={{ cli: true, socket: true, socket_url: "https://socket.dev/dashboard/test", snyk: true, snyk_url: "https://app.snyk.io/org/test/projects" }} />);
  expect(screen.getByText("CLI", { exact: true })).toBeVisible();
  expect(screen.getByText("Socket", { exact: true })).toBeVisible();
  expect(screen.getByText("Snyk", { exact: true })).toBeVisible();
  expect(screen.getByRole("link", { name: "Open Snyk dashboard" })).toHaveAttribute("href", "https://app.snyk.io/org/test/projects");
});

it("does not label a Snyk-only repository as CLI", () => {
  render(<RepositorySources repository={{ snyk: true }} />);
  expect(screen.getByText("Snyk")).toBeVisible();
  expect(screen.queryByText("CLI")).not.toBeInTheDocument();
});
