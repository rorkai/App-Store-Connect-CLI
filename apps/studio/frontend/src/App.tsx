import { lazy, Suspense, useCallback, useState } from "react";

import "./styles.css";
import { NavSection } from "./types";
import { allSections, sectionCommands } from "./constants";
import { sectionRequiresApp, insightsWeekStart } from "./utils";
import { useTheme } from "./hooks/useTheme";
import { useBootstrap } from "./hooks/useBootstrap";
import { useAppSelection } from "./hooks/useAppSelection";
import { useBundleIDSheet, useDeviceSheet } from "./hooks/useSheetForm";
import { useChatDock } from "./hooks/useChatDock";
import { useFocusTrap } from "./hooks/useFocusTrap";
import { Sidebar } from "./components/Sidebar";
import { ContextBar } from "./components/ContextBar";
import { ChatDock } from "./components/ChatDock";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { RunASCCommand } from "../wailsjs/go/main/App";

// Lazy-load all view components for code splitting
const SettingsView = lazy(() => import("./components/views/SettingsView").then(m => ({ default: m.SettingsView })));
const AppInfoView = lazy(() => import("./components/views/AppInfoView").then(m => ({ default: m.AppInfoView })));
const StatusView = lazy(() => import("./components/views/StatusView").then(m => ({ default: m.StatusView })));
const TestFlightView = lazy(() => import("./components/views/TestFlightView").then(m => ({ default: m.TestFlightView })));
const FeedbackView = lazy(() => import("./components/views/FeedbackView").then(m => ({ default: m.FeedbackView })));
const SubscriptionsView = lazy(() => import("./components/views/SubscriptionsView").then(m => ({ default: m.SubscriptionsView })));
const PricingView = lazy(() => import("./components/views/PricingView").then(m => ({ default: m.PricingView })));
const ReviewsView = lazy(() => import("./components/views/ReviewsView").then(m => ({ default: m.ReviewsView })));
const ScreenshotsView = lazy(() => import("./components/views/ScreenshotsView").then(m => ({ default: m.ScreenshotsView })));
const InsightsView = lazy(() => import("./components/views/InsightsView").then(m => ({ default: m.InsightsView })));
const FinanceView = lazy(() => import("./components/views/FinanceView").then(m => ({ default: m.FinanceView })));
const PromoCodesView = lazy(() => import("./components/views/PromoCodesView").then(m => ({ default: m.PromoCodesView })));
const GenericTableView = lazy(() => import("./components/views/GenericTableView").then(m => ({ default: m.GenericTableView })));
const ToolView = lazy(() => import("./components/views/ToolView").then(m => ({ default: m.ToolView })));

export { insightsWeekStart } from "./utils";

function ViewFallback() {
  return (
    <div className="empty-state" role="status">
      <p className="empty-hint">Loading…</p>
    </div>
  );
}

