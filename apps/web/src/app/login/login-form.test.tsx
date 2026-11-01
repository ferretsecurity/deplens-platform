import React from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import LoginPage from "./page";
import { LoginForm } from "@/components/login-form";

vi.mock("@/lib/auth", () => ({
  login: vi.fn().mockResolvedValue(undefined)
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    replace: vi.fn(),
    refresh: vi.fn()
  })
}));

describe("LoginForm", () => {
  it("shows validation errors and submits successfully", async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();

    render(<LoginForm onSuccess={onSuccess} />);

    await user.click(screen.getByRole("button", { name: /sign in/i }));
    expect(await screen.findByText(/enter a valid email address/i)).toBeInTheDocument();
    expect(screen.getByText(/password is required/i)).toBeInTheDocument();

    await user.type(screen.getByLabelText(/email/i), "admin@example.com");
    await user.type(screen.getByLabelText(/password/i), "change-me-now");
    await user.click(screen.getByRole("button", { name: /sign in/i }));

    expect(onSuccess).toHaveBeenCalledOnce();
  });
});


describe("development login prefill", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("prefills the configured credentials only in opted-in development", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("DEV_LOGIN_PREFILL", "true");
    vi.stubEnv("BOOTSTRAP_OWNER_EMAIL", "dev@example.test");
    vi.stubEnv("BOOTSTRAP_OWNER_PASSWORD", "test-only-password");
    render(<LoginPage />);
    expect(screen.getByLabelText(/email/i)).toHaveValue("dev@example.test");
    expect(screen.getByLabelText(/password/i)).toHaveValue("test-only-password");
  });

  it.each(["production", "test"])("keeps credentials out of the %s page", (mode) => {
    vi.stubEnv("NODE_ENV", mode);
    vi.stubEnv("DEV_LOGIN_PREFILL", "true");
    vi.stubEnv("BOOTSTRAP_OWNER_EMAIL", "dev@example.test");
    vi.stubEnv("BOOTSTRAP_OWNER_PASSWORD", "test-only-password");
    const page = LoginPage();
    expect(page.props.children.props.defaultValues).toBeUndefined();
    render(page);
    expect(screen.getByLabelText(/email/i)).toHaveValue("");
    expect(screen.getByLabelText(/password/i)).toHaveValue("");
  });
});
