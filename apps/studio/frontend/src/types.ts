export type NavSection = {
  id: string;
  label: string;
  description: string;
};

export type ChatMessage = {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  timestamp: string;
};

export type SidebarGroup = { label: string; items: NavSection[] };
export type Scope = { id: string; label: string; groups: SidebarGroup[] };

export type EnvSnapshot = {
  configPath: string;
  configPresent: boolean;
  defaultAppId: string;
  keychainAvailable: boolean;
  keychainBypassed: boolean;
  keychainWarning?: string;
  workflowPath: string;
};

export type StudioSettings = {
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

export type AuthState = {
  authenticated: boolean;
  storage: string;
  profile: string;
  rawOutput: string;
};

export type AppStatusData = {
  summary?: { health: string; nextAction?: string; blockers?: string[] };
  builds?: { latest?: { version: string; buildNumber: string; processingState: string; uploadedDate: string; platform: string } };
  testflight?: { betaReviewState: string; submittedDate?: string };
  appstore?: { version: string; state: string; platform: string; createdDate: string; versionId: string };
  submission?: { inFlight: boolean; blockingIssues?: string[] };
  review?: { state: string; submittedDate?: string; platform?: string };
  phasedRelease?: { configured: boolean };
  links?: { appStoreConnect?: string; testFlight?: string; review?: string };
};

export type AppDetail = {
  id: string;
  name: string;
  subtitle: string;
  bundleId: string;
  sku: string;
  primaryLocale: string;
  versions: { id: string; platform: string; version: string; state: string }[];
  error?: string;
};

export type AppListItem = {
  id: string;
  name: string;
  subtitle: string;
};

export type LocalizationEntry = {
  localizationId: string;
  locale: string;
  description: string;
  keywords: string;
  whatsNew: string;
  promotionalText: string;
  supportUrl: string;
  marketingUrl: string;
};

export type ScreenshotSet = {
  displayType: string;
  screenshots: { thumbnailUrl: string; width: number; height: number }[];
};

export type SectionCacheEntry = {
  loading: boolean;
  error?: string;
  items: Record<string, unknown>[];
};

export type AppStatusState = {
  loading: boolean;
  error?: string;
  data: AppStatusData | null;
};

export type TestFlightGroup = {
  id: string;
  name: string;
  isInternal: boolean;
  publicLink: string;
  feedbackEnabled: boolean;
  createdDate: string;
  testerCount: number;
};

export type TestFlightState = {
  loading: boolean;
  error?: string;
  groups: TestFlightGroup[];
};

export type Tester = {
  email: string;
  firstName: string;
  lastName: string;
  inviteType: string;
  state: string;
};

export type GroupTestersState = {
  loading: boolean;
  error?: string;
  testers: Tester[];
};

export type ReviewItem = {
  rating: number;
  title: string;
  body: string;
  reviewerNickname: string;
  createdDate: string;
  territory: string;
};

export type ReviewsState = {
  loading: boolean;
  error?: string;
  items: ReviewItem[];
};

export type SubscriptionItem = {
  id: string;
  groupName: string;
  name: string;
  productId: string;
  state: string;
  subscriptionPeriod: string;
  reviewNote: string;
  groupLevel: number;
};

export type SubscriptionsState = {
  loading: boolean;
  error?: string;
  items: SubscriptionItem[];
};

export type TerritoryEntry = {
  territory: string;
  available: boolean;
  releaseDate: string;
};

export type SubscriptionPricingEntry = {
  name: string;
  productId: string;
  subscriptionPeriod: string;
  state: string;
  groupName: string;
  price: string;
  currency: string;
  proceeds: string;
};

export type PricingOverviewState = {
  loading: boolean;
  error?: string;
  availableInNewTerritories: boolean;
  currentPrice: string;
  currentProceeds: string;
  baseCurrency: string;
  territories: TerritoryEntry[];
  subscriptionPricing: SubscriptionPricingEntry[];
};

export type FinanceRegion = {
  reportRegion: string;
  reportCurrency: string;
  regionCode: string;
  countriesOrRegions: string;
};

export type FinanceRegionsState = {
  loading: boolean;
  error?: string;
  regions: FinanceRegion[];
};

export type OfferCode = {
  subscriptionName: string;
  subscriptionId: string;
  name: string;
  offerEligibility: string;
  customerEligibilities: string[];
  duration: string;
  offerMode: string;
  numberOfPeriods: number;
  totalNumberOfCodes: number;
  productionCodeCount: number;
};

export type OfferCodesState = {
  loading: boolean;
  error?: string;
  loadedAppId?: string;
  codes: OfferCode[];
};

export type FeedbackItem = {
  id: string;
  comment: string;
  email: string;
  deviceModel: string;
  deviceFamily: string;
  osVersion: string;
  appPlatform: string;
  createdDate: string;
  locale: string;
  timeZone: string;
  connectionType: string;
  batteryPercentage: number;
  screenshots: { url: string; width: number; height: number }[];
};

export type FeedbackState = {
  loading: boolean;
  error?: string;
  total: number;
  items: FeedbackItem[];
};