// Tool view config (static)
const toolViews: Record<string, { title: string; description: string; commandHint: string | ((appId: string | null) => string) }> = {
  diff: { title: "Diff", description: "Generate deterministic diff plans between app versions.", commandHint: (appId) => `Use the ACP chat to run: asc diff metadata --app ${appId || "APP_ID"}` },
  actors: { title: "Actors", description: "Actors are users and API keys that appear in audit fields (e.g. submittedByActor). Look up an actor by ID:", commandHint: "asc actors view --id ACTOR_ID" },
  migrate: { title: "Migrate", description: "Migrate from Fastlane to asc.", commandHint: "Use ACP chat: asc migrate import --fastfile ./Fastfile" },
  notify: { title: "Notifications", description: "Send notifications via Slack or webhooks.", commandHint: "Use ACP chat: asc notify slack --webhook $WEBHOOK --message \"Build ready\"" },
  notarization: { title: "Notarization", description: "Submit macOS apps for Apple notarization.", commandHint: "Use the ACP chat to run: asc notarization submit --file ./MyApp.zip" },
  crashes: { title: "Crashes", description: "Crash diagnostics are per-build. Select a build ID from the Builds section to view diagnostics.", commandHint: "asc performance diagnostics list --build BUILD_ID --output json" },
  "app-setup": { title: "App Setup", description: "Post-create app configuration: locale, categories, availability, pricing.", commandHint: (appId) => `asc app-setup info set --app ${appId || "APP_ID"} --primary-locale "en-US"` },
  "routing-coverage": { title: "Routing Coverage", description: "Routing app coverage files require a version ID.", commandHint: "asc routing-coverage view --version-id VERSION_ID" },
  "build-localizations": { title: "Build Localizations", description: "Release notes per build. Requires a build ID.", commandHint: "asc build-localizations list --build BUILD_ID --output json" },
  "build-bundles": { title: "Build Bundles", description: "Build bundle information. Requires a build ID.", commandHint: "asc build-bundles list --build BUILD_ID --output json" },
  schema: { title: "Schema", description: "Browse the App Store Connect API schema.", commandHint: "asc schema index --output json" },
  metadata: { title: "Metadata", description: "Pull and push app metadata.", commandHint: (appId) => `asc metadata pull --app ${appId || "APP_ID"} --dir ./metadata` },
  agreements: { title: "Agreements", description: "Territory agreements for EULA. Requires an EULA ID.", commandHint: "asc agreements territories list --id EULA_ID --output json" },
  workflow: { title: "Workflow", description: "List and run asc workflows.", commandHint: "asc workflow list" },
};

