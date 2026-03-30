import { FormEvent, startTransition, useEffect, useEffectEvent, useRef, useState } from "react";

import "./styles.css";
import { ChatMessage, NavSection } from "./types";
import { Bootstrap, CheckAuthStatus, GetAppDetail, GetFinanceRegions, GetOfferCodes, GetPricingOverview, GetScreenshots, GetSettings, GetSubscriptions, GetTestFlight, GetTestFlightTesters, GetVersionMetadata, ListApps, RunASCCommand, SaveSettings } from "../wailsjs/go/main/App";
import { environment, settings as settingsNS } from "../wailsjs/go/models";

type SidebarGroup = { label: string; items: NavSection[] };
type Scope = { id: string; label: string; groups: SidebarGroup[] };

const scopes: Scope[] = [
  {
    id: "app", label: "App",
    groups: [
      {
        label: "Overview",
        items: [
          { id: "overview", label: "App Information", description: "App details and metadata" },
          { id: "status", label: "Status", description: "Release health dashboard" },
          { id: "history", label: "History", description: "Version history" },
        ],
      },
      {
        label: "Release",
        items: [
          { id: "builds", label: "Builds", description: "Build processing and history" },
          { id: "testflight", label: "TestFlight", description: "Beta groups and testers" },
          { id: "submit", label: "Submit", description: "Submit for review" },
          { id: "validate", label: "Validate", description: "Pre-submission validation" },
          { id: "release-notes", label: "Release Notes", description: "What's new" },
          { id: "build-localizations", label: "Build Localizations", description: "Build release notes" },
          { id: "sandbox", label: "Sandbox", description: "Sandbox testers" },
        ],
      },
      {
        label: "Metadata",
        items: [
          { id: "localizations", label: "Localizations", description: "Locale metadata" },
          { id: "screenshots", label: "Screenshots", description: "App Store screenshots" },
          { id: "video-previews", label: "Video Previews", description: "App preview videos" },
          { id: "background-assets", label: "Background Assets", description: "Background download assets" },
          { id: "categories", label: "Categories", description: "App categories" },
          { id: "pre-orders", label: "Pre-orders", description: "Pre-order configuration" },
          { id: "app-tags", label: "App Tags", description: "App tags" },
          { id: "app-setup", label: "App Setup", description: "App configuration" },
        ],
      },
      {
        label: "Growth",
        items: [
          { id: "ratings-reviews", label: "Ratings and Reviews", description: "Customer reviews" },
          { id: "in-app-events", label: "In-App Events", description: "App events" },
          { id: "custom-product-pages", label: "Custom Product Pages", description: "Product pages" },
          { id: "ppo", label: "Product Page Optimization", description: "A/B tests" },
          { id: "promo-codes", label: "Promo Codes", description: "Offer codes" },
          { id: "nominations", label: "Nominations", description: "Featuring nominations" },
        ],
      },
      {
        label: "Monetization",
        items: [
          { id: "pricing", label: "Pricing and Availability", description: "Pricing" },
          { id: "iap", label: "In-App Purchases", description: "In-app purchases" },
          { id: "subscriptions", label: "Subscriptions", description: "Subscription groups" },
        ],
      },
      {
        label: "Insights",
        items: [
          { id: "performance", label: "Performance", description: "Metrics and diagnostics" },
          { id: "insights", label: "Insights", description: "Weekly analytics" },
          { id: "finance", label: "Finance", description: "Financial reports" },
        ],
      },
      {
        label: "Compliance",
        items: [
          { id: "app-review", label: "App Review", description: "Review submissions" },
          { id: "age-rating", label: "Age Rating", description: "Age rating declarations" },
          { id: "app-accessibility", label: "Accessibility", description: "Accessibility declarations" },
          { id: "encryption", label: "Encryption", description: "Export compliance" },
          { id: "eula", label: "EULA", description: "License agreements" },
        ],
      },
      {
        label: "Platform",
        items: [
          { id: "game-center", label: "Game Center", description: "Game Center resources" },
          { id: "app-clips", label: "App Clips", description: "App Clip experiences" },
          { id: "android-ios-mapping", label: "Android to iOS", description: "Android app mapping" },
          { id: "marketplace", label: "Marketplace", description: "Marketplace search" },
          { id: "alt-distribution", label: "Alternative Distribution", description: "Alt distribution keys" },
          { id: "routing-coverage", label: "Routing Coverage", description: "Routing app coverage" },
        ],
      },
    ],
  },
  {
    id: "team", label: "Team",
    groups: [
      {
        label: "Account",
        items: [
          { id: "account-status", label: "Account", description: "Account health" },
          { id: "users", label: "Users", description: "Team members" },
          { id: "devices", label: "Devices", description: "Registered devices" },
        ],
      },
    ],
  },
  {
    id: "signing", label: "Signing",
    groups: [
      {
        label: "Identifiers",
        items: [
          { id: "bundle-ids", label: "Bundle IDs", description: "App identifiers" },
          { id: "certificates", label: "Certificates", description: "Signing certificates" },
          { id: "profiles", label: "Profiles", description: "Provisioning profiles" },
        ],
      },
    ],
  },
  {
    id: "automation", label: "Automation",
    groups: [
      {
        label: "Workflows",
        items: [
          { id: "xcode-cloud", label: "Xcode Cloud", description: "CI/CD workflows" },
          { id: "webhooks", label: "Webhooks", description: "Webhook management" },
        ],
      },
    ],
  },
];

// Flatten for lookup
const allSections: NavSection[] = scopes.flatMap((s) => s.groups.flatMap((g) => g.items));
allSections.push({ id: "settings", label: "Settings", description: "Studio preferences" });

// Map section IDs to asc CLI commands. APP_ID is replaced at runtime.
const sectionCommands: Record<string, string> = {
  "app-review": "review submissions-list --app APP_ID --output json",
  "history": "versions list --app APP_ID --output json",
  "builds": "builds list --app APP_ID --limit 20 --output json",
  "app-privacy": "age-rating view --app APP_ID --output json",
  "app-accessibility": "accessibility list --app APP_ID --output json",
  "in-app-events": "app-events list --app APP_ID --output json",
  "custom-product-pages": "product-pages custom-pages list --app APP_ID --output json",
  "ppo": "product-pages experiments list --v2 --app APP_ID --output json",
  "game-center": "game-center achievements list --app APP_ID --output json",
  "iap": "iap list --app APP_ID --output json",
  "nominations": "nominations list --app APP_ID --status DRAFT,SUBMITTED,ARCHIVED --output json",
  "performance": "performance metrics list --app APP_ID --output json",
  "submit": "review submissions-list --app APP_ID --output json",
  "validate": "validate --app APP_ID --output json",
  "release-notes": "localizations list --app APP_ID --output json",
  "localizations": "localizations list --app APP_ID --type app-info --output json",
  "video-previews": "localizations preview-sets list --app APP_ID --output json",
  "categories": "categories list --output json",
  "age-rating": "age-rating view --app APP_ID --output json",
  "encryption": "encryption list --app APP_ID --output json",
  "account-status": "account status --output json",
  "users": "users list --output json",
  "devices": "devices list --output json",
  "bundle-ids": "bundle-ids list --paginate --output json",
  "certificates": "certificates list --paginate --output json",
  "profiles": "profiles list --paginate --output json",
  "xcode-cloud": "xcode-cloud workflows list --app APP_ID --output json",
  "webhooks": "webhooks list --app APP_ID --output json",
  "background-assets": "background-assets list --app APP_ID --output json",
  "pre-orders": "pre-orders view --app APP_ID --output json",
  "app-tags": "app-tags list --app APP_ID --output json",
  "app-setup": "app-setup info list --app APP_ID --output json",
  "app-clips": "app-clips list --app APP_ID --output json",
  "android-ios-mapping": "android-ios-mapping list --app APP_ID --output json",
  "marketplace": "marketplace search-details view --app APP_ID --output json",
  "alt-distribution": "alternative-distribution domains list --output json",
  "routing-coverage": "routing-coverage list --app APP_ID --output json",
  "eula": "eula list --app APP_ID --output json",
  "build-localizations": "build-localizations list --app APP_ID --output json",
  "sandbox": "sandbox list --output json",
};

