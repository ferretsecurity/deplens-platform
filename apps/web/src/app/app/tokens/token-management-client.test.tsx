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
