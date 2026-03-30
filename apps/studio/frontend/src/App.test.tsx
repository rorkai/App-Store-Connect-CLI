import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";

const {
  mockListApps,
  mockCheckAuthStatus,
  mockBootstrap,
  mockGetSettings,
  mockSaveSettings,
  mockRunASCCommand,
  mockGetAppDetail,
  mockGetVersionMetadata,
  mockGetScreenshots,
  mockGetTestFlight,
  mockGetTestFlightTesters,
  mockGetPricingOverview,
  mockGetSubscriptions,
  mockGetFinanceRegions,
  mockGetOfferCodes,
} = vi.hoisted(() => ({
  mockListApps: vi.fn(),
  mockCheckAuthStatus: vi.fn(),
  mockBootstrap: vi.fn(),
  mockGetSettings: vi.fn(),
  mockSaveSettings: vi.fn(),
  mockRunASCCommand: vi.fn(),
  mockGetAppDetail: vi.fn(),
  mockGetVersionMetadata: vi.fn(),
  mockGetScreenshots: vi.fn(),
  mockGetTestFlight: vi.fn(),
  mockGetTestFlightTesters: vi.fn(),
  mockGetPricingOverview: vi.fn(),
  mockGetSubscriptions: vi.fn(),
  mockGetFinanceRegions: vi.fn(),
  mockGetOfferCodes: vi.fn(),
}));