// Human-readable field labels for known attribute keys
const fieldLabels: Record<string, string> = {
  name: "Name", productId: "Product ID", inAppPurchaseType: "Type", state: "State",
  rating: "Rating", title: "Title", body: "Review", reviewerNickname: "Reviewer",
  createdDate: "Date", territory: "Territory", platform: "Platform",
  versionString: "Version", appVersionState: "State", appStoreState: "Store State",
  referenceName: "Reference Name", vendorId: "Vendor ID", points: "Points",
  status: "Status", description: "Description", badge: "Badge",
  advertisingIdDeclaration: "Ad ID Declaration", advertising: "Advertising",
  gambling: "Gambling", lootBox: "Loot Box",
  subscriptionGroupId: "Group ID", groupLevel: "Group Level",
  healthOrWellnessTopics: "Health or Wellness", messagingAndChat: "Messaging & Chat",
  parentalControls: "Parental Controls", ageAssurance: "Age Assurance",
  unrestrictedWebAccess: "Unrestricted Web Access", userGeneratedContent: "User Generated Content",
  alcoholTobaccoOrDrugUseOrReferences: "Alcohol/Tobacco/Drug References",
  contests: "Contests", gamblingSimulated: "Simulated Gambling",
  gunsOrOtherWeapons: "Guns or Weapons", medicalOrTreatmentInformation: "Medical Information",
  profanityOrCrudeHumor: "Profanity or Crude Humor",
  sexualContentGraphicAndNudity: "Sexual Content (Graphic)", sexualContentOrNudity: "Sexual Content",
  horrorOrFearThemes: "Horror or Fear", matureOrSuggestiveThemes: "Mature Themes",
  violenceCartoonOrFantasy: "Violence (Cartoon)", violenceRealistic: "Violence (Realistic)",
  violenceRealisticProlongedGraphicOrSadistic: "Violence (Graphic/Sadistic)",
  copyright: "Copyright", releaseType: "Release Type",
  appStoreAgeRating: "Age Rating", kidsAgeBand: "Kids Age Band",
  deviceFamily: "Device", supportsAudioDescriptions: "Audio Descriptions",
  supportsCaptions: "Captions", supportsDarkInterface: "Dark Interface",
  supportsDifferentiateWithoutColorAlone: "Differentiate Without Color",
  supportsLargeText: "Large Text", supportsVoiceOver: "VoiceOver",
  supportsSwitchControl: "Switch Control", supportsAssistiveTouch: "Assistive Touch",
  supportsReduceMotion: "Reduce Motion", supportsGuidedAccess: "Guided Access",
  availableInNewTerritories: "Available in New Territories", customerPrice: "Customer Price",
  proceeds: "Proceeds", version: "Build", uploadedDate: "Uploaded", expirationDate: "Expires",
  processingState: "Processing", minOsVersion: "Min OS", usesNonExemptEncryption: "Encryption",
  submittedDate: "Submitted",
};

// Format raw API enum values for display
const displayValue: Record<string, string> = {
  IOS: "iOS", MAC_OS: "macOS", TV_OS: "tvOS", VISION_OS: "visionOS",
  IPHONE: "iPhone", IPAD: "iPad", APPLE_TV: "Apple TV", APPLE_WATCH: "Apple Watch",
  DRAFT: "Draft",
  READY_FOR_SALE: "Ready for Sale", READY_FOR_DISTRIBUTION: "Ready for Distribution",
  PREPARE_FOR_SUBMISSION: "Prepare for Submission", WAITING_FOR_REVIEW: "Waiting for Review",
  IN_REVIEW: "In Review", PENDING_DEVELOPER_RELEASE: "Pending Developer Release",
  DEVELOPER_REJECTED: "Developer Rejected", REJECTED: "Rejected",
  REMOVED_FROM_SALE: "Removed from Sale", AFTER_APPROVAL: "After Approval",
  MANUAL: "Manual", ONE_MONTH: "1 month", ONE_YEAR: "1 year", ONE_WEEK: "1 week",
  TWO_MONTHS: "2 months", THREE_MONTHS: "3 months", SIX_MONTHS: "6 months",
  CONSUMABLE: "Consumable", NON_CONSUMABLE: "Non-Consumable",
  AUTO_RENEWABLE: "Auto-Renewable", NON_RENEWING: "Non-Renewing",
  APPROVED: "Approved", VALID: "Valid", COMPLETE: "Complete", UNAVAILABLE: "Unavailable",
  FREE_TRIAL: "Free Trial", PAY_AS_YOU_GO: "Pay as You Go", PAY_UP_FRONT: "Pay Up Front",
  STACK_WITH_INTRO_OFFERS: "Stack with Intro", EXISTING: "Existing", EXPIRED: "Expired", NEW: "New",
  READY_FOR_REVIEW: "Ready for Review", WAITING_FOR_EXPORT_COMPLIANCE: "Waiting for Export Compliance",
  PROCESSING: "Processing", FAILED: "Failed", INVALID: "Invalid",
  INSTALLED: "Installed", INVITED: "Invited", ACCEPTED: "Accepted",
  PUBLIC_LINK: "Public Link", EMAIL: "Email",
};
function fmt(val: string): string {
  if (displayValue[val]) return displayValue[val];
  // Format ISO dates like "2026-03-28T08:32:01-07:00" → "2026-03-28"
  if (/^\d{4}-\d{2}-\d{2}T/.test(val)) return val.split("T")[0];
  return val;
}

type EnvSnapshot = {
  configPath: string;
  configPresent: boolean;
  defaultAppId: string;
  keychainAvailable: boolean;
  keychainBypassed: boolean;
  workflowPath: string;
};

type StudioSettings = {
  preferredPreset: string;
  agentCommand: string;
  agentArgs: string[];
  agentEnv: Record<string, string>;
  preferBundledASC: boolean;
  systemASCPath: string;
  workspaceRoot: string;
  showCommandPreviews: boolean;
  theme: string;
  windowMaterial: string;
};

const emptyEnv: EnvSnapshot = {
  configPath: "",
  configPresent: false,
  defaultAppId: "",
  keychainAvailable: false,
  keychainBypassed: false,
  workflowPath: "",
};

const defaultSettings: StudioSettings = {
  preferredPreset: "codex",
  agentCommand: "",
  agentArgs: [],
  agentEnv: {},
  preferBundledASC: true,
  systemASCPath: "",
  workspaceRoot: "",
  showCommandPreviews: true,
  theme: "system",
  windowMaterial: "translucent",
};

type AuthState = {
  authenticated: boolean;
  storage: string;
  profile: string;
  rawOutput: string;
};

const emptyAuthStatus: AuthState = {
  authenticated: false,
  storage: "",
  profile: "",
  rawOutput: "",
};

function normalizeEnvSnapshot(snapshot?: Partial<EnvSnapshot>): EnvSnapshot {
  return {
    configPath: snapshot?.configPath || "",
    configPresent: snapshot?.configPresent || false,
    defaultAppId: snapshot?.defaultAppId || "",
    keychainAvailable: snapshot?.keychainAvailable || false,
    keychainBypassed: snapshot?.keychainBypassed || false,
    workflowPath: snapshot?.workflowPath || "",
  };
}

function normalizeStudioSettings(input?: Partial<StudioSettings>): StudioSettings {
  return {
    preferredPreset: input?.preferredPreset || "codex",
    agentCommand: input?.agentCommand || "",
    agentArgs: input?.agentArgs || [],
    agentEnv: input?.agentEnv || {},
    preferBundledASC: input?.preferBundledASC ?? true,
    systemASCPath: input?.systemASCPath || "",
    workspaceRoot: input?.workspaceRoot || "",
    showCommandPreviews: input?.showCommandPreviews ?? true,
    theme: input?.theme || "system",
    windowMaterial: input?.windowMaterial || "translucent",
  };
}

