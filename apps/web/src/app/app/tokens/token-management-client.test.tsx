import React from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TokenManagementClient } from "./token-management-client";

const initialTokens = [
  {
    id: "token-1",
    label: "CI scanner",
    scopes: ["scan:read"],
    created_at: "2026-05-11T10:00:00Z"
  }
];

afterEach(() => {
  vi.restoreAllMocks();
});

describe("TokenManagementClient", () => {
  it("creates a token and shows the one-time secret", async () => {
    const fetchMock = vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "token-2",
          label: "Release scanner",
          scopes: ["scan:write"],
          created_at: "2026-05-11T11:00:00Z",
          token: "plain-token"
        }),
        { status: 201 }
      )
    );

    render(<TokenManagementClient initialTokens={initialTokens} />);

    await userEvent.type(screen.getByLabelText("Label"), "Release scanner");
    await userEvent.click(screen.getByLabelText("scan:write"));
    await userEvent.click(screen.getByRole("button", { name: /issue token/i }));

    await screen.findByText("plain-token");
    expect(screen.getByText("Release scanner")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/tokens",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ label: "Release scanner", scopes: ["scan:write"] })
      })
    );
  });

  it("edits label and scopes without sending a secret", async () => {
    const fetchMock = vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "token-1",
          label: "Updated scanner",
          scopes: ["scan:metadata:write"],
          created_at: "2026-05-11T10:00:00Z"
        }),
        { status: 200 }
      )
    );

    render(<TokenManagementClient initialTokens={initialTokens} />);

    await userEvent.click(screen.getByRole("button", { name: /edit ci scanner/i }));
    const label = screen.getByLabelText("Label");
    await userEvent.clear(label);
    await userEvent.type(label, "Updated scanner");
    await userEvent.click(screen.getByLabelText("scan:read"));
    await userEvent.click(screen.getByLabelText("scan:metadata:write"));
    await userEvent.click(screen.getByRole("button", { name: /save token/i }));

    await screen.findByText("Updated scanner");
    const body = JSON.parse(String(fetchMock.mock.calls[0][1]?.body));
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/tokens/token-1",
      expect.objectContaining({ method: "PATCH" })
    );
    expect(body).toEqual({ label: "Updated scanner", scopes: ["scan:metadata:write"] });
    expect(body).not.toHaveProperty("token");
  });

  it("requires delete confirmation before deleting", async () => {
    const fetchMock = vi.spyOn(global, "fetch").mockResolvedValue(new Response(null, { status: 204 }));

    render(<TokenManagementClient initialTokens={initialTokens} />);

    const row = screen.getByRole("row", { name: /ci scanner/i });
    await userEvent.click(within(row).getByRole("button", { name: /delete ci scanner/i }));

    expect(fetchMock).not.toHaveBeenCalled();
    expect(screen.getByText(/create a replacement/i)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /confirm delete ci scanner/i }));

    await waitFor(() => expect(screen.queryByText("CI scanner")).not.toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/tokens/token-1",
      expect.objectContaining({ method: "DELETE" })
    );
  });
});

it("links validation to fields and focuses the invalid field", async () => {
  render(<TokenManagementClient initialTokens={initialTokens} />);
  await userEvent.click(screen.getByRole("button", { name: "Issue token" }));
  expect(screen.getByLabelText("Label")).toHaveFocus();
  expect(screen.getByLabelText("Label")).toHaveAccessibleDescription("Label is required.");
  await userEvent.type(screen.getByLabelText("Label"), "New scanner");
  await userEvent.click(screen.getByRole("button", { name: "Issue token" }));
  expect(screen.getByRole("group", { name: "Scopes" })).toHaveAccessibleDescription("Choose at least one scope.");
  expect(screen.getByLabelText("scan:read")).toHaveFocus();
});

it("restores focus after editing and cancelling deletion", async () => {
  render(<TokenManagementClient initialTokens={initialTokens} />);
  const edit = screen.getByRole("button", { name: "Edit CI scanner" });
  await userEvent.click(edit);
  expect(screen.getByLabelText("Label")).toHaveFocus();
  await userEvent.type(screen.getByLabelText("Label"), " abandoned");
  await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(edit).toHaveFocus();
  expect(screen.getByLabelText("Label")).toHaveValue("");
  const remove = screen.getByRole("button", { name: "Delete CI scanner" });
  await userEvent.click(remove);
  expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
  await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(remove).toHaveFocus();
});

it("keeps edits after failure, blocks conflicting actions while pending, and reports success", async () => {
  let complete: ((response: Response) => void) | undefined;
  vi.spyOn(global, "fetch").mockImplementationOnce(() => new Promise<Response>((resolve) => { complete = resolve; }))
    .mockResolvedValueOnce(new Response(JSON.stringify({ ...initialTokens[0], label: "Updated" }), { status: 200 }));
  render(<TokenManagementClient initialTokens={initialTokens} />);
  await userEvent.click(screen.getByRole("button", { name: "Edit CI scanner" }));
  await userEvent.clear(screen.getByLabelText("Label"));
  await userEvent.type(screen.getByLabelText("Label"), "Updated");
  await userEvent.click(screen.getByRole("button", { name: "Save token" }));
  expect(screen.getByRole("button", { name: "Saving token…" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Delete CI scanner" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
  complete?.(new Response("{}", { status: 500 }));
  await screen.findByRole("alert");
  expect(screen.getByLabelText("Label")).toHaveValue("Updated");
  expect(screen.getByLabelText("scan:read")).toBeChecked();
  await userEvent.click(screen.getByRole("button", { name: "Save token" }));
  expect(await screen.findByRole("status")).toHaveTextContent("Token updated.");
  await waitFor(() => expect(screen.getByRole("button", { name: "Edit Updated" })).toHaveFocus());
});

it("retains a failed deletion for retry and focuses the form after success", async () => {
  vi.spyOn(global, "fetch").mockRejectedValueOnce(new Error("offline"))
    .mockResolvedValueOnce(new Response(null, { status: 204 }));
  render(<TokenManagementClient initialTokens={initialTokens} />);
  await userEvent.click(screen.getByRole("button", { name: "Delete CI scanner" }));
  await userEvent.click(screen.getByRole("button", { name: "Confirm delete CI scanner" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Delete failed");
  expect(screen.getByRole("button", { name: "Confirm delete CI scanner" })).toBeEnabled();
  await userEvent.click(screen.getByRole("button", { name: "Confirm delete CI scanner" }));
  expect(await screen.findByRole("status")).toHaveTextContent("Token deleted.");
  expect(screen.getByLabelText("Label")).toHaveFocus();
});
