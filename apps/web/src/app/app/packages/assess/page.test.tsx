import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { listPackagesServer } from "@/lib/queries";
import AssessmentPage from "./page";

const mocks = vi.hoisted(() => ({ listPackagesServer: vi.fn<typeof listPackagesServer>() }));
vi.mock("@/lib/queries", () => mocks);

describe("vulnerability assessment", () => {
  it("shows a form without claiming an empty assessment before submission", async () => {
    render(await AssessmentPage({}));
    expect(screen.getByLabelText("Exact package name")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Assess" })).toBeInTheDocument();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
  it("keeps invalid input editable", async () => {
    mocks.listPackagesServer.mockRejectedValueOnce(new ApiError(400, "bad range"));
    render(await AssessmentPage({ searchParams: Promise.resolve({ q: "widget", ecosystem: "npm", affected_range: "latest" }) }));
    expect(screen.getByRole("alert")).toHaveTextContent("valid, nonempty affected range");
    expect(screen.getByLabelText("Affected range")).toHaveValue("latest");
  });
  it("shows evidence and preserves the assessment through pagination", async () => {
    mocks.listPackagesServer.mockResolvedValueOnce({
      items: [{ name: "widget", version: "2.3.3", constraint: "", ecosystem: "npm", source: "cli", origin_kind: "registry", destination: "", repository_id: "11111111-1111-1111-1111-111111111111", repository_name: "Workspace", manifest_path: "pnpm-lock.yaml", assessment: { status: "issue", scope: "workspace", root: ".", reasons: ["Lockfile records an affected version."], lockfiles: ["pnpm-lock.yaml"], manifests: ["apps/web/package.json"] } }],
      filters: { q: "widget", version: "", ecosystem: "npm", affected_range: "<=2.3.4" },
      pagination: { page: 1, page_size: 25, total: 26, total_pages: 2, has_previous: false, has_next: true }
    });
    render(await AssessmentPage({ searchParams: Promise.resolve({ q: "widget", ecosystem: "npm", affected_range: "<=2.3.4" }) }));
    expect(screen.getByRole("heading", { name: "Issue: widget@2.3.3" })).toBeInTheDocument();
    expect(screen.getByText(/Affected member projects have not been established/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute("href", "/app/packages/assess?q=widget&ecosystem=npm&affected_range=%3C%3D2.3.4&page=2");
  });
});