export default function App() {
  const [activeScope, setActiveScope] = useState<string>("app");
  const [activeSection, setActiveSection] = useState<NavSection>(allSections[0]);
  const [appSearchTerm, setAppSearchTerm] = useState("");
  const [sectionSearchTerms, setSectionSearchTerms] = useState<Record<string, string>>({});
  const [bundleIDsPlatformSort, setBundleIDsPlatformSort] = useState<"asc" | "desc">("asc");

  const bootstrap = useBootstrap();
  const app = useAppSelection();
  const chat = useChatDock();
  const { resolvedTheme } = useTheme(bootstrap.studioSettings.theme);

  const bundleIDSheet = useBundleIDSheet(() => app.loadStandaloneSection("bundle-ids", true));
  const deviceSheet = useDeviceSheet(() => app.loadStandaloneSection("devices", true));

  const sheetOpen = bundleIDSheet.state.open || deviceSheet.state.open;
  const closeSheet = useCallback(() => {
    if (bundleIDSheet.state.open) bundleIDSheet.dispatch({ type: "close" });
    if (deviceSheet.state.open) deviceSheet.dispatch({ type: "close" });
  }, [bundleIDSheet.state.open, deviceSheet.state.open]);
  const { trapRef, onTrapKeyDown } = useFocusTrap(sheetOpen, closeSheet);

  const filteredApps = bootstrap.appList.filter((a) =>
    `${a.name} ${a.subtitle}`.toLowerCase().includes(appSearchTerm.trim().toLowerCase()),
  );
  const activeSectionSearch = sectionSearchTerms[activeSection.id] ?? "";
  const insightsWeek = insightsWeekStart(new Date());
  const insightsCache = app.sectionCache.insights;

  function handleSelectApp(id: string) {
    app.handleSelectApp(id, activeSection.id);
  }

  function handleSetActiveSection(section: NavSection) {
    setActiveSection(section);
    if (sectionCommands[section.id] && !sectionRequiresApp(section.id)) {
      app.loadStandaloneSectionIfNeeded(section.id);
    }
    app.loadOfferCodesIfNeeded(section.id, app.selectedAppId);
    app.loadInsightsIfNeeded(section.id, app.selectedAppId);
  }

  function handleRefresh() {
    bootstrap.handleRefresh(app.selectedAppId, handleSelectApp);
  }

  function renderContent() {
    if (bootstrap.loading) {
      return <ViewFallback />;
    }
    if (bootstrap.bootstrapError) {
      return (
        <div className="empty-state">
          <p className="empty-title">Bootstrap failed</p>
          <p className="empty-hint">{bootstrap.bootstrapError}</p>
        </div>
      );
    }
    if (activeSection.id === "settings") {
      return (
        <SettingsView
          authStatus={bootstrap.authStatus}
          env={bootstrap.env}
          studioSettings={bootstrap.studioSettings}
          settingsSaved={bootstrap.settingsSaved}
          updateSetting={bootstrap.updateSetting}
          handleSaveSettings={bootstrap.handleSaveSettings}
        />
      );
    }
    if (!bootstrap.authStatus.authenticated) {
      return (
        <div className="empty-state">
          <p className="empty-title">No credentials configured</p>
          <p className="empty-hint">
            Run <code>asc init</code> to create an API key profile, or go to Settings to check your configuration.
          </p>
          <button className="toolbar-btn" type="button" onClick={() => handleSetActiveSection(allSections.find((s) => s.id === "settings")!)}>
            Open Settings
          </button>
        </div>
      );
    }
    if (activeScope === "app" && !app.selectedAppId && bootstrap.appsError) {
      return (
        <div className="empty-state">
          <p className="empty-title">Failed to load apps</p>
          <p className="empty-hint">{bootstrap.appsError}</p>
        </div>
      );
    }
    if (activeSection.id === "overview" && app.appDetail?.error) {
      return (
        <div className="empty-state">
          <p className="empty-title">Overview unavailable</p>
          <p className="empty-hint">{app.appDetail.error}</p>
        </div>
      );
    }
    if (activeSection.id === "overview" && app.appDetail) {
      return (
        <AppInfoView appDetail={app.appDetail} selectedAppId={app.selectedAppId} metadataLoading={app.metadataLoading}
          metadataError={app.metadataError} allLocalizations={app.allLocalizations} selectedLocale={app.selectedLocale} screenshotsLoading={app.screenshotsLoading}
          screenshotsError={app.screenshotsError}
          screenshotSets={app.screenshotSets} onLocaleChange={app.handleLocaleChange} onRunCommand={RunASCCommand} />
      );
    }
    if (activeSection.id === "status" && app.selectedAppId) return <StatusView appStatus={app.appStatus} />;
    if (activeSection.id === "testflight" && app.selectedAppId) {
      return <TestFlightView testflightData={app.testflightData} selectedGroup={app.selectedGroup} groupTesters={app.groupTesters}
        onSelectGroup={app.handleSelectGroup} onBackToGroups={app.handleBackToGroups} />;
    }
    if (activeSection.id === "insights" && app.selectedAppId) return <InsightsView insightsWeek={insightsWeek} insightsCache={insightsCache} />;
    if (activeSection.id === "finance" && app.selectedAppId) return <FinanceView financeRegions={app.financeRegions} />;
    if (activeSection.id === "pricing" && app.selectedAppId) return <PricingView pricingOverview={app.pricingOverview} />;
    if (activeSection.id === "subscriptions" && app.selectedAppId) {
      return <SubscriptionsView subscriptions={app.subscriptions} selectedSub={app.selectedSub} onSelectSub={app.setSelectedSub} />;
    }
    if (activeSection.id === "ratings-reviews" && app.selectedAppId) return <ReviewsView reviews={app.reviews} />;
    if (activeSection.id === "screenshots" && app.selectedAppId) {
      return <ScreenshotsView screenshotsLoading={app.screenshotsLoading} screenshotsError={app.screenshotsError}
        screenshotSets={app.screenshotSets}
        allLocalizations={app.allLocalizations} selectedLocale={app.selectedLocale} onLocaleChange={app.handleLocaleChange} />;
    }
    if (activeSection.id === "feedback" && app.selectedAppId) return <FeedbackView feedbackData={app.feedbackData} />;
    if (activeSection.id === "promo-codes" && app.selectedAppId) return <PromoCodesView offerCodes={app.offerCodes} />;
    if (toolViews[activeSection.id]) {
      const tv = toolViews[activeSection.id];
      const hint = typeof tv.commandHint === "function" ? tv.commandHint(app.selectedAppId) : tv.commandHint;
      return <ToolView title={tv.title} description={tv.description} commandHint={hint} />;
    }
    if (sectionCommands[activeSection.id] && (!sectionRequiresApp(activeSection.id) || app.selectedAppId)) {
      const cache = app.sectionCache[activeSection.id];
      return (
        <GenericTableView activeSection={activeSection} cache={cache ?? { loading: true, items: [] }}
          bundleIDsPlatformSort={bundleIDsPlatformSort} activeSectionSearch={activeSectionSearch}
          onSetSectionSearch={(id, term) => setSectionSearchTerms((prev) => ({ ...prev, [id]: term }))}
          onToggleBundleIDSort={() => setBundleIDsPlatformSort((prev) => prev === "asc" ? "desc" : "asc")}
          onOpenBundleIDSheet={() => bundleIDSheet.dispatch({ type: "open" })}
          onOpenDeviceSheet={() => deviceSheet.dispatch({ type: "open" })} />
      );
    }
    return (
      <div className="empty-state">
        <p className="empty-title">{!app.selectedAppId && activeSection.id !== "settings" ? "Select an App" : activeSection.label}</p>
        <p className="empty-hint">{!app.selectedAppId && activeSection.id !== "settings" ? "Use search in the sidebar to pick an app." : ""}</p>
      </div>
    );
  }

  return (
    <div className="studio-shell" data-theme={resolvedTheme}>
      <a href="#main-content" className="skip-nav">Skip to main content</a>
      <Sidebar
        activeScope={activeScope} selectedAppId={app.selectedAppId} appDetail={app.appDetail}
        appList={bootstrap.appList} appSearchTerm={appSearchTerm} activeSection={activeSection}
        appsLoading={bootstrap.appsLoading} appsError={bootstrap.appsError} authAuthenticated={bootstrap.authStatus.authenticated}
        filteredApps={filteredApps} onAppSearchChange={setAppSearchTerm}
        onSelectApp={handleSelectApp} onSetActiveSection={handleSetActiveSection}
      />

      <div className="shell-separator" />

      <main id="main-content" className="main-area">
        <ContextBar
          authStatus={bootstrap.authStatus} activeScope={activeScope}
          handleRefresh={handleRefresh} setActiveScope={setActiveScope}
          setActiveSection={handleSetActiveSection}
        />

        <ErrorBoundary>
          <Suspense fallback={<ViewFallback />}>
            {renderContent()}
          </Suspense>
        </ErrorBoundary>

        {activeSection.id !== "settings" && (
          <ChatDock messages={chat.messages} draft={chat.draft} dockExpanded={chat.dockExpanded}
            handleSubmit={chat.handleSubmit} setDraft={chat.setDraft} setDockExpanded={chat.setDockExpanded} />
        )}
      </main>

      {bundleIDSheet.state.open && (
        <div className="sheet-backdrop" role="presentation" onClick={() => bundleIDSheet.dispatch({ type: "close" })}>
          <section ref={trapRef} className="sheet-panel" role="dialog" aria-modal="true" aria-labelledby="bundle-id-sheet-title"
            onKeyDown={onTrapKeyDown}
            onClick={(e) => e.stopPropagation()}>
            <div className="sheet-header">
              <div>
                <p className="sheet-eyebrow">Signing</p>
                <h2 id="bundle-id-sheet-title" className="sheet-title">Create Bundle ID</h2>
              </div>
              <button type="button" className="sheet-close" onClick={() => bundleIDSheet.dispatch({ type: "close" })} aria-label="Close create bundle ID sheet">&times;</button>
            </div>
            <div className="sheet-body">
              <label className="sheet-field">
                <span className="sheet-label">Name</span>
                <input type="text" value={bundleIDSheet.state.name} onChange={(e) => bundleIDSheet.dispatch({ type: "setName", value: e.target.value })} placeholder="Example App" />
              </label>
              <label className="sheet-field">
                <span className="sheet-label">Identifier</span>
                <input type="text" value={bundleIDSheet.state.identifier} onChange={(e) => bundleIDSheet.dispatch({ type: "setIdentifier", value: e.target.value })} placeholder="com.example.app" />
              </label>
              <label className="sheet-field">
                <span className="sheet-label">Platform</span>
                <select value={bundleIDSheet.state.platform} onChange={(e) => bundleIDSheet.dispatch({ type: "setPlatform", value: e.target.value })}>
                  <option value="IOS">iOS</option><option value="MAC_OS">macOS</option><option value="TV_OS">tvOS</option><option value="VISION_OS">visionOS</option>
                </select>
              </label>
              <div className="sheet-preview"><p className="sheet-label">Command preview</p><code>{bundleIDSheet.commandPreview}</code></div>
              {bundleIDSheet.state.error && <p className="sheet-error" role="alert">{bundleIDSheet.state.error}</p>}
            </div>
            <div className="sheet-footer">
              <button type="button" className="toolbar-btn" onClick={() => bundleIDSheet.dispatch({ type: "close" })}>Cancel</button>
              <button type="button" className="toolbar-btn toolbar-btn-primary" onClick={bundleIDSheet.handleCreate} disabled={bundleIDSheet.state.creating}>
                {bundleIDSheet.state.creating ? "Creating…" : "Create"}
              </button>
            </div>
          </section>
        </div>
      )}

      {deviceSheet.state.open && (
        <div className="sheet-backdrop" role="presentation" onClick={() => deviceSheet.dispatch({ type: "close" })}>
          <section ref={trapRef} className="sheet-panel" role="dialog" aria-modal="true" aria-labelledby="device-sheet-title"
            onKeyDown={onTrapKeyDown}
            onClick={(e) => e.stopPropagation()}>
            <div className="sheet-header">
              <div>
                <p className="sheet-eyebrow">Team</p>
                <h2 id="device-sheet-title" className="sheet-title">Register Device</h2>
              </div>
              <button type="button" className="sheet-close" onClick={() => deviceSheet.dispatch({ type: "close" })} aria-label="Close register device sheet">&times;</button>
            </div>
            <div className="sheet-body">
              <label className="sheet-field">
                <span className="sheet-label">Name</span>
                <input type="text" value={deviceSheet.state.name} onChange={(e) => deviceSheet.dispatch({ type: "setName", value: e.target.value })} placeholder="Rudrank's iPhone" />
              </label>
              <label className="sheet-field">
                <span className="sheet-label">UDID</span>
                <input type="text" value={deviceSheet.state.identifier} onChange={(e) => deviceSheet.dispatch({ type: "setIdentifier", value: e.target.value })} placeholder="00008110-001234560E90003A" />
              </label>
              <label className="sheet-field">
                <span className="sheet-label">Platform</span>
                <select value={deviceSheet.state.platform} onChange={(e) => deviceSheet.dispatch({ type: "setPlatform", value: e.target.value })}>
                  <option value="IOS">iOS</option><option value="MAC_OS">macOS</option><option value="TV_OS">tvOS</option><option value="VISION_OS">visionOS</option>
                </select>
              </label>
              <div className="sheet-preview"><p className="sheet-label">Command preview</p><code>{deviceSheet.commandPreview}</code></div>
              {deviceSheet.state.error && <p className="sheet-error" role="alert">{deviceSheet.state.error}</p>}
            </div>
            <div className="sheet-footer">
              <button type="button" className="toolbar-btn" onClick={() => deviceSheet.dispatch({ type: "close" })}>Cancel</button>
              <button type="button" className="toolbar-btn toolbar-btn-primary" onClick={deviceSheet.handleCreate} disabled={deviceSheet.state.creating}>
                {deviceSheet.state.creating ? "Registering…" : "Register"}
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