function getSystemTheme(): "light" | "dark" {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return "light";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function resolveTheme(theme: string | undefined, systemTheme: "light" | "dark"): "light" | "dark" {
  switch (theme) {
    case "dark":
    case "glass-dark":
      return "dark";
    case "light":
    case "glass-light":
      return "light";
    default:
      return systemTheme;
  }
}

export function insightsWeekStart(today: Date): string {
  const monday = new Date(today);
  const day = today.getDay();
  const daysSinceMonday = day === 0 ? 6 : day - 1;
  monday.setDate(today.getDate() - daysSinceMonday);
  return monday.toISOString().split("T")[0];
}

function normalizeAuthStatus(input?: Partial<AuthState>): AuthState {
  return {
    authenticated: input?.authenticated || false,
    storage: input?.storage || "",
    profile: input?.profile || "",
    rawOutput: input?.rawOutput || "",
  };
}

function mapAppList(apps?: { id: string; name: string; subtitle: string }[]) {
  return (apps ?? []).map((app) => ({
    id: app.id,
    name: app.name,
    subtitle: app.subtitle,
  }));
}

export default function App() {
  const [activeScope, setActiveScope] = useState<string>("app");
  const [activeSection, setActiveSection] = useState<NavSection>(allSections[0]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [dockExpanded, setDockExpanded] = useState(false);

  const [env, setEnv] = useState<EnvSnapshot>(emptyEnv);
  const [studioSettings, setStudioSettings] = useState<StudioSettings>(defaultSettings);
  const [systemTheme, setSystemTheme] = useState<"light" | "dark">(getSystemTheme);
  const [settingsSaved, setSettingsSaved] = useState(false);
  const [bootstrapError, setBootstrapError] = useState("");
  const [loading, setLoading] = useState(true);
  const [authStatus, setAuthStatus] = useState<AuthState>(emptyAuthStatus);
  const [appList, setAppList] = useState<{ id: string; name: string; subtitle: string }[]>([]);
  const [selectedAppId, setSelectedAppId] = useState<string | null>(null);
  const [appDetail, setAppDetail] = useState<{
    id: string; name: string; subtitle: string; bundleId: string; sku: string; primaryLocale: string;
    versions: { id: string; platform: string; version: string; state: string }[];
    error?: string;
  } | null>(null);
  const [, setDetailLoading] = useState(false);
  const [allLocalizations, setAllLocalizations] = useState<{
    localizationId: string; locale: string; description: string; keywords: string;
    whatsNew: string; promotionalText: string; supportUrl: string; marketingUrl: string;
  }[]>([]);
  const [selectedLocale, setSelectedLocale] = useState<string>("");
  const [metadataLoading, setMetadataLoading] = useState(false);
  const [screenshotSets, setScreenshotSets] = useState<{
    displayType: string;
    screenshots: { thumbnailUrl: string; width: number; height: number }[];
  }[]>([]);
  const [screenshotsLoading, setScreenshotsLoading] = useState(false);
  const [appsLoading, setAppsLoading] = useState(false);
  // Cache of section data keyed by section ID. Prefetched in parallel on app select.
  const [sectionCache, setSectionCache] = useState<Record<string, { loading: boolean; error?: string; items: Record<string, unknown>[] }>>({});
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  type AppStatusData = {
    summary?: { health: string; nextAction?: string; blockers?: string[] };
    builds?: { latest?: { version: string; buildNumber: string; processingState: string; uploadedDate: string; platform: string } };
    testflight?: { betaReviewState: string; submittedDate?: string };
    appstore?: { version: string; state: string; platform: string; createdDate: string; versionId: string };
    submission?: { inFlight: boolean; blockingIssues?: string[] };
    review?: { state: string; submittedDate?: string; platform?: string };
    phasedRelease?: { configured: boolean };
    links?: { appStoreConnect?: string; testFlight?: string; review?: string };
  };
  const [appStatus, setAppStatus] = useState<{ loading: boolean; error?: string; data: AppStatusData | null }>({ loading: false, data: null });
  const [testflightData, setTestflightData] = useState<{ loading: boolean; error?: string; groups: { id: string; name: string; isInternal: boolean; publicLink: string; feedbackEnabled: boolean; createdDate: string; testerCount: number }[] }>({ loading: false, groups: [] });
  const [selectedGroup, setSelectedGroup] = useState<string | null>(null);
  const [groupTesters, setGroupTesters] = useState<{ loading: boolean; testers: { email: string; firstName: string; lastName: string; inviteType: string; state: string }[] }>({ loading: false, testers: [] });
  const [reviews, setReviews] = useState<{ loading: boolean; error?: string; items: { rating: number; title: string; body: string; reviewerNickname: string; createdDate: string; territory: string }[] }>({ loading: false, items: [] });
  const [subscriptions, setSubscriptions] = useState<{ loading: boolean; error?: string; items: { id: string; groupName: string; name: string; productId: string; state: string; subscriptionPeriod: string; reviewNote: string; groupLevel: number }[] }>({ loading: false, items: [] });
  const [pricingOverview, setPricingOverview] = useState<{ loading: boolean; error?: string; availableInNewTerritories: boolean; currentPrice: string; currentProceeds: string; baseCurrency: string; territories: { territory: string; available: boolean; releaseDate: string }[]; subscriptionPricing: { name: string; productId: string; subscriptionPeriod: string; state: string; groupName: string; price: string; currency: string; proceeds: string }[] }>({ loading: false, availableInNewTerritories: false, currentPrice: "", currentProceeds: "", baseCurrency: "", territories: [], subscriptionPricing: [] });
  const [selectedSub, setSelectedSub] = useState<string | null>(null);
  const [financeRegions, setFinanceRegions] = useState<{ loading: boolean; error?: string; regions: { reportRegion: string; reportCurrency: string; regionCode: string; countriesOrRegions: string }[] }>({ loading: false, regions: [] });
  const [offerCodes, setOfferCodes] = useState<{ loading: boolean; error?: string; codes: { subscriptionName: string; subscriptionId: string; name: string; offerEligibility: string; customerEligibilities: string[]; duration: string; offerMode: string; numberOfPeriods: number; totalNumberOfCodes: number; productionCodeCount: number }[] }>({ loading: false, codes: [] });
  const appSelectionRequestRef = useRef(0);
  const screenshotRequestRef = useRef(0);
  const groupTesterRequestRef = useRef(0);

  const loadStudioShell = useEffectEvent(async (options?: {
    clearApps?: boolean;
    isCancelled?: () => boolean;
  }) => {
    const isCancelled = options?.isCancelled ?? (() => false);

    try {
      const [data, auth] = await Promise.all([Bootstrap(), CheckAuthStatus()]);
      if (isCancelled()) return;

      startTransition(() => {
        setEnv(normalizeEnvSnapshot(data.environment));
        setStudioSettings(normalizeStudioSettings(data.settings));
        setAuthStatus(normalizeAuthStatus(auth));
        setBootstrapError("");
        if (options?.clearApps) {
          setAppList([]);
        }
      });

      if (!auth?.authenticated) {
        if (isCancelled()) return;
        startTransition(() => {
          setAppList([]);
          setAppsLoading(false);
        });
        return;
      }

      setAppsLoading(true);
      try {
        const res = await ListApps();
        if (isCancelled()) return;
        startTransition(() => {
          setAppList(mapAppList(res.apps));
        });
      } catch {
        if (isCancelled()) return;
        startTransition(() => {
          setAppList([]);
        });
      } finally {
        if (!isCancelled()) {
          setAppsLoading(false);
        }
      }
    } catch (err) {
      if (isCancelled()) return;
      setBootstrapError(String(err));
    } finally {
      if (!isCancelled()) {
        setLoading(false);
      }
    }
  });

  useEffect(() => {
    let cancelled = false;

    void loadStudioShell({ isCancelled: () => cancelled });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    const handleChange = (event?: MediaQueryListEvent) => {
      setSystemTheme(event?.matches ?? mediaQuery.matches ? "dark" : "light");
    };

    handleChange();
    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", handleChange);
      return () => mediaQuery.removeEventListener("change", handleChange);
    }

    mediaQuery.addListener(handleChange);
    return () => mediaQuery.removeListener(handleChange);
  }, []);

  function updateSetting<K extends keyof StudioSettings>(key: K, value: StudioSettings[K]) {
    setStudioSettings((prev) => ({ ...prev, [key]: value }));
    setSettingsSaved(false);
  }

  function handleSaveSettings() {
    const payload = new settingsNS.StudioSettings({
      preferredPreset: studioSettings.preferredPreset,
      agentCommand: studioSettings.agentCommand,
      agentArgs: studioSettings.agentArgs,
      agentEnv: studioSettings.agentEnv,
      preferBundledASC: studioSettings.preferBundledASC,
      systemASCPath: studioSettings.systemASCPath,
      workspaceRoot: studioSettings.workspaceRoot,
      theme: studioSettings.theme === "glass-light" ? "system" : studioSettings.theme,
      windowMaterial: studioSettings.windowMaterial,
      showCommandPreviews: studioSettings.showCommandPreviews,
    });
    SaveSettings(payload)
      .then(() => setSettingsSaved(true))
      .catch((err) => console.error("save settings:", err));
  }

  // Prefetch all section data in parallel for an app
  function prefetchSections(appId: string, requestID: number) {
    const isStale = () => appSelectionRequestRef.current !== requestID;
    const initial: Record<string, { loading: boolean; error?: string; items: Record<string, unknown>[] }> = {};
    for (const sectionId of Object.keys(sectionCommands)) {
      initial[sectionId] = { loading: true, items: [] };
    }
    setSectionCache(initial);
    // App status dashboard
    setAppStatus({ loading: true, data: null });
    RunASCCommand(`status --app ${appId} --output json`)
      .then((res) => {
        if (isStale()) return;
        if (res.error) { setAppStatus({ loading: false, error: res.error, data: null }); return; }
        try { setAppStatus({ loading: false, data: JSON.parse(res.data) }); }
        catch { setAppStatus({ loading: false, error: "Failed to parse status", data: null }); }
      })
      .catch((e) => {
        if (isStale()) return;
        setAppStatus({ loading: false, error: String(e), data: null });
      });

    // TestFlight groups with tester counts
    setTestflightData({ loading: true, groups: [] });
    setSelectedGroup(null);
    setGroupTesters({ loading: false, testers: [] });
    groupTesterRequestRef.current += 1;
    GetTestFlight(appId)
      .then((res) => {
        if (isStale()) return;
        if (res.error) setTestflightData({ loading: false, error: res.error, groups: [] });
        else setTestflightData({ loading: false, groups: res.groups ?? [] });
      })
      .catch((e) => {
        if (isStale()) return;
        setTestflightData({ loading: false, error: String(e), groups: [] });
      });

    // Reviews
    setReviews({ loading: true, items: [] });
    RunASCCommand(`reviews list --app ${appId} --limit 25 --output json`)
      .then((res) => {
        if (isStale()) return;
        if (res.error) { setReviews({ loading: false, error: res.error, items: [] }); return; }
        try {
          const d = JSON.parse(res.data);
          setReviews({ loading: false, items: (d.data ?? []).map((i: { attributes: Record<string, unknown> }) => i.attributes) });
        } catch { setReviews({ loading: false, error: "Failed to parse", items: [] }); }
      })
      .catch((e) => {
        if (isStale()) return;
        setReviews({ loading: false, error: String(e), items: [] });
      });

    // Pricing overview
    setPricingOverview({ loading: true, availableInNewTerritories: false, currentPrice: "", currentProceeds: "", baseCurrency: "", territories: [], subscriptionPricing: [] });
    GetPricingOverview(appId)
      .then((res) => {
        if (isStale()) return;
        if (res.error) setPricingOverview({ loading: false, error: res.error, availableInNewTerritories: false, currentPrice: "", currentProceeds: "", baseCurrency: "", territories: [], subscriptionPricing: [] });
        else setPricingOverview({ loading: false, availableInNewTerritories: res.availableInNewTerritories, currentPrice: res.currentPrice, currentProceeds: res.currentProceeds, baseCurrency: res.baseCurrency, territories: res.territories ?? [], subscriptionPricing: res.subscriptionPricing ?? [] });
      })
      .catch((e) => {
        if (isStale()) return;
        setPricingOverview({ loading: false, error: String(e), availableInNewTerritories: false, currentPrice: "", currentProceeds: "", baseCurrency: "", territories: [], subscriptionPricing: [] });
      });

    // Subscriptions: dedicated two-phase fetch
    setSubscriptions({ loading: true, items: [] });
    GetSubscriptions(appId)
      .then((res) => {
        if (isStale()) return;
        if (res.error) setSubscriptions({ loading: false, error: res.error, items: [] });
        else setSubscriptions({ loading: false, items: res.subscriptions ?? [] });
      })
      .catch((e) => {
        if (isStale()) return;
        setSubscriptions({ loading: false, error: String(e), items: [] });
      });

    // Finance regions
    setFinanceRegions({ loading: true, regions: [] });
    GetFinanceRegions()
      .then((res) => {
        if (isStale()) return;
        if (res.error) setFinanceRegions({ loading: false, error: res.error, regions: [] });
        else setFinanceRegions({ loading: false, regions: res.regions ?? [] });
      })
      .catch((e) => { if (!isStale()) setFinanceRegions({ loading: false, error: String(e), regions: [] }); });

    // Offer codes for all subscriptions
    setOfferCodes({ loading: true, codes: [] });
    GetOfferCodes(appId)
      .then((res) => {
        if (isStale()) return;
        if (res.error) setOfferCodes({ loading: false, error: res.error, codes: [] });
        else setOfferCodes({ loading: false, codes: res.offerCodes ?? [] });
      })
      .catch((e) => { if (!isStale()) setOfferCodes({ loading: false, error: String(e), codes: [] }); });

    for (const [sectionId, cmdTemplate] of Object.entries(sectionCommands)) {
      const cmd = cmdTemplate.replace(/APP_ID/g, appId);
      RunASCCommand(cmd)
        .then((res) => {
          if (isStale()) return;
          if (res.error) {
            setSectionCache((prev) => ({ ...prev, [sectionId]: { loading: false, error: res.error, items: [] } }));
            return;
          }
          try {
            const parsed = JSON.parse(res.data);
            const items: Record<string, unknown>[] = [];
            if (Array.isArray(parsed?.data)) {
              for (const item of parsed.data) {
                items.push({ id: item.id, type: item.type, ...item.attributes });
              }
            } else if (parsed?.data?.attributes) {
              items.push({ id: parsed.data.id, type: parsed.data.type, ...parsed.data.attributes });
            }
            setSectionCache((prev) => ({ ...prev, [sectionId]: { loading: false, items } }));
          } catch {
            setSectionCache((prev) => ({ ...prev, [sectionId]: { loading: false, error: "Failed to parse response", items: [] } }));
          }
        })
        .catch((e) => {
          if (isStale()) return;
          setSectionCache((prev) => ({ ...prev, [sectionId]: { loading: false, error: String(e), items: [] } }));
        });
    }
  }

  function handleSelectApp(id: string) {
    const requestID = appSelectionRequestRef.current + 1;
    appSelectionRequestRef.current = requestID;
    screenshotRequestRef.current += 1;
    groupTesterRequestRef.current += 1;
    startTransition(() => {
      setSelectedAppId(id);
      setAppDetail(null);
      setAllLocalizations([]);
      setSelectedLocale("");
      setScreenshotSets([]);
      setSectionCache({});
      setSelectedGroup(null);
      setGroupTesters({ loading: false, testers: [] });
      setSelectedSub(null);
      setDetailLoading(true);
    });
    // Fire all section prefetches in parallel
    prefetchSections(id, requestID);
    GetAppDetail(id)
      .then((d) => {
        if (appSelectionRequestRef.current !== requestID) return;
        const detail = {
          id: d.id, name: d.name, subtitle: d.subtitle, bundleId: d.bundleId,
          sku: d.sku, primaryLocale: d.primaryLocale, versions: d.versions ?? [], error: d.error,
        };
        setAppDetail(detail);
        // Fetch metadata for the primary iOS version (fallback to first version)
        const primaryVersion = (d.versions ?? []).find((v: { platform: string }) => v.platform === "IOS")
          ?? (d.versions ?? [])[0];
        if (primaryVersion?.id) {
          setMetadataLoading(true);
          GetVersionMetadata(primaryVersion.id)
            .then((meta) => {
              if (appSelectionRequestRef.current !== requestID) return;
              if (meta.localizations?.length) {
                setAllLocalizations(meta.localizations);
                const defaultLoc = meta.localizations.find(
                  (l: { locale: string }) => l.locale === d.primaryLocale
                ) ?? meta.localizations[0];
                setSelectedLocale(defaultLoc.locale);
                // Fetch screenshots for the default locale in parallel
                if (defaultLoc.localizationId) {
                  const screenshotRequestID = screenshotRequestRef.current + 1;
                  screenshotRequestRef.current = screenshotRequestID;
                  setScreenshotsLoading(true);
                  GetScreenshots(defaultLoc.localizationId)
                    .then((res) => {
                      if (appSelectionRequestRef.current !== requestID || screenshotRequestRef.current !== screenshotRequestID) return;
                      setScreenshotSets(res.sets ?? []);
                    })
                    .catch(() => {})
                    .finally(() => {
                      if (appSelectionRequestRef.current !== requestID || screenshotRequestRef.current !== screenshotRequestID) return;
                      setScreenshotsLoading(false);
                    });
                }
              }
            })
            .catch(() => {})
            .finally(() => {
              if (appSelectionRequestRef.current !== requestID) return;
              setMetadataLoading(false);
            });
        }
      })
      .catch((e) => {
        if (appSelectionRequestRef.current !== requestID) return;
        setAppDetail({ id, name: "", subtitle: "", bundleId: "", sku: "", primaryLocale: "", versions: [], error: String(e) });
      })
      .finally(() => {
        if (appSelectionRequestRef.current !== requestID) return;
        setDetailLoading(false);
      });
  }

  function handleLocaleChange(locale: string) {
    setSelectedLocale(locale);
    const loc = allLocalizations.find((l) => l.locale === locale);
    if (loc?.localizationId) {
      const screenshotRequestID = screenshotRequestRef.current + 1;
      screenshotRequestRef.current = screenshotRequestID;
      setScreenshotsLoading(true);
      setScreenshotSets([]);
      GetScreenshots(loc.localizationId)
        .then((res) => {
          if (screenshotRequestRef.current !== screenshotRequestID) return;
          setScreenshotSets(res.sets ?? []);
        })
        .catch(() => {})
        .finally(() => {
          if (screenshotRequestRef.current !== screenshotRequestID) return;
          setScreenshotsLoading(false);
        });
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = draft.trim();
    if (!trimmed) return;

    setMessages((current) => [
      ...current,
      { id: `user-${current.length}`, role: "user", content: trimmed, timestamp: "Now" },
      {
        id: `assistant-${current.length}`,
        role: "assistant",
        content: "Bootstrap mode recorded the prompt. Live ACP transport is not wired yet.",
        timestamp: "Now",
      },
    ]);
    setDraft("");
    setDockExpanded(true);
  }

  const handleRefresh = useEffectEvent(() => {
    if (selectedAppId) {
      handleSelectApp(selectedAppId);
    } else {
      setLoading(true);
      setBootstrapError("");
      void loadStudioShell({ clearApps: true });
    }
  });

  // Cmd+R to refresh
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "r") {
        e.preventDefault();
        handleRefresh();
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const authConfigured = authStatus.authenticated;
  const resolvedTheme = resolveTheme(studioSettings.theme, systemTheme);

  return (
    <div className="studio-shell" data-theme={resolvedTheme}>
      {/* Sidebar */}
      <aside className="sidebar">
        <div className="sidebar-header" />

        {/* App picker dropdown — only in App scope */}
        {activeScope === "app" && <div className="sidebar-app-picker">
          {appsLoading ? (
            <div className="app-picker-placeholder">Loading apps…</div>
          ) : appList.length > 0 ? (
            <select
              className="app-picker-select"
              value={selectedAppId ?? ""}
              onChange={(e) => {
                const id = e.target.value;
                if (id) {
                  handleSelectApp(id);
                  setActiveSection(allSections[0]);
                }
              }}
            >
              <option value="" disabled>Select an app…</option>
              {appList.map((app) => (
                <option key={app.id} value={app.id}>
                  {app.name}{app.subtitle ? ` — ${app.subtitle}` : ""}
                </option>
              ))}
            </select>
          ) : (
            <div className="app-picker-placeholder">
              {authStatus.authenticated ? "No apps found" : "Not authenticated"}
            </div>
          )}
        </div>}

        <div className="sidebar-scroll">
        {/* Version badges when an app is selected (app scope only) */}
        {activeScope === "app" && appDetail && appDetail.versions.length > 0 && (
          <div className="sidebar-section">
            {(["IOS", "MAC_OS", "VISION_OS"] as const).map((platform) => {
              const v = appDetail.versions.find((ver) => ver.platform === platform);
              if (!v) return null;
              const label = platform === "IOS" ? "iOS App" : platform === "MAC_OS" ? "macOS App" : "visionOS App";
              return (
                <div key={platform} className="sidebar-version-group">
                  <p className="sidebar-version-platform">{label}</p>
                  <div className="sidebar-version-row">
                    <span className={`sidebar-version-dot state-${v.state.toLowerCase().replace(/_/g, "-")}`} />
                    <span className="sidebar-version-text">
                      {v.version} {v.state.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {(scopes.find((s) => s.id === activeScope)?.groups ?? []).map((group) => {
          // App scope needs an app selected; other scopes don't
          if (activeScope === "app" && !selectedAppId) return null;
          return (
            <div key={group.label} className="sidebar-section">
              <p className="sidebar-section-label">{group.label}</p>
              {group.items.map((section) => (
                <button
                  key={section.id}
                  type="button"
                  className={`sidebar-row ${section.id === activeSection.id ? "is-active" : ""}`}
                  onClick={() => setActiveSection(section)}
                >
                  <span>{section.label}</span>
                </button>
              ))}
            </div>
          );
        })}

        <div className="sidebar-section">
          <button
            type="button"
            className={`sidebar-row ${activeSection.id === "settings" ? "is-active" : ""}`}
            onClick={() => setActiveSection(allSections.find((s) => s.id === "settings")!)}
          >
            <span className="sidebar-row-icon">⚙</span>
            <span>Settings</span>
          </button>
        </div>

        <div className="sidebar-spacer" />
        </div>
      </aside>

      <div className="shell-separator" />

      {/* Main area */}
      <div className="main-area">
        {/* Context bar */}
        <header className="context-bar">
          <div className="context-app">
            {authConfigured ? (
              <>
                <span className="context-badge">{authStatus.storage || "Authenticated"}</span>
                {authStatus.profile && (
                  <span className="context-version">{authStatus.profile}</span>
                )}
                <span className="context-status state-ready">Connected</span>
              </>
            ) : (
              <span className="context-status state-processing">Not authenticated</span>
            )}
          </div>
          <div className="toolbar-right">
            <div className="scope-tabs">
              {scopes.map((scope) => (
                <button
                  key={scope.id}
                  type="button"
                  className={`scope-tab ${activeScope === scope.id ? "is-active" : ""}`}
                  onClick={() => {
                    setActiveScope(scope.id);
                    const firstSection = scope.groups[0]?.items[0];
                    if (firstSection) setActiveSection(firstSection);
                  }}
                >
                  {scope.label}
                </button>
              ))}
            </div>
            <button
              className="toolbar-btn"
              type="button"
              onClick={handleRefresh}
              title="Refresh (⌘R)"
            >
              ↻
            </button>
            {!authConfigured && (
              <button
                className="toolbar-btn"
                type="button"
                onClick={() => setActiveSection(allSections.find((s) => s.id === "settings")!)}
              >
                Configure
              </button>
            )}
          </div>
        </header>

        {loading ? (
          <div className="empty-state">
            <p className="empty-hint">Loading…</p>
          </div>
        ) : bootstrapError ? (
          <div className="empty-state">
            <p className="empty-title">Bootstrap failed</p>
            <p className="empty-hint">{bootstrapError}</p>
          </div>
        ) : activeSection.id === "settings" ? (
          <div className="settings-view">
            {/* Auth status */}
            <div className="workspace-section">
              <h3 className="section-label">Authentication</h3>
              <div className="env-grid">
                <div className="env-row">
                  <span className="env-key">Status</span>
                  <span className="env-value">
                    {authStatus.authenticated ? (
                      <span style={{ color: "var(--green)" }}>Authenticated</span>
                    ) : (
                      <span style={{ color: "var(--orange)" }}>Not authenticated</span>
                    )}
                  </span>
                </div>
                {authStatus.storage && (
                  <div className="env-row">
                    <span className="env-key">Storage</span>
                    <span className="env-value">{authStatus.storage}</span>
                  </div>
                )}
                {authStatus.profile && (
                  <div className="env-row">
                    <span className="env-key">Profile</span>
                    <span className="env-value">{authStatus.profile}</span>
                  </div>
                )}
                <div className="env-row">
                  <span className="env-key">Config file</span>
                  <span className="env-value">{env.configPresent ? env.configPath : "Not found"}</span>
                </div>
                <div className="env-row">
                  <span className="env-key">Default app ID</span>
                  <span className="env-value">{env.defaultAppId || "Not set"}</span>
                </div>
              </div>
              {!authConfigured && (
                <p className="settings-hint">
                  Run <code>asc auth login</code> in your terminal to set up credentials, then relaunch Studio.
                </p>
              )}
            </div>

            {/* ACP Provider */}
            <div className="workspace-section">
              <h3 className="section-label">ACP Provider</h3>
              <div className="settings-field">
                <label className="settings-label">Preferred preset</label>
                <div className="segmented">
                  {["codex", "claude", "custom"].map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      className={studioSettings.preferredPreset === preset ? "is-active" : ""}
                      onClick={() => updateSetting("preferredPreset", preset)}
                    >
                      {preset.charAt(0).toUpperCase() + preset.slice(1)}
                    </button>
                  ))}
                </div>
              </div>
              <div className="settings-field">
                <label className="settings-label" htmlFor="agent-command">Agent command</label>
                <input
                  id="agent-command"
                  className="settings-input"
                  type="text"
                  value={studioSettings.agentCommand}
                  onChange={(e) => updateSetting("agentCommand", e.target.value)}
                  placeholder="e.g. codex, claude-acp"
                />
              </div>
            </div>

            {/* ASC Binary */}
            <div className="workspace-section">
              <h3 className="section-label">ASC Binary</h3>
              <div className="settings-field">
                <label className="settings-toggle">
                  <input
                    type="checkbox"
                    checked={studioSettings.preferBundledASC}
                    onChange={(e) => updateSetting("preferBundledASC", e.target.checked)}
                  />
                  <span>Prefer bundled asc binary</span>
                </label>
              </div>
              <div className="settings-field">
                <label className="settings-label" htmlFor="asc-path">System asc path override</label>
                <input
                  id="asc-path"
                  className="settings-input"
                  type="text"
                  value={studioSettings.systemASCPath}
                  onChange={(e) => updateSetting("systemASCPath", e.target.value)}
                  placeholder="/usr/local/bin/asc"
                />
              </div>
            </div>

            {/* Workspace */}
            <div className="workspace-section">
              <h3 className="section-label">Workspace</h3>
              <div className="settings-field">
                <label className="settings-label" htmlFor="workspace-root">Workspace root</label>
                <input
                  id="workspace-root"
                  className="settings-input"
                  type="text"
                  value={studioSettings.workspaceRoot}
                  onChange={(e) => updateSetting("workspaceRoot", e.target.value)}
                  placeholder="~/Developer/my-app"
                />
              </div>
              <div className="settings-field">
                <label className="settings-toggle">
                  <input
                    type="checkbox"
                    checked={studioSettings.showCommandPreviews}
                    onChange={(e) => updateSetting("showCommandPreviews", e.target.checked)}
                  />
                  <span>Show command previews before execution</span>
                </label>
              </div>
            </div>

            <div className="workspace-section">
              <div className="settings-actions">
                <button className="settings-save" type="button" onClick={handleSaveSettings}>
                  Save settings
                </button>
                {settingsSaved && <span className="settings-saved-label">Saved</span>}
              </div>
            </div>
          </div>
        ) : !authConfigured ? (
          <div className="empty-state">
            <p className="empty-title">No credentials configured</p>
            <p className="empty-hint">
              Run <code>asc init</code> to create an API key profile, or go to Settings to check your configuration.
            </p>
            <button
              className="toolbar-btn"
              type="button"
              onClick={() => setActiveSection(allSections.find((s) => s.id === "settings")!)}
            >
              Open Settings
            </button>
          </div>
        ) : activeSection.id === "overview" && appDetail ? (
          <div className="app-detail-view">
            {/* Header */}
            <div className="app-detail-header">
              <p className="app-detail-name">{appDetail.name}</p>
              {appDetail.subtitle && <p className="app-detail-subtitle">{appDetail.subtitle}</p>}
            </div>

            {/* General info */}
            <div className="app-detail-section">
              <h3 className="section-label">General</h3>
              <div className="env-grid">
                <div className="env-row">
                  <span className="env-key">Bundle ID</span>
                  <span className="env-value mono">{appDetail.bundleId}</span>
                </div>
                <div className="env-row">
                  <span className="env-key">SKU</span>
                  <span className="env-value mono">{appDetail.sku}</span>
                </div>
                <div className="env-row">
                  <span className="env-key">Primary locale</span>
                  <span className="env-value">{appDetail.primaryLocale}</span>
                </div>
              </div>
            </div>

            {/* App Store metadata */}
            {metadataLoading ? (
              <div className="app-detail-section">
                <p className="empty-hint">Loading metadata…</p>
              </div>
            ) : allLocalizations.length > 0 ? (() => {
              const loc = allLocalizations.find((l) => l.locale === selectedLocale) ?? allLocalizations[0];
              return (
                <div className="app-detail-section">
                  <div className="metadata-header">
                    <h3 className="section-label" style={{ margin: 0 }}>App Store Metadata</h3>
                    <select
                      className="locale-picker"
                      value={selectedLocale}
                      onChange={(e) => handleLocaleChange(e.target.value)}
                    >
                      {allLocalizations.map((l) => (
                        <option key={l.locale} value={l.locale}>{l.locale}</option>
                      ))}
                    </select>
                  </div>

                  {/* Screenshots */}
                  {screenshotsLoading ? (
                    <div className="metadata-field">
                      <p className="metadata-label">Screenshots</p>
                      <p className="empty-hint" style={{ margin: 0 }}>Loading…</p>
                    </div>
                  ) : screenshotSets.length > 0 ? (
                    <div className="metadata-field">
                      <p className="metadata-label">Screenshots</p>
                      {screenshotSets.map((set) => {
                        const label = set.displayType
                          .replace(/^APP_/, "")
                          .replace(/_/g, " ")
                          .replace(/\b\w/g, (c) => c.toUpperCase());
                        return (
                          <div key={set.displayType} className="screenshot-set">
                            <p className="screenshot-set-label">{label}</p>
                            <div className="screenshot-row">
                              {set.screenshots.map((s, i) => (
                                <img
                                  key={i}
                                  src={s.thumbnailUrl}
                                  alt={`Screenshot ${i + 1}`}
                                  className={`screenshot-thumb ${s.width > s.height ? "landscape" : ""}`}
                                />
                              ))}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  ) : null}

                  {loc.promotionalText && (
                    <div className="metadata-field">
                      <p className="metadata-label">Promotional Text</p>
                      <p className="metadata-value">{loc.promotionalText}</p>
                    </div>
                  )}
                  {loc.description && (
                    <div className="metadata-field">
                      <p className="metadata-label">Description</p>
                      <p className="metadata-value metadata-multiline">{loc.description}</p>
                    </div>
                  )}
                  {loc.whatsNew && (
                    <div className="metadata-field">
                      <p className="metadata-label">What's New</p>
                      <p className="metadata-value metadata-multiline">{loc.whatsNew}</p>
                    </div>
                  )}
                  {loc.keywords && (
                    <div className="metadata-field">
                      <p className="metadata-label">Keywords</p>
                      <p className="metadata-value mono">{loc.keywords}</p>
                    </div>
                  )}
                  {(loc.supportUrl || loc.marketingUrl) && (
                    <div className="metadata-field">
                      <p className="metadata-label">URLs</p>
                      {loc.supportUrl && <p className="metadata-value mono">{loc.supportUrl}</p>}
                      {loc.marketingUrl && <p className="metadata-value mono">{loc.marketingUrl}</p>}
                    </div>
                  )}
                </div>
              );
            })() : null}
          </div>
        ) : activeSection.id === "status" && selectedAppId ? (
          <div className="app-detail-view">
            <div className="app-detail-section">
              <h3 className="section-label">Release Status</h3>
              {appStatus.loading ? (
                <p className="empty-hint">Loading…</p>
              ) : appStatus.error ? (
                <p className="empty-hint">{appStatus.error}</p>
              ) : appStatus.data ? (() => {
                const s = appStatus.data;
                return (
                  <>
                    {/* Health summary */}
                    <div className="status-health" style={{ marginBottom: 16 }}>
                      <span className={`status-pill status-health-${s.summary?.health}`} style={{ fontSize: 13, padding: "4px 10px" }}>
                        {s.summary?.health === "green" ? "Healthy" : s.summary?.health === "yellow" ? "Attention" : s.summary?.health === "red" ? "Blocked" : s.summary?.health}
                      </span>
                      {s.summary?.nextAction && <p style={{ margin: "8px 0 0", fontSize: 13, color: "var(--text-secondary)" }}>{s.summary.nextAction}</p>}
                    </div>

                    {/* Blockers */}
                    {(s.summary?.blockers?.length ?? 0) > 0 && (
                      <div className="status-blockers" style={{ marginBottom: 16 }}>
                        {s.summary!.blockers!.map((b: string, i: number) => (
                          <div key={i} className="blocker-row">
                            <span className="blocker-icon">!</span>
                            <span>{b}</span>
                          </div>
                        ))}
                      </div>
                    )}

                    <table className="data-table" style={{ marginBottom: 20 }}>
                      <tbody>
                        <tr><td className="vcard-label">App Store Version</td><td>{s.appstore?.version} — <span className={`status-pill status-${(s.appstore?.state ?? "").toLowerCase().replace(/_/g, "-")}`}>{fmt(s.appstore?.state ?? "")}</span></td></tr>
                        <tr><td className="vcard-label">Platform</td><td>{fmt(s.appstore?.platform ?? "")}</td></tr>
                        <tr><td className="vcard-label">Latest Build</td><td>{s.builds?.latest?.version} (#{s.builds?.latest?.buildNumber}) — <span className={`status-pill status-${(s.builds?.latest?.processingState ?? "").toLowerCase()}`}>{fmt(s.builds?.latest?.processingState ?? "")}</span></td></tr>
                        <tr><td className="vcard-label">Uploaded</td><td>{fmt(s.builds?.latest?.uploadedDate ?? "")}</td></tr>
                        <tr><td className="vcard-label">Review</td><td><span className={`status-pill status-${(s.review?.state ?? "").toLowerCase()}`}>{fmt(s.review?.state ?? "")}</span> {s.review?.submittedDate ? `(submitted ${s.review.submittedDate.split("T")[0]})` : ""}</td></tr>
                        <tr><td className="vcard-label">TestFlight</td><td><span className={`status-pill status-${(s.testflight?.betaReviewState ?? "").toLowerCase()}`}>{fmt(s.testflight?.betaReviewState ?? "")}</span></td></tr>
                        <tr><td className="vcard-label">Phased Release</td><td>{s.phasedRelease?.configured ? "Configured" : "Not configured"}</td></tr>
                        <tr><td className="vcard-label">Submission</td><td>{s.submission?.inFlight ? "In flight" : "None"}{s.submission?.blockingIssues?.length ? ` — ${s.submission.blockingIssues.length} blocking` : ""}</td></tr>
                      </tbody>
                    </table>

                    {/* Links */}
                    {s.links && (
                      <div style={{ marginTop: 8 }}>
                        <p className="metadata-label">Links</p>
                        <div style={{ display: "flex", gap: 12 }}>
                          {s.links.appStoreConnect && <a href={s.links.appStoreConnect} target="_blank" rel="noopener" style={{ color: "var(--accent)", fontSize: 12 }}>App Store Connect</a>}
                          {s.links.testFlight && <a href={s.links.testFlight} target="_blank" rel="noopener" style={{ color: "var(--accent)", fontSize: 12 }}>TestFlight</a>}
                          {s.links.review && <a href={s.links.review} target="_blank" rel="noopener" style={{ color: "var(--accent)", fontSize: 12 }}>Review</a>}
                        </div>
                      </div>
                    )}
                  </>
                );
              })() : null}
            </div>
          </div>
        ) : activeSection.id === "testflight" && selectedAppId ? (() => {
          // Detail view for a group's testers
          if (selectedGroup) {
            const group = testflightData.groups.find((g) => g.id === selectedGroup);
            // Compute state breakdown
            const stateCounts: Record<string, number> = {};
            for (const t of groupTesters.testers) {
              stateCounts[t.state] = (stateCounts[t.state] || 0) + 1;
            }
            return (
              <div className="app-detail-view">
                <div className="app-detail-section">
                  <button className="back-link" type="button" onClick={() => setSelectedGroup(null)}>← TestFlight</button>
                  <p className="app-detail-name" style={{ marginTop: 8 }}>{group?.name ?? "Group"}</p>
                  <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--text-secondary)" }}>
                    {group?.isInternal ? "Internal" : "External"} · {group?.testerCount ?? 0} testers
                    {group?.publicLink && <> · <a href={group.publicLink} target="_blank" rel="noopener" style={{ color: "var(--accent)" }}>TestFlight Link</a></>}
                  </p>

                  {groupTesters.loading ? (
                    <p className="empty-hint">Loading testers…</p>
                  ) : groupTesters.testers.length === 0 ? (
                    <p className="empty-hint">No testers in this group.</p>
                  ) : (
                    <>
                      {/* State summary */}
                      <div style={{ display: "flex", gap: 10, margin: "12px 0" }}>
                        {Object.entries(stateCounts).map(([state, count]) => (
                          <div key={state} style={{ textAlign: "center" }}>
                            <span style={{ fontSize: 20, fontWeight: 600, color: "var(--text-primary)" }}>{count}</span>
                            <p style={{ margin: 0, fontSize: 10, color: "var(--text-secondary)", textTransform: "uppercase" }}>{fmt(state)}</p>
                          </div>
                        ))}
                      </div>

                      {/* Tester table — only show testers with actual email/name data */}
                      <table className="data-table" style={{ marginTop: 8 }}>
                        <thead>
                          <tr>
                            <th>Email</th>
                            <th>Name</th>
                            <th>Invite</th>
                            <th>Status</th>
                          </tr>
                        </thead>
                        <tbody>
                          {groupTesters.testers.map((t, i) => (
                            <tr key={i}>
                              <td className="mono">{t.email || "—"}</td>
                              <td>{[t.firstName, t.lastName].filter(Boolean).join(" ") || "Anonymous"}</td>
                              <td>{fmt(t.inviteType)}</td>
                              <td><span className={`status-pill status-${t.state.toLowerCase()}`}>{fmt(t.state)}</span></td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                      <p style={{ marginTop: 8, fontSize: 11, color: "var(--text-secondary)" }}>
                        {groupTesters.testers.length} testers loaded
                      </p>
                    </>
                  )}
                </div>
              </div>
            );
          }
          // Groups list
          return (
            <div className="app-detail-view">
              <div className="app-detail-section">
                <h3 className="section-label">TestFlight</h3>
                {testflightData.loading ? (
                  <p className="empty-hint">Loading…</p>
                ) : testflightData.error ? (
                  <p className="empty-hint">{testflightData.error}</p>
                ) : testflightData.groups.length === 0 ? (
                  <p className="empty-hint">No beta groups found.</p>
                ) : (
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Group</th>
                        <th>Type</th>
                        <th>Testers</th>
                        <th>Public Link</th>
                        <th>Feedback</th>
                        <th>Created</th>
                      </tr>
                    </thead>
                    <tbody>
                      {testflightData.groups.map((g) => (
                        <tr key={g.id} className="clickable-row" onClick={() => {
                          const testerRequestID = groupTesterRequestRef.current + 1;
                          groupTesterRequestRef.current = testerRequestID;
                          setSelectedGroup(g.id);
                          setGroupTesters({ loading: true, testers: [] });
                          GetTestFlightTesters(g.id)
                            .then((res) => {
                              if (groupTesterRequestRef.current !== testerRequestID) return;
                              setGroupTesters({ loading: false, testers: res.testers ?? [] });
                            })
                            .catch(() => {
                              if (groupTesterRequestRef.current !== testerRequestID) return;
                              setGroupTesters({ loading: false, testers: [] });
                            });
                        }}>
                          <td style={{ fontWeight: 500 }}>{g.name}</td>
                          <td>{g.isInternal ? "Internal" : "External"}</td>
                          <td>{g.testerCount}</td>
                          <td>{g.publicLink ? <a href={g.publicLink} target="_blank" rel="noopener" style={{ color: "var(--accent)" }} onClick={(e) => e.stopPropagation()}>{g.publicLink.replace("https://testflight.apple.com/join/", "")}</a> : "—"}</td>
                          <td>{g.feedbackEnabled ? "On" : "Off"}</td>
                          <td>{g.createdDate.split("T")[0]}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          );
        })() : activeSection.id === "insights" && selectedAppId ? (() => {
          const today = new Date();
          const weekStr = insightsWeekStart(today);
          return (
            <div className="app-detail-view">
              <div className="app-detail-section">
                <h3 className="section-label">Weekly Insights</h3>
                <p style={{ fontSize: 12, color: "var(--text-secondary)", margin: "0 0 12px" }}>
                  Week of {weekStr} — analytics source
                </p>
                {(() => {
                  const cache = sectionCache["insights"];
                  if (!cache) {
                    // Fetch on first view
                    RunASCCommand(`insights weekly --app ${selectedAppId} --source analytics --week ${weekStr} --output json`)
                      .then((res) => {
                        if (res.error) {
                          setSectionCache((prev) => ({ ...prev, insights: { loading: false, error: res.error, items: [] } }));
                        } else {
                          try {
                            const d = JSON.parse(res.data);
                            const metrics = (d.metrics ?? []).map((m: Record<string, unknown>) => m);
                            setSectionCache((prev) => ({ ...prev, insights: { loading: false, items: metrics } }));
                          } catch {
                            setSectionCache((prev) => ({ ...prev, insights: { loading: false, error: "Failed to parse", items: [] } }));
                          }
                        }
                      });
                    setSectionCache((prev) => ({ ...prev, insights: { loading: true, items: [] } }));
                    return <p className="empty-hint">Loading…</p>;
                  }
                  if (cache.loading) return <p className="empty-hint">Loading…</p>;
                  if (cache.error) return <p className="empty-hint">{cache.error}</p>;
                  if (cache.items.length === 0) return <p className="empty-hint">No insights data.</p>;
                  return (
                    <table className="data-table">
                      <thead><tr><th>Metric</th><th>Status</th><th>Value</th></tr></thead>
                      <tbody>
                        {cache.items.map((m, i) => (
                          <tr key={i}>
                            <td>{String(m.name ?? "").replace(/_/g, " ")}</td>
                            <td><span className={`status-pill status-${String(m.status ?? "").toLowerCase()}`}>{fmt(String(m.status ?? ""))}</span></td>
                            <td>{m.thisWeek != null ? String(m.thisWeek) : (m.reason ? String(m.reason) : "—")}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  );
                })()}
              </div>
            </div>
          );
        })() : activeSection.id === "finance" && selectedAppId ? (
          <div className="app-detail-view">
            <div className="app-detail-section">
              <h3 className="section-label">Finance Regions</h3>
              {financeRegions.loading ? (
                <p className="empty-hint">Loading…</p>
              ) : financeRegions.error ? (
                <p className="empty-hint">{financeRegions.error}</p>
              ) : financeRegions.regions.length === 0 ? (
                <p className="empty-hint">No finance regions found.</p>
              ) : (
                <>
                  <div className="section-header-row">
                    <span className="section-count">{financeRegions.regions.length} regions</span>
                  </div>
                  <table className="data-table">
                    <thead><tr><th>Region</th><th>Code</th><th>Currency</th><th>Countries</th></tr></thead>
                    <tbody>
                      {financeRegions.regions.map((r) => (
                        <tr key={r.regionCode}>
                          <td style={{ fontWeight: 500 }}>{r.reportRegion}</td>
                          <td className="mono">{r.regionCode}</td>
                          <td>{r.reportCurrency}</td>
                          <td>{r.countriesOrRegions}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </>
              )}
            </div>
          </div>
        ) : activeSection.id === "pricing" && selectedAppId ? (
          <div className="app-detail-view">
            <div className="app-detail-section">
              <h3 className="section-label">Pricing and Availability</h3>
              {pricingOverview.loading ? (
                <p className="empty-hint">Loading…</p>
              ) : pricingOverview.error ? (
                <p className="empty-hint">{pricingOverview.error}</p>
              ) : (
                <>
                  <table className="data-table" style={{ marginBottom: 20 }}>
                    <tbody>
                      <tr>
                        <td className="vcard-label">Current Price</td>
                        <td style={{ fontWeight: 600 }}>
                          {pricingOverview.currentPrice === "0.0" || pricingOverview.currentPrice === "0.00"
                            ? "Free"
                            : pricingOverview.currentPrice
                              ? `${pricingOverview.baseCurrency} $${pricingOverview.currentPrice}`
                              : "—"}
                        </td>
                      </tr>
                      {pricingOverview.currentPrice && pricingOverview.currentPrice !== "0.0" && pricingOverview.currentPrice !== "0.00" && (
                        <tr>
                          <td className="vcard-label">Proceeds</td>
                          <td>{pricingOverview.baseCurrency} ${pricingOverview.currentProceeds}</td>
                        </tr>
                      )}
                      <tr>
                        <td className="vcard-label">Available in New Territories</td>
                        <td>{pricingOverview.availableInNewTerritories ? "Yes" : "No"}</td>
                      </tr>
                      <tr>
                        <td className="vcard-label">Territories</td>
                        <td>{pricingOverview.territories.filter((t) => t.available).length} available / {pricingOverview.territories.length} total</td>
                      </tr>
                    </tbody>
                  </table>

                  {pricingOverview.territories.length > 0 && (
                    <>
                      <h3 className="section-label">Territory Availability</h3>
                      <table className="data-table">
                        <thead>
                          <tr>
                            <th>Territory</th>
                            <th>Price</th>
                            <th>Available</th>
                            <th>Release Date</th>
                          </tr>
                        </thead>
                        <tbody>
                          {pricingOverview.territories.map((t) => (
                            <tr key={t.territory}>
                              <td>{t.territory}</td>
                              <td>{pricingOverview.currentPrice === "0.0" || pricingOverview.currentPrice === "0.00" ? "Free" : `$${pricingOverview.currentPrice}`}</td>
                              <td>{t.available ? "Yes" : "No"}</td>
                              <td>{t.releaseDate || "—"}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </>
                  )}

                  {pricingOverview.subscriptionPricing.length > 0 && (
                    <>
                      <h3 className="section-label">Subscription Prices</h3>
                      <table className="data-table">
                        <thead>
                          <tr>
                            <th>Group</th>
                            <th>Name</th>
                            <th>Period</th>
                            <th>Price</th>
                            <th>Proceeds</th>
                            <th>Status</th>
                          </tr>
                        </thead>
                        <tbody>
                          {pricingOverview.subscriptionPricing.map((s) => (
                            <tr key={s.productId}>
                              <td>{s.groupName}</td>
                              <td>{s.name}</td>
                              <td>{fmt(s.subscriptionPeriod)}</td>
                              <td>{s.currency} {s.price}</td>
                              <td>{s.currency} {s.proceeds}</td>
                              <td><span className={`status-pill status-${s.state.toLowerCase()}`}>{fmt(s.state)}</span></td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </>
                  )}
                </>
              )}
            </div>
          </div>
        ) : activeSection.id === "subscriptions" && selectedAppId ? (() => {
          const sub = selectedSub ? subscriptions.items.find((s) => s.id === selectedSub) : null;
          if (sub) {
            // Detail view for a single subscription
            return (
              <div className="app-detail-view">
                <div className="app-detail-section">
                  <button className="back-link" type="button" onClick={() => setSelectedSub(null)}>← Subscriptions</button>
                  <p className="app-detail-name" style={{ marginTop: 8 }}>{sub.name}</p>
                  <div className="env-grid" style={{ marginTop: 12 }}>
                    <div className="env-row">
                      <span className="env-key">Status</span>
                      <span className="env-value"><span className={`status-pill status-${sub.state.toLowerCase()}`}>{sub.state}</span></span>
                    </div>
                    <div className="env-row">
                      <span className="env-key">Product ID</span>
                      <span className="env-value mono">{sub.productId}</span>
                    </div>
                    <div className="env-row">
                      <span className="env-key">Subscription Duration</span>
                      <span className="env-value">{sub.subscriptionPeriod.replace(/_/g, " ").toLowerCase()}</span>
                    </div>
                    <div className="env-row">
                      <span className="env-key">Group</span>
                      <span className="env-value">{sub.groupName}</span>
                    </div>
                    <div className="env-row">
                      <span className="env-key">Group Level</span>
                      <span className="env-value">{sub.groupLevel}</span>
                    </div>
                    {sub.reviewNote && (
                      <div className="env-row">
                        <span className="env-key">Review Notes</span>
                        <span className="env-value">{sub.reviewNote}</span>
                      </div>
                    )}
                    <div className="env-row">
                      <span className="env-key">Apple ID</span>
                      <span className="env-value mono">{sub.id}</span>
                    </div>
                  </div>
                </div>
              </div>
            );
          }
          // List view
          return (
            <div className="app-detail-view">
              <div className="app-detail-section">
                <h3 className="section-label">Subscriptions</h3>
                {subscriptions.loading ? (
                  <p className="empty-hint">Loading…</p>
                ) : subscriptions.error ? (
                  <p className="empty-hint">{subscriptions.error}</p>
                ) : subscriptions.items.length === 0 ? (
                  <p className="empty-hint">No subscriptions found.</p>
                ) : (() => {
                  const groups = [...new Set(subscriptions.items.map((s) => s.groupName))];
                  return groups.map((group) => (
                    <div key={group} className="sub-group">
                      <p className="sub-group-name">{group}</p>
                      <table className="data-table">
                        <thead>
                          <tr>
                            <th>Name</th>
                            <th>Product ID</th>
                            <th>Period</th>
                            <th>Level</th>
                            <th>Status</th>
                          </tr>
                        </thead>
                        <tbody>
                          {subscriptions.items
                            .filter((s) => s.groupName === group)
                            .sort((a, b) => a.groupLevel - b.groupLevel)
                            .map((s) => (
                              <tr key={s.productId} className="clickable-row" onClick={() => setSelectedSub(s.id)}>
                                <td>{s.name}</td>
                                <td className="mono">{s.productId}</td>
                                <td>{s.subscriptionPeriod.replace(/_/g, " ").toLowerCase()}</td>
                                <td>{s.groupLevel}</td>
                                <td><span className={`status-pill status-${s.state.toLowerCase()}`}>{s.state}</span></td>
                              </tr>
                            ))}
                        </tbody>
                      </table>
                    </div>
                  ));
                })()}
              </div>
            </div>
          );
        })() : activeSection.id === "ratings-reviews" && selectedAppId ? (
          <div className="app-detail-view">
            <div className="app-detail-section">
              <div className="section-header-row">
                <h3 className="section-label">Ratings and Reviews</h3>
                {reviews.items.length > 0 && <span className="section-count">{reviews.items.length} reviews</span>}
              </div>
              {reviews.loading ? (
                <p className="empty-hint">Loading…</p>
              ) : reviews.error ? (
                <p className="empty-hint">{reviews.error}</p>
              ) : reviews.items.length === 0 ? (
                <p className="empty-hint">No reviews found.</p>
              ) : (
                <div className="reviews-list">
                  {reviews.items.map((r, i) => (
                    <div key={i} className="review-card">
                      <div className="review-header">
                        <span className="review-stars">{"★".repeat(r.rating)}{"☆".repeat(5 - r.rating)}</span>
                        <span className="review-meta">{r.reviewerNickname} · {r.territory} · {fmt(r.createdDate)}</span>
                      </div>
                      {r.title && <p className="review-title">{r.title}</p>}
                      {r.body && <p className="review-body">{r.body}</p>}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        ) : activeSection.id === "promo-codes" && selectedAppId ? (
          <div className="app-detail-view">
            <div className="app-detail-section">
              <h3 className="section-label">Offer Codes</h3>
              {offerCodes.loading ? (
                <p className="empty-hint">Loading…</p>
              ) : offerCodes.error ? (
                <p className="empty-hint">{offerCodes.error}</p>
              ) : offerCodes.codes.length === 0 ? (
                <p className="empty-hint">No offer codes found for this app's subscriptions.</p>
              ) : (
                <>
                  <div className="section-header-row">
                    <span className="section-count">{offerCodes.codes.length} offer codes</span>
                  </div>
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Subscription</th>
                        <th>Offer Name</th>
                        <th>Duration</th>
                        <th>Mode</th>
                        <th>Eligibility</th>
                        <th>Total Codes</th>
                        <th>Remaining</th>
                      </tr>
                    </thead>
                    <tbody>
                      {offerCodes.codes.map((c, i) => (
                        <tr key={i}>
                          <td style={{ fontWeight: 500 }}>{c.subscriptionName}</td>
                          <td>{c.name}</td>
                          <td>{fmt(c.duration)}</td>
                          <td>{fmt(c.offerMode)}</td>
                          <td>{fmt(c.offerEligibility)}</td>
                          <td>{c.totalNumberOfCodes}</td>
                          <td>{c.productionCodeCount}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </>
              )}
            </div>
          </div>
        ) : selectedAppId && sectionCommands[activeSection.id] ? (() => {
          const cache = sectionCache[activeSection.id];
          if (!cache || cache.loading) {
            return (
              <div className="app-detail-view">
                <div className="app-detail-section">
                  <h3 className="section-label">{activeSection.label}</h3>
                  <p className="empty-hint">Loading…</p>
                </div>
              </div>
            );
          }
          if (cache.error) {
            return (
              <div className="app-detail-view">
                <div className="app-detail-section">
                  <h3 className="section-label">{activeSection.label}</h3>
                  <p className="empty-hint">{cache.error}</p>
                </div>
              </div>
            );
          }
          if (cache.items.length === 0) {
            return (
              <div className="app-detail-view">
                <div className="app-detail-section">
                  <h3 className="section-label">{activeSection.label}</h3>
                  <p className="empty-hint">No data found.</p>
                </div>
              </div>
            );
          }
          // Build column list from all items' keys
          const allKeys = new Set<string>();
          for (const item of cache.items) {
            for (const [k, v] of Object.entries(item)) {
              if (k !== "id" && k !== "type" && v !== null && v !== undefined && v !== "" && typeof v !== "object") {
                allKeys.add(k);
              }
            }
          }
          const columns = [...allKeys];
          // Single-item views (like age-rating) render as key-value pairs
          if (cache.items.length === 1) {
            const item = cache.items[0];
            return (
              <div className="app-detail-view">
                <div className="app-detail-section">
                  <h3 className="section-label">{activeSection.label}</h3>
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Setting</th>
                        <th>Value</th>
                      </tr>
                    </thead>
                    <tbody>
                      {columns.map((key) => (
                        <tr key={key}>
                          <td>{fieldLabels[key] ?? key}</td>
                          <td>{fmt(String(item[key] ?? ""))}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            );
          }
          // Wide tables (>5 columns): render each item as a vertical card
          if (columns.length > 5) {
            return (
              <div className="app-detail-view">
                <div className="app-detail-section">
                  <div className="section-header-row">
                    <h3 className="section-label">{activeSection.label}</h3>
                    <span className="section-count">{cache.items.length} items</span>
                  </div>
                  {cache.items.map((item, idx) => (
                    <div key={item.id as string ?? idx} className="vertical-card">
                      <table className="data-table">
                        <tbody>
                          {columns.map((key) => {
                            const raw = item[key] != null ? String(item[key]) : "";
                            const display = fmt(raw);
                            const isState = key === "state" || key === "appVersionState" || key === "appStoreState";
                            return (
                              <tr key={key}>
                                <td className="vcard-label">{fieldLabels[key] ?? key}</td>
                                <td>{isState ? <span className={`status-pill status-${raw.toLowerCase().replace(/_/g, "-")}`}>{display}</span> : display}</td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  ))}
                </div>
              </div>
            );
          }
          return (
            <div className="app-detail-view">
              <div className="app-detail-section">
                <div className="section-header-row">
                  <h3 className="section-label">{activeSection.label}</h3>
                  <span className="section-count">{cache.items.length} items</span>
                </div>
                <table className="data-table">
                  <thead>
                    <tr>
                      {columns.map((col) => (
                        <th key={col}>{fieldLabels[col] ?? col}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {cache.items.map((item, idx) => (
                      <tr key={item.id as string ?? idx}>
                        {columns.map((col) => {
                          const val = item[col];
                          const isState = col === "state" || col === "appVersionState" || col === "appStoreState" || col === "processingState";
                          const raw = val != null ? String(val) : "";
                          const display = fmt(raw);
                          return (
                            <td key={col}>
                              {isState ? (
                                <span className={`status-pill status-${raw.toLowerCase().replace(/_/g, "-")}`}>
                                  {display}
                                </span>
                              ) : display}
                            </td>
                          );
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          );
        })() : (
          <div className="empty-state">
            <p className="empty-title">
              {!selectedAppId && activeSection.id !== "settings" ? "Select an App" : activeSection.label}
            </p>
            <p className="empty-hint">
              {!selectedAppId && activeSection.id !== "settings"
                ? "Use the dropdown in the sidebar to pick an app."
                : ""}
            </p>
          </div>
        )}

        {/* Chat dock — hidden on settings */}
        {activeSection.id !== "settings" && <section className={`dock ${dockExpanded ? "dock-expanded" : ""}`}>
          {dockExpanded && (
            <div className="dock-header">
              <span className="dock-title">ACP Chat</span>
              <button
                className="dock-collapse"
                type="button"
                onClick={() => setDockExpanded(false)}
                aria-label="Collapse chat"
              >
                ▾
              </button>
            </div>
          )}

          <div className="dock-body">
            {messages.length > 0 && (
              <div className="message-list" aria-label="Chat messages">
                {messages.map((message) => (
                  <article key={message.id} className={`message-row role-${message.role}`}>
                    <p>{message.content}</p>
                  </article>
                ))}
              </div>
            )}
          </div>

          <form className="composer" onSubmit={handleSubmit}>
            <div className="composer-card" onClick={() => !dockExpanded && setDockExpanded(true)}>
              <textarea
                aria-label="Chat prompt"
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                placeholder="Ask Studio to inspect builds, explain blockers, or draft a command…"
                rows={2}
              />
              <div className="composer-bar">
                <div className="composer-meta">
                  <span>Codex</span>
                  <span>Cursor</span>
                  <span>Custom ACP</span>
                </div>
                <button className="send-btn" type="submit" aria-label="Send">⬆</button>
              </div>
            </div>
          </form>
        </section>}
      </div>
    </div>
  );
}
