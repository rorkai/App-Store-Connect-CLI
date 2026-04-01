import { NavSection, Scope } from "./types";

export const scopes: Scope[] = [
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
        ],
      },
      {
        label: "TestFlight",
        items: [
          { id: "testflight", label: "Groups", description: "Beta groups and testers" },
          { id: "feedback", label: "Feedback", description: "Screenshot and crash feedback" },
          { id: "sandbox", label: "Sandbox", description: "Sandbox testers" },
        ],
      },
      {
        label: "Metadata",
        items: [
          { id: "localizations", label: "Localizations", description: "Locale metadata" },
          { id: "screenshots", label: "Screenshots", description: "App Store screenshots" },
          { id: "categories", label: "Categories", description: "App categories" },
          { id: "app-tags", label: "App Tags", description: "App tags" },
          { id: "pre-orders", label: "Pre-orders", description: "Pre-order configuration" },
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
          { id: "analytics", label: "Analytics", description: "Analytics reports" },
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
          { id: "actors", label: "Actors", description: "API key actors" },
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
      {
        label: "Commerce",
        items: [
          { id: "merchant-ids", label: "Merchant IDs", description: "Payment merchant IDs" },
          { id: "pass-type-ids", label: "Pass Type IDs", description: "Wallet pass types" },
        ],
      },
      {
        label: "Distribution",
        items: [
          { id: "notarization", label: "Notarization", description: "macOS notarization" },
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
          { id: "workflow", label: "Workflow", description: "Workflow orchestration" },
          { id: "notify", label: "Notifications", description: "Slack/webhook notifications" },
        ],
      },
      {
        label: "Tools",
        items: [
          { id: "schema", label: "Schema", description: "API schema browser" },
          { id: "diff", label: "Diff", description: "Version diff" },
          { id: "migrate", label: "Migrate", description: "Fastlane migration" },
        ],
      },
    ],
  },
];

// Flatten for lookup
export const allSections: NavSection[] = scopes.flatMap((s) => s.groups.flatMap((g) => g.items));
allSections.push({ id: "settings", label: "Settings", description: "Studio preferences" });

// Map section IDs to asc CLI commands. APP_ID is replaced at runtime.
export const sectionCommands: Record<string, string> = {
  "app-review": "review submissions-list --app APP_ID --output json",
  "history": "versions list --app APP_ID --output json",
  "builds": "builds list --app APP_ID --limit 20 --output json",
  "app-accessibility": "accessibility list --app APP_ID --output json",
  "in-app-events": "app-events list --app APP_ID --output json",
  "custom-product-pages": "product-pages custom-pages list --app APP_ID --output json",
  "ppo": "product-pages experiments list --v2 --app APP_ID --output json",
  "game-center": "game-center achievements list --app APP_ID --output json",
  "iap": "iap list --app APP_ID --output json",
  "nominations": "nominations list --status DRAFT --output json",
  "performance": "performance metrics list --app APP_ID --output json",
  "localizations": "localizations list --app APP_ID --type app-info --output json",
  "categories": "categories list --output json",
  "age-rating": "age-rating view --app APP_ID --output json",
  "encryption": "encryption declarations list --app APP_ID --output json",
  "account-status": "account status --output json",
  "users": "users list --output json",
  "devices": "devices list --output json",
  "bundle-ids": "bundle-ids list --paginate --output json",
  "certificates": "certificates list --paginate --output json",
  "profiles": "profiles list --paginate --output json",
  "xcode-cloud": "xcode-cloud workflows list --app APP_ID --output json",
  "webhooks": "webhooks list --app APP_ID --output json",
  "pre-orders": "pre-orders view --app APP_ID --output json",
  "app-tags": "app-tags list --app APP_ID --output json",
  "app-clips": "app-clips list --app APP_ID --output json",
  "android-ios-mapping": "android-ios-mapping list --app APP_ID --output json",
  "marketplace": "marketplace search-details view --app APP_ID --output json",
  "alt-distribution": "alternative-distribution domains list --output json",
  "eula": "eula view --app APP_ID --output json",
  "sandbox": "sandbox list --output json",
  "merchant-ids": "merchant-ids list --output json",
  "pass-type-ids": "pass-type-ids list --output json",
  "analytics": "analytics requests --app APP_ID --output json",
};

// Human-readable field labels for known attribute keys
export const fieldLabels: Record<string, string> = {
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
export const displayValue: Record<string, string> = {
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

// Human-readable screenshot display type labels
export const screenshotDisplayLabels: Record<string, string> = {
  APP_IPHONE_67: "iPhone 6.7\"",
  APP_IPHONE_65: "iPhone 6.5\"",
  APP_IPHONE_61: "iPhone 6.1\"",
  APP_IPHONE_58: "iPhone 5.8\"",
  APP_IPHONE_55: "iPhone 5.5\"",
  APP_IPHONE_47: "iPhone 4.7\"",
  APP_IPHONE_40: "iPhone 4\"",
  APP_IPHONE_35: "iPhone 3.5\"",
  APP_IPAD_PRO_3GEN_129: "iPad Pro 12.9\" (3rd gen)",
  APP_IPAD_PRO_3GEN_11: "iPad Pro 11\"",
  APP_IPAD_PRO_129: "iPad Pro 12.9\"",
  APP_IPAD_105: "iPad 10.5\"",
  APP_IPAD_97: "iPad 9.7\"",
  APP_DESKTOP: "Mac",
  APP_WATCH_SERIES_7: "Apple Watch Series 7",
  APP_WATCH_SERIES_4: "Apple Watch Series 4",
  APP_WATCH_SERIES_3: "Apple Watch Series 3",
  APP_WATCH_ULTRA: "Apple Watch Ultra",
  APP_APPLE_TV: "Apple TV",
  APP_VISION_PRO: "Apple Vision Pro",
};

export const bundleIDPlatformOrder: Record<string, number> = {
  IOS: 0,
  UNIVERSAL: 1,
  MAC_OS: 2,
  TV_OS: 3,
  VISION_OS: 4,
  WATCH_OS: 5,
};

export const appScopedSectionIDs = Object.keys(sectionCommands).filter(
  (sectionId) => sectionCommands[sectionId]?.includes("APP_ID") ?? false,
);

export const emptyEnv = {
  configPath: "",
  configPresent: false,
  defaultAppId: "",
  keychainAvailable: false,
  keychainBypassed: false,
  keychainWarning: "",
  workflowPath: "",
} as const;

export const defaultSettings = {
  preferredPreset: "codex",
  agentCommand: "",
  agentArgs: [] as string[],
  agentEnv: {} as Record<string, string>,
  preferBundledASC: true,
  systemASCPath: "",
  workspaceRoot: "",
  showCommandPreviews: true,
  theme: "system",
  windowMaterial: "translucent",
} as const;

export const emptyAuthStatus = {
  authenticated: false,
  storage: "",
  profile: "",
  rawOutput: "",
} as const;
