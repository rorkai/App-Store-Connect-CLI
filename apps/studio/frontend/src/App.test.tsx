import { fireEvent, render, screen } from "@testing-library/react";
import { vi } from "vitest";

// Mock the Wails bindings since they don't exist in test environment
vi.mock("../wailsjs/go/main/App", () => ({
  CheckAuthStatus: vi.fn().mockResolvedValue({
    authenticated: true,
    storage: "System Keychain",
    profile: "default",
    rawOutput: "Credential storage: System Keychain\nActive profile: default",
  }),
  Bootstrap: vi.fn().mockResolvedValue({
    appName: "ASC Studio",
    environment: {
      configPath: "/Users/test/.asc/config.json",
      configPresent: true,
      defaultAppId: "123456",
      keychainAvailable: true,
      keychainBypassed: false,
      workflowPath: "",
    },
    settings: {
      preferredPreset: "codex",
      agentCommand: "",
      agentArgs: [],
      agentEnv: {},
      preferBundledASC: true,
      systemASCPath: "",
      workspaceRoot: "",
      theme: "glass-light",
      windowMaterial: "translucent",
      showCommandPreviews: true,
    },
    presets: [],
    threads: [],
    approvals: [],
  }),
  GetSettings: vi.fn().mockResolvedValue({}),
  SaveSettings: vi.fn().mockResolvedValue({}),
}));

vi.mock("../wailsjs/go/models", () => ({
  environment: { Snapshot: class {} },
  settings: {
    StudioSettings: class {
      constructor(source: Record<string, unknown> = {}) {
        Object.assign(this, source);
      }
    },
    ProviderPreset: class {},
  },
}));

import App from "./App";

describe("App", () => {
  it("renders and calls Bootstrap on mount", async () => {
    render(<App />);

    // After bootstrap resolves, should show "Connected" status
    expect(await screen.findByText("Connected")).toBeInTheDocument();
    expect(screen.getByText("System Keychain")).toBeInTheDocument();
  });

  it("navigates to settings view", async () => {
    render(<App />);

    await screen.findByText("Connected");

    fireEvent.click(screen.getByRole("button", { name: /settings/i }));

    expect(screen.getByText("Authentication")).toBeInTheDocument();
    expect(screen.getByText("ACP Provider")).toBeInTheDocument();
  });

  it("sends a chat message and expands the dock", async () => {
    render(<App />);

    await screen.findByText("Connected");

    const textarea = screen.getByLabelText("Chat prompt");
    fireEvent.change(textarea, { target: { value: "list builds" } });
    fireEvent.submit(textarea.closest("form")!);

    expect(screen.getByText("list builds")).toBeInTheDocument();
    expect(screen.getByText("ACP Chat")).toBeInTheDocument();
  });

  it("collapses the dock when chevron is clicked", async () => {
    render(<App />);

    await screen.findByText("Connected");

    const textarea = screen.getByLabelText("Chat prompt");
    fireEvent.change(textarea, { target: { value: "test" } });
    fireEvent.submit(textarea.closest("form")!);

    expect(screen.getByText("ACP Chat")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Collapse chat"));

    expect(screen.queryByText("ACP Chat")).not.toBeInTheDocument();
  });
});