// Mock the Wails bindings since they don't exist in test environment
vi.mock("../wailsjs/go/main/App", () => ({
  ListApps: mockListApps,
  CheckAuthStatus: mockCheckAuthStatus,
  Bootstrap: mockBootstrap,
  GetSettings: mockGetSettings,
  SaveSettings: mockSaveSettings,
  RunASCCommand: mockRunASCCommand,
  GetAppDetail: mockGetAppDetail,
  GetVersionMetadata: mockGetVersionMetadata,
  GetScreenshots: mockGetScreenshots,
  GetTestFlight: mockGetTestFlight,
  GetTestFlightTesters: mockGetTestFlightTesters,
  GetPricingOverview: mockGetPricingOverview,
  GetSubscriptions: mockGetSubscriptions,
  GetFinanceRegions: mockGetFinanceRegions,
  GetOfferCodes: mockGetOfferCodes,
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

import App, { insightsWeekStart } from "./App";

describe("App", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockListApps.mockResolvedValue({
      apps: [
        { id: "1", name: "Test App", subtitle: "A great app" },
      ],
    });
    mockCheckAuthStatus.mockResolvedValue({
      authenticated: true,
      storage: "System Keychain",
      profile: "default",
      rawOutput: "Credential storage: System Keychain\nActive profile: default",
    });
    mockBootstrap.mockResolvedValue({
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
    });
    mockGetSettings.mockResolvedValue({});
    mockSaveSettings.mockResolvedValue({});
    mockRunASCCommand.mockResolvedValue({ error: "", data: "{\"data\":[]}" });
    mockGetAppDetail.mockResolvedValue({
      id: "1",
      name: "Test App",
      subtitle: "A great app",
      bundleId: "com.example.test",
      sku: "TESTSKU",
      primaryLocale: "en-US",
      versions: [{ id: "version-1", platform: "IOS", version: "1.0", state: "READY_FOR_SALE" }],
    });
    mockGetVersionMetadata.mockResolvedValue({ localizations: [] });
    mockGetScreenshots.mockResolvedValue({ sets: [] });
    mockGetTestFlight.mockResolvedValue({ groups: [] });
    mockGetTestFlightTesters.mockResolvedValue({ testers: [] });
    mockGetPricingOverview.mockResolvedValue({
      availableInNewTerritories: false,
      currentPrice: "",
      currentProceeds: "",
      baseCurrency: "",
      territories: [],
      subscriptionPricing: [],
    });
    mockGetSubscriptions.mockResolvedValue({ subscriptions: [] });
    mockGetFinanceRegions.mockResolvedValue({ regions: [] });
    mockGetOfferCodes.mockResolvedValue({ offerCodes: [] });
  });

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

  it("shows a loading state in TestFlight while beta groups are still fetching", async () => {
    mockGetTestFlight.mockImplementation(
      () => new Promise(() => {})
    );

    render(<App />);

    await screen.findByText("Connected");

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "1" } });

    await screen.findByRole("button", { name: "TestFlight" });
    fireEvent.click(screen.getByRole("button", { name: "TestFlight" }));

    await waitFor(() => {
      expect(screen.getByText("Loading…")).toBeInTheDocument();
    });
    expect(screen.queryByText("No beta groups found.")).not.toBeInTheDocument();
  });

  it("ignores stale app detail responses after switching to another app", async () => {
    let resolveFirstApp: ((value: {
      id: string;
      name: string;
      subtitle: string;
      bundleId: string;
      sku: string;
      primaryLocale: string;
      versions: { id: string; platform: string; version: string; state: string }[];
    }) => void) | undefined;

    mockListApps.mockResolvedValue({
      apps: [
        { id: "1", name: "First App", subtitle: "One" },
        { id: "2", name: "Second App", subtitle: "Two" },
      ],
    });
    mockGetAppDetail.mockImplementation((appID: string) => {
      if (appID === "1") {
        return new Promise((resolve) => {
          resolveFirstApp = resolve;
        });
      }
      return Promise.resolve({
        id: "2",
        name: "Second App",
        subtitle: "Two",
        bundleId: "com.example.second",
        sku: "SECONDSKU",
        primaryLocale: "en-US",
        versions: [{ id: "version-2", platform: "IOS", version: "2.0", state: "READY_FOR_SALE" }],
      });
    });

    render(<App />);

    await screen.findByText("Connected");

    const picker = screen.getByRole("combobox");
    fireEvent.change(picker, { target: { value: "1" } });
    fireEvent.change(picker, { target: { value: "2" } });

    expect(await screen.findByText("Second App")).toBeInTheDocument();

    resolveFirstApp?.({
      id: "1",
      name: "First App",
      subtitle: "One",
      bundleId: "com.example.first",
      sku: "FIRSTSKU",
      primaryLocale: "en-US",
      versions: [{ id: "version-1", platform: "IOS", version: "1.0", state: "READY_FOR_SALE" }],
    });

    await waitFor(() => {
      expect(screen.getByText("Second App")).toBeInTheDocument();
    });
    expect(screen.queryByText("First App")).not.toBeInTheDocument();
  });

  it("includes required statuses when loading nominations", async () => {
    render(<App />);

    await screen.findByText("Connected");

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "1" } });
    await screen.findByText("Test App");

    await waitFor(() => {
      expect(mockRunASCCommand.mock.calls.map(([cmd]) => cmd)).toContain(
        "nominations list --app 1 --status DRAFT,SUBMITTED,ARCHIVED --output json",
      );
    });
  });

  it("fetches all bundle IDs with pagination enabled", async () => {
    render(<App />);

    await screen.findByText("Connected");

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "1" } });
    await screen.findByText("Test App");

    await waitFor(() => {
      expect(mockRunASCCommand.mock.calls.map(([cmd]) => cmd)).toContain(
        "bundle-ids list --paginate --output json",
      );
    });
  });

  it("fetches all certificates and profiles with pagination enabled", async () => {
    render(<App />);

    await screen.findByText("Connected");

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "1" } });
    await screen.findByText("Test App");

    await waitFor(() => {
      const commands = mockRunASCCommand.mock.calls.map(([cmd]) => cmd);
      expect(commands).toContain("certificates list --paginate --output json");
      expect(commands).toContain("profiles list --paginate --output json");
    });
  });

  it("preserves agent env when saving settings", async () => {
    mockBootstrap.mockResolvedValue({
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
        agentCommand: "codex",
        agentArgs: ["agent", "acp"],
        agentEnv: { OPENAI_API_KEY: "secret" },
        preferBundledASC: true,
        systemASCPath: "",
        workspaceRoot: "",
        theme: "system",
        windowMaterial: "translucent",
        showCommandPreviews: true,
      },
      presets: [],
      threads: [],
      approvals: [],
    });

    render(<App />);

    await screen.findByText("Connected");
    fireEvent.click(screen.getByRole("button", { name: /settings/i }));
    fireEvent.click(screen.getByRole("button", { name: /save settings/i }));

    await waitFor(() => {
      expect(mockSaveSettings).toHaveBeenCalled();
    });

    expect(mockSaveSettings.mock.calls.at(-1)?.[0]).toMatchObject({
      agentEnv: { OPENAI_API_KEY: "secret" },
    });
  });

  it("honors system dark mode when theme is set to system", async () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn().mockImplementation((query: string) => ({
        matches: query === "(prefers-color-scheme: dark)",
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    );

    mockBootstrap.mockResolvedValue({
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
        theme: "system",
        windowMaterial: "translucent",
        showCommandPreviews: true,
      },
      presets: [],
      threads: [],
      approvals: [],
    });

    render(<App />);

    await screen.findByText("Connected");

    expect(document.querySelector(".studio-shell")).toHaveAttribute("data-theme", "dark");
  });

  it("uses the current week's Monday for weekly insights on Sundays", () => {
    expect(insightsWeekStart(new Date("2026-03-29T12:00:00Z"))).toBe("2026-03-23");
  });

  it("ignores stale tester responses after switching groups", async () => {
    let resolveFirstGroup: ((value: { testers: { email: string; firstName: string; lastName: string; inviteType: string; state: string }[] }) => void) | undefined;

    mockGetTestFlight.mockResolvedValue({
      groups: [
        { id: "group-1", name: "Internal", isInternal: true, publicLink: "", feedbackEnabled: false, createdDate: "2026-03-30T00:00:00Z", testerCount: 1 },
        { id: "group-2", name: "External", isInternal: false, publicLink: "", feedbackEnabled: true, createdDate: "2026-03-30T00:00:00Z", testerCount: 1 },
      ],
    });
    mockGetTestFlightTesters.mockImplementation((groupID: string) => {
      if (groupID === "group-1") {
        return new Promise((resolve) => {
          resolveFirstGroup = resolve;
        });
      }
      return Promise.resolve({
        testers: [{ email: "second@example.com", firstName: "Second", lastName: "Tester", inviteType: "EMAIL", state: "ACCEPTED" }],
      });
    });

    render(<App />);

    await screen.findByText("Connected");
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "1" } });
    fireEvent.click(await screen.findByRole("button", { name: "TestFlight" }));

    fireEvent.click((await screen.findAllByText("Internal"))[0].closest("tr")!);
    fireEvent.click(await screen.findByRole("button", { name: "← TestFlight" }));
    fireEvent.click((await screen.findAllByText("External"))[0].closest("tr")!);

    expect(await screen.findByText("second@example.com")).toBeInTheDocument();

    resolveFirstGroup?.({
      testers: [{ email: "first@example.com", firstName: "First", lastName: "Tester", inviteType: "EMAIL", state: "INVITED" }],
    });

    await waitFor(() => {
      expect(screen.getByText("second@example.com")).toBeInTheDocument();
    });
    expect(screen.queryByText("first@example.com")).not.toBeInTheDocument();
  });
});
