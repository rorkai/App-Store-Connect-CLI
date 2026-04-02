import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  mockGetFeedback,
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
  mockGetFeedback: vi.fn(),
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
  GetFeedback: mockGetFeedback,
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
import { sectionCommands } from "./constants";

async function pickApp(name: string) {
  fireEvent.change(screen.getByLabelText("Search apps"), { target: { value: name } });
  fireEvent.click(await screen.findByRole("option", { name: new RegExp(name, "i") }));
}

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
    mockGetFeedback.mockResolvedValue({ feedback: [], total: 0 });
  });

  it("renders and calls Bootstrap on mount", async () => {
    render(<App />);

    // After bootstrap resolves, should show title and connected dot with hover info
    expect(await screen.findByRole("img", { name: /Connected via System Keychain/i })).toBeInTheDocument();
    expect(screen.getByText("ASC Studio")).toBeInTheDocument();
  });

  it("surfaces ListApps backend errors during bootstrap", async () => {
    mockListApps.mockResolvedValue({
      error: "auth expired",
      apps: [],
    });

    render(<App />);

    expect(await screen.findByText("Failed to load apps")).toBeInTheDocument();
    expect(screen.getAllByText("auth expired").length).toBeGreaterThan(0);
    expect(screen.queryByText("No apps found")).not.toBeInTheDocument();
  });

  it("navigates to settings view", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });

    fireEvent.click(screen.getByRole("button", { name: /settings/i }));

    expect(await screen.findByText("Authentication")).toBeInTheDocument();
    expect(screen.getByText("ACP Provider")).toBeInTheDocument();
  });

  it("sends a chat message and expands the dock", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });

    const textarea = screen.getByLabelText("Chat prompt");
    fireEvent.change(textarea, { target: { value: "list builds" } });
    fireEvent.submit(textarea.closest("form")!);

    expect(screen.getByText("list builds")).toBeInTheDocument();
    expect(screen.getByText("ACP Chat")).toBeInTheDocument();
  });

  it("collapses the dock when chevron is clicked", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });

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

    await screen.findByRole("img", { name: /Connected/i });

    await pickApp("Test App");

    await screen.findByRole("button", { name: "Groups" });
    fireEvent.click(screen.getByRole("button", { name: "Groups" }));

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

    await screen.findByRole("img", { name: /Connected/i });

    await pickApp("First App");
    await pickApp("Second App");

    expect(await screen.findByText("com.example.second")).toBeInTheDocument();

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
      expect(screen.getByText("com.example.second")).toBeInTheDocument();
    });
    expect(screen.queryByText("com.example.first")).not.toBeInTheDocument();
  });

  it("loads app-scoped data lazily instead of fan-out prefetching on selection", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    await waitFor(() => {
      expect(mockGetAppDetail).toHaveBeenCalledWith("1");
    });

    const commands = mockRunASCCommand.mock.calls.map(([cmd]) => cmd);
    expect(commands).not.toContain("status --app '1' --output json");
    expect(commands).not.toContain("reviews list --app '1' --limit 25 --output json");
    expect(commands).not.toContain("builds list --app '1' --limit 20 --output json");
    expect(mockGetPricingOverview).not.toHaveBeenCalled();
    expect(mockGetSubscriptions).not.toHaveBeenCalled();
    expect(mockGetFinanceRegions).not.toHaveBeenCalled();
    expect(mockGetFeedback).not.toHaveBeenCalled();
    expect(mockGetOfferCodes).not.toHaveBeenCalled();
    expect(mockGetTestFlight).not.toHaveBeenCalled();

    fireEvent.click(await screen.findByRole("button", { name: "Status" }));
    await waitFor(() => {
      expect(mockRunASCCommand).toHaveBeenCalledWith("status --app '1' --output json");
    });

    fireEvent.click(await screen.findByRole("button", { name: "Pricing and Availability" }));
    await waitFor(() => {
      expect(mockGetPricingOverview).toHaveBeenCalledWith("1");
    });

    fireEvent.click(await screen.findByRole("button", { name: "Groups" }));
    await waitFor(() => {
      expect(mockGetTestFlight).toHaveBeenCalledWith("1");
    });
  });

  it("switches back to overview before loading a different app", async () => {
    mockListApps.mockResolvedValue({
      apps: [
        { id: "1", name: "First App", subtitle: "One" },
        { id: "2", name: "Second App", subtitle: "Two" },
      ],
    });
    mockGetAppDetail.mockImplementation((appID: string) => Promise.resolve({
      id: appID,
      name: appID === "1" ? "First App" : "Second App",
      subtitle: appID === "1" ? "One" : "Two",
      bundleId: appID === "1" ? "com.example.first" : "com.example.second",
      sku: appID === "1" ? "FIRSTSKU" : "SECONDSKU",
      primaryLocale: "en-US",
      versions: [{ id: `version-${appID}`, platform: "IOS", version: appID === "1" ? "1.0" : "2.0", state: "READY_FOR_SALE" }],
    }));

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("First App");

    fireEvent.click(await screen.findByRole("button", { name: "Insights" }));
    await waitFor(() => {
      expect(mockRunASCCommand).toHaveBeenCalledWith(
        expect.stringContaining("insights weekly --app '1'"),
      );
    });

    mockRunASCCommand.mockClear();

    await pickApp("Second App");

    expect(await screen.findByText("com.example.second")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "App Information" })).toHaveAttribute("aria-current", "page");
    expect(
      mockRunASCCommand.mock.calls.map(([cmd]) => cmd).filter((cmd) => cmd.includes("insights weekly")),
    ).toHaveLength(0);
  });

  it("renders an overview error state when app detail loading fails", async () => {
    mockGetAppDetail.mockResolvedValue({
      id: "1",
      name: "",
      subtitle: "",
      bundleId: "",
      sku: "",
      primaryLocale: "",
      versions: [],
      error: "detail failed",
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    expect(await screen.findByText("Overview unavailable")).toBeInTheDocument();
    expect(screen.getByText("detail failed")).toBeInTheDocument();
    expect(screen.queryByText("General")).not.toBeInTheDocument();
  });

  it("shows screenshot fetch errors instead of an empty state", async () => {
    mockGetVersionMetadata.mockResolvedValue({
      localizations: [
        {
          localizationId: "loc-1",
          locale: "en-US",
          description: "Localized description",
          keywords: "",
          whatsNew: "",
          promotionalText: "",
          supportUrl: "",
          marketingUrl: "",
        },
      ],
    });
    mockGetScreenshots.mockResolvedValue({
      error: "screenshots unavailable",
      sets: [],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    expect(await screen.findByText("screenshots unavailable")).toBeInTheDocument();
    expect(screen.queryByText("No screenshots found. Select an app with screenshots or change locale.")).not.toBeInTheDocument();

    fireEvent.click(await screen.findByRole("button", { name: "Screenshots" }));

    expect(await screen.findByText("screenshots unavailable")).toBeInTheDocument();
    expect(screen.queryByText("No screenshots found. Select an app with screenshots or change locale.")).not.toBeInTheDocument();
  });

  it("shows partial subscription data alongside backend warnings", async () => {
    mockGetSubscriptions.mockResolvedValue({
      error: "failed to load subscriptions for Secondary Group: timeout",
      subscriptions: [
        {
          id: "sub-1",
          groupName: "Main Group",
          name: "Pro Monthly",
          productId: "pro.monthly",
          state: "READY_FOR_SUBMISSION",
          subscriptionPeriod: "ONE_MONTH",
          reviewNote: "",
          groupLevel: 1,
        },
      ],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Subscriptions" }));

    expect(await screen.findByText("failed to load subscriptions for Secondary Group: timeout")).toBeInTheDocument();
    expect(screen.getByText("Pro Monthly")).toBeInTheDocument();
  });

  it("surfaces version metadata errors without hiding overview details", async () => {
    mockGetVersionMetadata.mockResolvedValue({
      error: "metadata unavailable",
      localizations: [],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    expect(await screen.findByText("General")).toBeInTheDocument();
    expect(screen.getByText("metadata unavailable")).toBeInTheDocument();
    expect(screen.queryByText("Overview unavailable")).not.toBeInTheDocument();
  });

  it("renders version badges safely when a version state is missing", async () => {
    mockGetAppDetail.mockResolvedValue({
      id: "1",
      name: "Test App",
      subtitle: "A great app",
      bundleId: "com.example.test",
      sku: "TESTSKU",
      primaryLocale: "en-US",
      versions: [{ id: "version-1", platform: "IOS", version: "1.0", state: null as unknown as string }],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    expect(await screen.findByText("com.example.test")).toBeInTheDocument();
    expect(screen.getByText("1.0 Unknown State")).toBeInTheDocument();
    expect(screen.queryByText("Something went wrong")).not.toBeInTheDocument();
  });

  it("includes required statuses when loading nominations", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });

    await pickApp("Test App");
    await screen.findByText("Test App");

    // Nominations is standalone (no APP_ID) — loads lazily when clicked
    fireEvent.click(await screen.findByRole("button", { name: "Nominations" }));

    await waitFor(() => {
      expect(mockRunASCCommand.mock.calls.map(([cmd]) => cmd)).toContain(
        "nominations list --status DRAFT --output json",
      );
    });
  });

  it("fetches all bundle IDs with pagination enabled", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));

    await waitFor(() => {
      expect(mockRunASCCommand.mock.calls.map(([cmd]) => cmd)).toContain(
        "bundle-ids list --paginate --output json",
      );
    });
  });

  it("fetches all certificates and profiles with pagination enabled", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));
    fireEvent.click(await screen.findByRole("button", { name: "Certificates" }));
    fireEvent.click(await screen.findByRole("button", { name: "Profiles" }));

    await waitFor(() => {
      const commands = mockRunASCCommand.mock.calls.map(([cmd]) => cmd);
      expect(commands).toContain("certificates list --paginate --output json");
      expect(commands).toContain("profiles list --paginate --output json");
    });
  });

  it("filters the app picker with search", async () => {
    mockListApps.mockResolvedValue({
      apps: [
        { id: "1", name: "Test App", subtitle: "A great app" },
        { id: "2", name: "Music Box", subtitle: "Audio tools" },
      ],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.change(screen.getByLabelText("Search apps"), { target: { value: "music" } });

    expect(screen.getByRole("option", { name: /Music Box/i })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: /Test App/i })).not.toBeInTheDocument();
  });

  it("loads signing sections without requiring an app selection", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });

    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));

    await waitFor(() => {
      expect(mockRunASCCommand.mock.calls.map(([cmd]) => cmd)).toContain(
        "bundle-ids list --paginate --output json",
      );
    });

    expect(screen.queryByText("Select an App")).not.toBeInTheDocument();
  });

  it("retries standalone sections after a cached transient error when revisiting them", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve(
          mockRunASCCommand.mock.calls.filter(([calledCmd]) => calledCmd === cmd).length === 1
            ? { error: "temporary failure", data: "" }
            : {
                error: "",
                data: JSON.stringify({
                  data: [
                    { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.recovered", platform: "IOS", seedId: "AAA" } },
                  ],
                }),
              },
        );
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));

    expect(await screen.findByText("temporary failure")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Team" }));
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));

    expect(await screen.findByText("com.example.recovered")).toBeInTheDocument();
    expect(
      mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "bundle-ids list --paginate --output json"),
    ).toHaveLength(2);
  });

  it("renders top-level account status checks in the team scope", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "account status --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            summary: { health: "warn", nextAction: "Fix credentials" },
            checks: [
              { name: "authentication", status: "warn", message: "auth doctor found 1 warning(s)" },
              { name: "api_access", status: "ok", message: "able to read apps list" },
            ],
            generatedAt: "2026-03-31T00:00:00Z",
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });

    fireEvent.click(screen.getByRole("tab", { name: "Team" }));
    fireEvent.click(screen.getByRole("button", { name: "Account" }));

    expect(await screen.findByText("authentication")).toBeInTheDocument();
    expect(screen.getByText("auth doctor found 1 warning(s)")).toBeInTheDocument();
    expect(screen.getByText("able to read apps list")).toBeInTheDocument();
  });

  it("does not refetch standalone sections when switching apps", async () => {
    mockListApps.mockResolvedValue({
      apps: [
        { id: "1", name: "First App", subtitle: "One" },
        { id: "2", name: "Second App", subtitle: "Two" },
      ],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));

    await waitFor(() => {
      expect(
        mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "bundle-ids list --paginate --output json"),
      ).toHaveLength(1);
    });

    fireEvent.click(screen.getByRole("tab", { name: "App" }));
    await pickApp("First App");
    await pickApp("Second App");

    expect(
      mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "bundle-ids list --paginate --output json"),
    ).toHaveLength(1);
  });

  it("refreshes bootstrap data and the visible standalone section even when an app is selected", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve(
          mockRunASCCommand.mock.calls.filter(([calledCmd]) => calledCmd === cmd).length === 1
            ? {
                error: "",
                data: JSON.stringify({
                  data: [
                    { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.initial", platform: "IOS", seedId: "AAA" } },
                  ],
                }),
              }
            : {
                error: "",
                data: JSON.stringify({
                  data: [
                    { id: "bundle-2", type: "bundleIds", attributes: { identifier: "com.example.refreshed", platform: "IOS", seedId: "BBB" } },
                  ],
                }),
              },
        );
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));

    expect(await screen.findByText("com.example.initial")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Refresh/i }));

    expect(await screen.findByText("com.example.refreshed")).toBeInTheDocument();
    expect(mockBootstrap).toHaveBeenCalledTimes(2);
    expect(mockCheckAuthStatus).toHaveBeenCalledTimes(2);
    expect(mockListApps).toHaveBeenCalledTimes(2);
    expect(
      mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "bundle-ids list --paginate --output json"),
    ).toHaveLength(2);
  });

  it("keeps the latest app selection when refresh resolves after another app is chosen", async () => {
    let resolveRefreshBootstrap: ((value: {
      appName: string;
      environment: {
        configPath: string;
        configPresent: boolean;
        defaultAppId: string;
        keychainAvailable: boolean;
        keychainBypassed: boolean;
        workflowPath: string;
      };
      settings: {
        preferredPreset: string;
        agentCommand: string;
        agentArgs: string[];
        agentEnv: Record<string, string>;
        preferBundledASC: boolean;
        systemASCPath: string;
        workspaceRoot: string;
        theme: string;
        windowMaterial: string;
        showCommandPreviews: boolean;
      };
      presets: unknown[];
      threads: unknown[];
      approvals: unknown[];
    }) => void) | undefined;

    const bootstrapPayload = {
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
    };

    mockListApps.mockResolvedValue({
      apps: [
        { id: "1", name: "First App", subtitle: "One" },
        { id: "2", name: "Second App", subtitle: "Two" },
      ],
    });
    mockBootstrap
      .mockResolvedValueOnce(bootstrapPayload)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveRefreshBootstrap = resolve;
          }),
      );
    mockGetAppDetail.mockImplementation((appID: string) =>
      Promise.resolve({
        id: appID,
        name: appID === "1" ? "First App" : "Second App",
        subtitle: appID === "1" ? "One" : "Two",
        bundleId: appID === "1" ? "com.example.first" : "com.example.second",
        sku: appID === "1" ? "FIRSTSKU" : "SECONDSKU",
        primaryLocale: "en-US",
        versions: [{ id: `version-${appID}`, platform: "IOS", version: "1.0", state: "READY_FOR_SALE" }],
      }),
    );

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("First App");
    expect(await screen.findByText("com.example.first")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Refresh/i }));
    await waitFor(() => {
      expect(mockBootstrap).toHaveBeenCalledTimes(2);
    });

    await pickApp("Second App");
    expect(screen.getByRole("option", { name: /Second App/i })).toHaveAttribute("aria-selected", "true");

    await act(async () => {
      resolveRefreshBootstrap?.(bootstrapPayload);
    });

    await waitFor(() => {
      expect(mockGetAppDetail.mock.calls.at(-1)?.[0]).toBe("2");
    });

    expect(await screen.findByText("com.example.second")).toBeInTheDocument();
    expect(mockGetAppDetail.mock.calls.filter(([appID]) => appID === "1")).toHaveLength(1);
    expect(screen.getByText("com.example.second")).toBeInTheDocument();
    expect(screen.queryByText("com.example.first")).not.toBeInTheDocument();
  });

  it("quotes app IDs when running app-scoped commands", async () => {
    mockListApps.mockResolvedValue({
      apps: [
        { id: "1 2", name: "Quoted App", subtitle: "Spacing" },
      ],
    });
    mockGetAppDetail.mockResolvedValue({
      id: "1 2",
      name: "Quoted App",
      subtitle: "Spacing",
      bundleId: "com.example.quoted",
      sku: "QUOTEDSKU",
      primaryLocale: "en-US",
      versions: [{ id: "version-q", platform: "IOS", version: "1.0", state: "READY_FOR_SALE" }],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Quoted App");

    fireEvent.click(await screen.findByRole("button", { name: "Status" }));
    fireEvent.click(await screen.findByRole("button", { name: "Ratings and Reviews" }));

    await waitFor(() => {
      const commands = mockRunASCCommand.mock.calls.map(([cmd]) => cmd);
      expect(commands).toContain("status --app '1 2' --output json");
      expect(commands).toContain("reviews list --app '1 2' --limit 25 --output json");
    });
  });

  it("does not rerun cached app-scoped table commands when revisiting a loaded section", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    fireEvent.click(await screen.findByRole("button", { name: "Builds" }));

    await waitFor(() => {
      expect(
        mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "builds list --app '1' --limit 20 --output json"),
      ).toHaveLength(1);
    });

    fireEvent.click(await screen.findByRole("button", { name: "Status" }));
    fireEvent.click(await screen.findByRole("button", { name: "Builds" }));

    await waitFor(() => {
      expect(
        mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "builds list --app '1' --limit 20 --output json"),
      ).toHaveLength(1);
    });
  });

  it("uses only supported app-scoped section commands", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Encryption" }));

    await waitFor(() => {
      expect(mockRunASCCommand).toHaveBeenCalledWith(
        "encryption declarations list --app '1' --output json",
      );
    });

    const commands = mockRunASCCommand.mock.calls.map(([cmd]) => cmd);
    expect(commands).not.toContain("encryption list --app '1' --output json");
    expect(commands).not.toContain("localizations preview-sets list --app '1' --output json");
    expect(sectionCommands["video-previews"]).toBeUndefined();
  });

  it("sorts bundle IDs by platform from the header control", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.mac", platform: "MAC_OS", seedId: "AAA" } },
              { id: "bundle-2", type: "bundleIds", attributes: { identifier: "com.example.ios", platform: "IOS", seedId: "BBB" } },
              { id: "bundle-3", type: "bundleIds", attributes: { identifier: "com.example.universal", platform: "UNIVERSAL", seedId: "CCC" } },
            ],
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));

    expect(await screen.findByText("com.example.ios")).toBeInTheDocument();

    const rowsBefore = screen.getAllByRole("row").slice(1);
    expect(rowsBefore[0]).toHaveTextContent("com.example.ios");
    expect(rowsBefore[1]).toHaveTextContent("com.example.universal");
    expect(rowsBefore[2]).toHaveTextContent("com.example.mac");

    fireEvent.click(screen.getByRole("button", { name: /Platform/i }));

    await waitFor(() => {
      const rowsAfter = screen.getAllByRole("row").slice(1);
      expect(rowsAfter[0]).toHaveTextContent("com.example.mac");
      expect(rowsAfter[1]).toHaveTextContent("com.example.universal");
      expect(rowsAfter[2]).toHaveTextContent("com.example.ios");
    });
  });

  it("filters standalone signing lists with search", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.alpha", platform: "IOS", seedId: "AAA" } },
              { id: "bundle-2", type: "bundleIds", attributes: { identifier: "com.example.beta", platform: "MAC_OS", seedId: "BBB" } },
            ],
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));
    await screen.findByText("com.example.alpha");

    fireEvent.change(screen.getByLabelText("Bundle IDs search"), { target: { value: "beta" } });

    expect(screen.getByText("com.example.beta")).toBeInTheDocument();
    expect(screen.queryByText("com.example.alpha")).not.toBeInTheDocument();
  });

  it("creates a bundle ID from the signing sheet", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.existing", platform: "IOS", seedId: "AAA" } },
            ],
          }),
        });
      }
      if (cmd === "bundle-ids create --identifier 'com.example.newapp' --name 'New App' --platform 'MAC_OS' --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: {
              id: "bundle-2",
              type: "bundleIds",
              attributes: { identifier: "com.example.newapp", name: "New App", platform: "IOS", seedId: "BBB" },
            },
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));
    await screen.findByText("com.example.existing");

    fireEvent.click(screen.getByRole("button", { name: /New Bundle ID/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "New App" } });
    fireEvent.change(screen.getByLabelText("Identifier"), { target: { value: "com.example.newapp" } });
    fireEvent.change(screen.getByLabelText("Platform"), { target: { value: "MAC_OS" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(mockRunASCCommand).toHaveBeenCalledWith(
        "bundle-ids create --identifier 'com.example.newapp' --name 'New App' --platform 'MAC_OS' --output json",
      );
    });
  });

  it("ignores stale bundle ID create completions after the sheet is reopened", async () => {
    let resolveCreate: ((value: { error: string; data: string }) => void) | undefined;

    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.existing", platform: "IOS", seedId: "AAA" } },
            ],
          }),
        });
      }
      if (cmd === "bundle-ids create --identifier 'com.example.newapp' --name 'New App' --platform 'IOS' --output json") {
        return new Promise((resolve) => {
          resolveCreate = resolve as (value: { error: string; data: string }) => void;
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));
    await screen.findByText("com.example.existing");

    fireEvent.click(screen.getByRole("button", { name: /New Bundle ID/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "New App" } });
    fireEvent.change(screen.getByLabelText("Identifier"), { target: { value: "com.example.newapp" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: /Create Bundle ID/i })).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /New Bundle ID/i }));
    expect(screen.getByRole("dialog", { name: /Create Bundle ID/i })).toBeInTheDocument();

    await act(async () => {
      resolveCreate?.({
        error: "",
        data: JSON.stringify({
          data: {
            id: "bundle-2",
            type: "bundleIds",
            attributes: { identifier: "com.example.newapp", name: "New App", platform: "IOS", seedId: "BBB" },
          },
        }),
      });
    });

    await waitFor(() => {
      expect(screen.getByRole("dialog", { name: /Create Bundle ID/i })).toBeInTheDocument();
    });

    expect(
      mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "bundle-ids list --paginate --output json"),
    ).toHaveLength(2);
  });

  it("registers a device from the team devices sheet", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "devices list --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              { id: "device-1", type: "devices", attributes: { name: "Existing iPhone", udid: "EXISTING-UDID", platform: "IOS", status: "ENABLED" } },
            ],
          }),
        });
      }
      if (cmd === "devices register --name 'QA iPhone' --udid 'NEW-UDID-123' --platform 'MAC_OS' --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: {
              id: "device-2",
              type: "devices",
              attributes: { name: "QA iPhone", udid: "NEW-UDID-123", platform: "IOS", status: "ENABLED" },
            },
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Team" }));
    fireEvent.click(await screen.findByRole("button", { name: "Devices" }));
    await screen.findByText("Existing iPhone");

    fireEvent.click(screen.getByRole("button", { name: /New Device/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "QA iPhone" } });
    fireEvent.change(screen.getByLabelText("UDID"), { target: { value: "NEW-UDID-123" } });
    fireEvent.change(screen.getByLabelText("Platform"), { target: { value: "MAC_OS" } });
    fireEvent.click(screen.getByRole("button", { name: "Register" }));

    await waitFor(() => {
      expect(mockRunASCCommand).toHaveBeenCalledWith(
        "devices register --name 'QA iPhone' --udid 'NEW-UDID-123' --platform 'MAC_OS' --output json",
      );
    });
  });

  it("ignores stale device registration completions after the sheet is reopened", async () => {
    let resolveRegister: ((value: { error: string; data: string }) => void) | undefined;

    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "devices list --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              {
                id: "device-1",
                type: "devices",
                attributes: {
                  name: "Existing iPhone",
                  udid: "EXISTING-UDID",
                  platform: "IOS",
                  status: "ENABLED",
                },
              },
            ],
          }),
        });
      }
      if (cmd === "devices register --name 'QA iPhone' --udid 'NEW-UDID-123' --platform 'IOS' --output json") {
        return new Promise((resolve) => {
          resolveRegister = resolve as (value: { error: string; data: string }) => void;
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Team" }));
    fireEvent.click(await screen.findByRole("button", { name: "Devices" }));
    await screen.findByText("Existing iPhone");

    fireEvent.click(screen.getByRole("button", { name: /New Device/i }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "QA iPhone" } });
    fireEvent.change(screen.getByLabelText("UDID"), { target: { value: "NEW-UDID-123" } });
    fireEvent.click(screen.getByRole("button", { name: "Register" }));

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: /Register Device/i })).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /New Device/i }));
    expect(screen.getByRole("dialog", { name: /Register Device/i })).toBeInTheDocument();

    await act(async () => {
      resolveRegister?.({
        error: "",
        data: JSON.stringify({
          data: {
            id: "device-2",
            type: "devices",
            attributes: {
              name: "QA iPhone",
              udid: "NEW-UDID-123",
              platform: "IOS",
              status: "ENABLED",
            },
          },
        }),
      });
    });

    await waitFor(() => {
      expect(screen.getByRole("dialog", { name: /Register Device/i })).toBeInTheDocument();
    });

    expect(
      mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "devices list --output json"),
    ).toHaveLength(2);
  });

  it("closes the bundle ID sheet when escape is pressed inside the dialog", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.existing", platform: "IOS", seedId: "AAA" } },
            ],
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));
    fireEvent.click(await screen.findByRole("button", { name: /New Bundle ID/i }));

    const nameInput = screen.getByLabelText("Name");
    nameInput.focus();
    fireEvent.keyDown(nameInput, { key: "Escape" });

    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: /Create Bundle ID/i })).not.toBeInTheDocument();
    });
  });

  it("wraps focus within the bundle ID sheet without a global keydown listener", async () => {
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "bundle-ids list --paginate --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            data: [
              { id: "bundle-1", type: "bundleIds", attributes: { identifier: "com.example.existing", platform: "IOS", seedId: "AAA" } },
            ],
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    fireEvent.click(screen.getByRole("tab", { name: "Signing" }));
    fireEvent.click(await screen.findByRole("button", { name: /New Bundle ID/i }));

    const closeButton = screen.getByRole("button", { name: /Close create bundle ID sheet/i });
    const createButton = screen.getByRole("button", { name: "Create" });

    createButton.focus();
    fireEvent.keyDown(createButton, { key: "Tab" });

    expect(closeButton).toHaveFocus();
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

    await screen.findByRole("img", { name: /Connected/i });
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

    await screen.findByRole("img", { name: /Connected/i });

    expect(document.querySelector(".studio-shell")).toHaveAttribute("data-theme", "dark");
  });

  it("updates the shell theme when the system theme changes", async () => {
    const listeners = new Set<() => void>();
    let matches = false;

    vi.stubGlobal(
      "matchMedia",
      vi.fn().mockImplementation((query: string) => ({
        get matches() {
          return matches;
        },
        media: query,
        onchange: null,
        addListener: vi.fn((listener: () => void) => listeners.add(listener)),
        removeListener: vi.fn((listener: () => void) => listeners.delete(listener)),
        addEventListener: vi.fn((_event: string, listener: () => void) => listeners.add(listener)),
        removeEventListener: vi.fn((_event: string, listener: () => void) => listeners.delete(listener)),
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

    await screen.findByRole("img", { name: /Connected/i });
    expect(document.querySelector(".studio-shell")).toHaveAttribute("data-theme", "light");

    matches = true;
    listeners.forEach((listener) => listener());

    await waitFor(() => {
      expect(document.querySelector(".studio-shell")).toHaveAttribute("data-theme", "dark");
    });
  });

  it("uses the current week's Monday for weekly insights on Sundays", () => {
    expect(insightsWeekStart(new Date("2026-03-29T12:00:00Z"))).toBe("2026-03-23");
  });

  it("loads offer codes only when the promo codes section is opened", async () => {
    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    expect(mockGetOfferCodes).not.toHaveBeenCalled();

    fireEvent.click(await screen.findByRole("button", { name: "Promo Codes" }));

    await waitFor(() => {
      expect(mockGetOfferCodes).toHaveBeenCalledWith("1");
    });
  });

  it("retries offer codes after a failed load", async () => {
    mockGetOfferCodes
      .mockResolvedValueOnce({ error: "offer codes unavailable", offerCodes: [] })
      .mockResolvedValueOnce({
        offerCodes: [
          {
            subscriptionName: "Pro Plan",
            subscriptionId: "sub-1",
            name: "Welcome Offer",
            offerEligibility: "NEW",
            customerEligibilities: [],
            duration: "ONE_MONTH",
            offerMode: "FREE_TRIAL",
            numberOfPeriods: 1,
            totalNumberOfCodes: 100,
            productionCodeCount: 40,
          },
        ],
      });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    fireEvent.click(await screen.findByRole("button", { name: "Promo Codes" }));
    expect(await screen.findByText("offer codes unavailable")).toBeInTheDocument();

    fireEvent.click(await screen.findByRole("button", { name: "Builds" }));
    fireEvent.click(await screen.findByRole("button", { name: "Promo Codes" }));

    expect(await screen.findByText("Welcome Offer")).toBeInTheDocument();
    expect(mockGetOfferCodes).toHaveBeenCalledTimes(2);
  });

  it("shows partial offer codes alongside backend warnings", async () => {
    mockGetOfferCodes.mockResolvedValue({
      error: "failed to load offer codes for Plus: timeout",
      offerCodes: [
        {
          subscriptionName: "Pro Plan",
          subscriptionId: "sub-1",
          name: "Welcome Offer",
          offerEligibility: "NEW",
          customerEligibilities: [],
          duration: "ONE_MONTH",
          offerMode: "FREE_TRIAL",
          numberOfPeriods: 1,
          totalNumberOfCodes: 100,
          productionCodeCount: 40,
        },
      ],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");

    fireEvent.click(await screen.findByRole("button", { name: "Promo Codes" }));

    expect(await screen.findByText("failed to load offer codes for Plus: timeout")).toBeInTheDocument();
    expect(screen.getByText("Welcome Offer")).toBeInTheDocument();
  });

  it("ignores stale insights responses after switching apps", async () => {
    let resolveFirstInsights: ((value: { error: string; data: string }) => void) | undefined;

    mockListApps.mockResolvedValue({
      apps: [
        { id: "1", name: "First App", subtitle: "One" },
        { id: "2", name: "Second App", subtitle: "Two" },
      ],
    });
    mockGetAppDetail.mockImplementation((appID: string) => {
      if (appID === "1") {
        return Promise.resolve({
          id: "1",
          name: "First App",
          subtitle: "One",
          bundleId: "com.example.first",
          sku: "FIRSTSKU",
          primaryLocale: "en-US",
          versions: [{ id: "version-1", platform: "IOS", version: "1.0", state: "READY_FOR_SALE" }],
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
    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "insights weekly --app '1' --source analytics --week 2026-03-30 --output json") {
        return new Promise((resolve) => {
          resolveFirstInsights = resolve as (value: { error: string; data: string }) => void;
        });
      }
      if (cmd === "insights weekly --app '2' --source analytics --week 2026-03-30 --output json") {
        return Promise.resolve({
          error: "",
          data: JSON.stringify({
            metrics: [{ name: "second_metric", status: "VALID", thisWeek: 10 }],
          }),
        });
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("First App");
    fireEvent.click(await screen.findByRole("button", { name: "Insights" }));

    await waitFor(() => {
      expect(mockRunASCCommand).toHaveBeenCalledWith(
        "insights weekly --app '1' --source analytics --week 2026-03-30 --output json",
      );
    });

    await pickApp("Second App");
    fireEvent.click(await screen.findByRole("button", { name: "Insights" }));

    expect(await screen.findByText("second metric")).toBeInTheDocument();

    resolveFirstInsights?.({
      error: "",
      data: JSON.stringify({
        metrics: [{ name: "first_metric", status: "VALID", thisWeek: 1 }],
      }),
    });

    await waitFor(() => {
      expect(screen.getByText("second metric")).toBeInTheDocument();
    });
    expect(screen.queryByText("first metric")).not.toBeInTheDocument();
  });

  it("reloads insights when refreshing the selected app on the insights view", async () => {
    const firstInsights = {
      error: "",
      data: JSON.stringify({
        metrics: [{ name: "first_metric", status: "VALID", thisWeek: 1 }],
      }),
    };
    const refreshedInsights = {
      error: "",
      data: JSON.stringify({
        metrics: [{ name: "refreshed_metric", status: "VALID", thisWeek: 2 }],
      }),
    };

    mockRunASCCommand.mockImplementation((cmd: string) => {
      if (cmd === "insights weekly --app '1' --source analytics --week 2026-03-30 --output json") {
        return Promise.resolve(
          mockRunASCCommand.mock.calls.filter(([calledCmd]) => calledCmd === cmd).length === 1
            ? firstInsights
            : refreshedInsights,
        );
      }
      return Promise.resolve({ error: "", data: "{\"data\":[]}" });
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Insights" }));

    expect(await screen.findByText("first metric")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Refresh/i }));

    expect(await screen.findByText("refreshed metric")).toBeInTheDocument();
    expect(mockRunASCCommand.mock.calls.filter(([cmd]) => cmd === "insights weekly --app '1' --source analytics --week 2026-03-30 --output json")).toHaveLength(2);
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

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Groups" }));

    fireEvent.click((await screen.findAllByText("Internal"))[0].closest("tr")!);
    fireEvent.click(await screen.findByRole("button", { name: "Back to TestFlight groups" }));
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

  it("surfaces tester fetch failures in the TestFlight detail view", async () => {
    mockGetTestFlight.mockResolvedValue({
      groups: [
        { id: "group-1", name: "Internal", isInternal: true, publicLink: "", feedbackEnabled: false, createdDate: "2026-03-30T00:00:00Z", testerCount: 1 },
      ],
    });
    mockGetTestFlightTesters.mockResolvedValue({
      error: "auth expired",
      testers: [],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Groups" }));
    fireEvent.click((await screen.findAllByText("Internal"))[0].closest("tr")!);

    expect(await screen.findByText("auth expired")).toBeInTheDocument();
    expect(screen.queryByText("No testers in this group.")).not.toBeInTheDocument();
  });

  it("renders TestFlight views safely when optional fields are missing", async () => {
    mockGetTestFlight.mockResolvedValue({
      groups: [
        { id: "group-1", name: "Internal", isInternal: true, publicLink: "", feedbackEnabled: false, createdDate: null as unknown as string, testerCount: 1 },
      ],
    });
    mockGetTestFlightTesters.mockResolvedValue({
      testers: [{ email: "", firstName: "", lastName: "", inviteType: null as unknown as string, state: null as unknown as string }],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Groups" }));

    expect(await screen.findAllByText("Internal")).not.toHaveLength(0);

    fireEvent.click((await screen.findAllByText("Internal"))[0].closest("tr")!);

    expect((await screen.findAllByText("Unknown")).length).toBeGreaterThan(0);
    expect(screen.queryByText("Something went wrong")).not.toBeInTheDocument();
  });

  it("renders subscription views safely when status fields are missing", async () => {
    mockGetSubscriptions.mockResolvedValue({
      subscriptions: [
        {
          id: "sub-1",
          groupName: "Main Group",
          name: "Pro Monthly",
          productId: "pro.monthly",
          state: null as unknown as string,
          subscriptionPeriod: null as unknown as string,
          reviewNote: "",
          groupLevel: 1,
        },
      ],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Subscriptions" }));

    expect(await screen.findByText("Pro Monthly")).toBeInTheDocument();
    expect(screen.getByText("Unknown")).toBeInTheDocument();
    expect(screen.queryByText("Something went wrong")).not.toBeInTheDocument();
  });

  it("renders pricing subscription rows safely when status fields are missing", async () => {
    mockGetPricingOverview.mockResolvedValue({
      availableInNewTerritories: false,
      currentPrice: "1.99",
      currentProceeds: "1.39",
      baseCurrency: "USD",
      territories: [],
      subscriptionPricing: [
        {
          name: "Pro Monthly",
          productId: "pro.monthly",
          subscriptionPeriod: "ONE_MONTH",
          state: null as unknown as string,
          groupName: "Main Group",
          price: "1.99",
          currency: "USD",
          proceeds: "1.39",
        },
      ],
    });

    render(<App />);

    await screen.findByRole("img", { name: /Connected/i });
    await pickApp("Test App");
    fireEvent.click(await screen.findByRole("button", { name: "Pricing and Availability" }));

    expect(await screen.findByText("Pro Monthly")).toBeInTheDocument();
    expect(screen.getByText("Unknown")).toBeInTheDocument();
    expect(screen.queryByText("Something went wrong")).not.toBeInTheDocument();
  });
});
