import { useRef, useState, type MutableRefObject } from "react";

import {
  AppStatusState,
  FeedbackState,
  FinanceRegionsState,
  OfferCodesState,
  PricingOverviewState,
  ReviewsState,
  SectionCacheEntry,
  SubscriptionsState,
} from "../../types";
import { appScopedSectionIDs, sectionCommands } from "../../constants";
import { commandForApp, insightsWeekStart, sectionRequiresApp, shellQuote } from "../../utils";
import {
  GetFeedback,
  GetFinanceRegions,
  GetOfferCodes,
  GetPricingOverview,
  GetSubscriptions,
  RunASCCommand,
} from "../../../wailsjs/go/main/App";
import { parseCommandItems } from "./commandItems";
import { appSectionPrefetchConcurrency, runWithConcurrency } from "./concurrency";

function emptyPricingOverview(): PricingOverviewState {
  return {
    loading: false,
    availableInNewTerritories: false,
    currentPrice: "",
    currentProceeds: "",
    baseCurrency: "",
    territories: [],
    subscriptionPricing: [],
  };
}

function emptyOfferCodes(): OfferCodesState {
  return { loading: false, loadedAppId: "", codes: [] };
}

function emptyFeedback(): FeedbackState {
  return { loading: false, total: 0, items: [] };
}

export function useAppSectionData(appSelectionRequestRef: MutableRefObject<number>) {
  const [sectionCache, setSectionCache] = useState<Record<string, SectionCacheEntry>>({});
  const [appStatus, setAppStatus] = useState<AppStatusState>({ loading: false, data: null });
  const [reviews, setReviews] = useState<ReviewsState>({ loading: false, items: [] });
  const [subscriptions, setSubscriptions] = useState<SubscriptionsState>({ loading: false, items: [] });
  const [pricingOverview, setPricingOverview] = useState<PricingOverviewState>(emptyPricingOverview());
  const [selectedSub, setSelectedSub] = useState<string | null>(null);
  const [financeRegions, setFinanceRegions] = useState<FinanceRegionsState>({ loading: false, regions: [] });
  const [offerCodes, setOfferCodes] = useState<OfferCodesState>(emptyOfferCodes());
  const [feedbackData, setFeedbackData] = useState<FeedbackState>(emptyFeedback());

  const insightsRequestRef = useRef(0);
  const offerCodesRequestRef = useRef(0);

  function resetSelection() {
    setSectionCache({});
    setSelectedSub(null);
  }

  function prefetchSections(appId: string, requestID: number) {
    const isStale = () => appSelectionRequestRef.current !== requestID;
    const quotedAppID = shellQuote(appId);

    setSectionCache((prev) => {
      const next = { ...prev };
      delete next.insights;
      for (const sectionId of appScopedSectionIDs) {
        next[sectionId] = { loading: true, items: [] };
      }
      return next;
    });

    setAppStatus({ loading: true, data: null });
    RunASCCommand(`status --app ${quotedAppID} --output json`)
      .then((res) => {
        if (isStale()) return;
        if (res.error) {
          setAppStatus({ loading: false, error: res.error, data: null });
          return;
        }
        try {
          setAppStatus({ loading: false, data: JSON.parse(res.data) });
        } catch {
          setAppStatus({ loading: false, error: "Failed to parse status", data: null });
        }
      })
      .catch((error) => {
        if (isStale()) return;
        setAppStatus({ loading: false, error: String(error), data: null });
      });

    setReviews({ loading: true, items: [] });
    RunASCCommand(`reviews list --app ${quotedAppID} --limit 25 --output json`)
      .then((res) => {
        if (isStale()) return;
        if (res.error) {
          setReviews({ loading: false, error: res.error, items: [] });
          return;
        }
        try {
          const parsed = JSON.parse(res.data);
          setReviews({
            loading: false,
            items: (parsed.data ?? []).map(
              (item: { attributes: Record<string, unknown> }) => item.attributes,
            ),
          });
        } catch {
          setReviews({ loading: false, error: "Failed to parse", items: [] });
        }
      })
      .catch((error) => {
        if (isStale()) return;
        setReviews({ loading: false, error: String(error), items: [] });
      });

    setPricingOverview({ ...emptyPricingOverview(), loading: true });
    GetPricingOverview(appId)
      .then((res) => {
        if (isStale()) return;
        if (res.error) {
          setPricingOverview({ ...emptyPricingOverview(), loading: false, error: res.error });
          return;
        }
        setPricingOverview({
          loading: false,
          availableInNewTerritories: res.availableInNewTerritories,
          currentPrice: res.currentPrice,
          currentProceeds: res.currentProceeds,
          baseCurrency: res.baseCurrency,
          territories: res.territories ?? [],
          subscriptionPricing: res.subscriptionPricing ?? [],
        });
      })
      .catch((error) => {
        if (isStale()) return;
        setPricingOverview({ ...emptyPricingOverview(), loading: false, error: String(error) });
      });

    setSubscriptions({ loading: true, items: [] });
    GetSubscriptions(appId)
      .then((res) => {
        if (isStale()) return;
        if (res.error) {
          setSubscriptions({ loading: false, error: res.error, items: res.subscriptions ?? [] });
          return;
        }
        setSubscriptions({ loading: false, items: res.subscriptions ?? [] });
      })
      .catch((error) => {
        if (isStale()) return;
        setSubscriptions({ loading: false, error: String(error), items: [] });
      });

    setFinanceRegions({ loading: true, regions: [] });
    GetFinanceRegions()
      .then((res) => {
        if (isStale()) return;
        if (res.error) {
          setFinanceRegions({ loading: false, error: res.error, regions: [] });
          return;
        }
        setFinanceRegions({ loading: false, regions: res.regions ?? [] });
      })
      .catch((error) => {
        if (isStale()) return;
        setFinanceRegions({ loading: false, error: String(error), regions: [] });
      });

    setFeedbackData({ ...emptyFeedback(), loading: true });
    GetFeedback(appId)
      .then((res) => {
        if (isStale()) return;
        if (res.error) {
          setFeedbackData({ ...emptyFeedback(), loading: false, error: res.error });
          return;
        }
        setFeedbackData({ loading: false, total: res.total, items: res.feedback ?? [] });
      })
      .catch((error) => {
        if (isStale()) return;
        setFeedbackData({ ...emptyFeedback(), loading: false, error: String(error) });
      });

    setOfferCodes(emptyOfferCodes());

    const sectionPrefetchTasks = Object.entries(sectionCommands)
      .filter(([sectionId]) => sectionRequiresApp(sectionId))
      .map(([sectionId, cmdTemplate]) => {
        const cmd = commandForApp(cmdTemplate, appId);
        return async () => {
          if (isStale()) return;
          try {
            const res = await RunASCCommand(cmd);
            if (isStale()) return;
            if (res.error) {
              setSectionCache((prev) => ({
                ...prev,
                [sectionId]: { loading: false, error: res.error, items: [] },
              }));
              return;
            }
            try {
              setSectionCache((prev) => ({
                ...prev,
                [sectionId]: { loading: false, items: parseCommandItems(res.data) },
              }));
            } catch {
              setSectionCache((prev) => ({
                ...prev,
                [sectionId]: { loading: false, error: "Failed to parse response", items: [] },
              }));
            }
          } catch (error) {
            if (isStale()) return;
            setSectionCache((prev) => ({
              ...prev,
              [sectionId]: { loading: false, error: String(error), items: [] },
            }));
          }
        };
      });

    void runWithConcurrency(sectionPrefetchTasks, appSectionPrefetchConcurrency);
  }

  function loadStandaloneSection(sectionId: string, force = false) {
    const cmd = sectionCommands[sectionId];
    if (!cmd || sectionRequiresApp(sectionId)) return;

    setSectionCache((prev) => {
      const existing = prev[sectionId];
      if (existing && !force) return prev;
      return { ...prev, [sectionId]: { loading: true, items: [] } };
    });

    RunASCCommand(cmd)
      .then((res) => {
        if (res.error) {
          setSectionCache((prev) => ({
            ...prev,
            [sectionId]: { loading: false, error: res.error, items: [] },
          }));
          return;
        }
        try {
          setSectionCache((prev) => ({
            ...prev,
            [sectionId]: { loading: false, items: parseCommandItems(res.data) },
          }));
        } catch {
          setSectionCache((prev) => ({
            ...prev,
            [sectionId]: { loading: false, error: "Failed to parse response", items: [] },
          }));
        }
      })
      .catch((error) => {
        setSectionCache((prev) => ({
          ...prev,
          [sectionId]: { loading: false, error: String(error), items: [] },
        }));
      });
  }

  function loadStandaloneSectionIfNeeded(sectionId: string) {
    if (sectionCache[sectionId]) return;
    loadStandaloneSection(sectionId);
  }

  function loadOfferCodesIfNeeded(sectionId: string, appId: string | null, force = false) {
    if (sectionId !== "promo-codes" || !appId) return;
    if (!force && (offerCodes.loading || (offerCodes.loadedAppId === appId && !offerCodes.error))) return;

    const appRequestID = appSelectionRequestRef.current;
    const offerRequestID = offerCodesRequestRef.current + 1;
    offerCodesRequestRef.current = offerRequestID;

    setOfferCodes({ loading: true, loadedAppId: appId, codes: [] });

    GetOfferCodes(appId)
      .then((res) => {
        if (
          appSelectionRequestRef.current !== appRequestID ||
          offerCodesRequestRef.current !== offerRequestID
        ) {
          return;
        }
        if (res.error) {
          setOfferCodes({
            loading: false,
            loadedAppId: appId,
            error: res.error,
            codes: res.offerCodes ?? [],
          });
          return;
        }
        setOfferCodes({ loading: false, loadedAppId: appId, codes: res.offerCodes ?? [] });
      })
      .catch((error) => {
        if (
          appSelectionRequestRef.current !== appRequestID ||
          offerCodesRequestRef.current !== offerRequestID
        ) {
          return;
        }
        setOfferCodes({ loading: false, loadedAppId: appId, error: String(error), codes: [] });
      });
  }

  function loadInsightsIfNeeded(sectionId: string, appId: string | null, force = false) {
    if (sectionId !== "insights" || !appId) return;
    const existingInsights = sectionCache.insights;
    if (!force && (existingInsights?.loading || (existingInsights && !existingInsights.error))) return;

    const weekStr = insightsWeekStart(new Date());
    const appRequestID = appSelectionRequestRef.current;
    const insightsRequestID = insightsRequestRef.current + 1;
    insightsRequestRef.current = insightsRequestID;

    setSectionCache((prev) => ({ ...prev, insights: { loading: true, items: [] } }));

    void RunASCCommand(
      `insights weekly --app ${shellQuote(appId)} --source analytics --week ${weekStr} --output json`,
    )
      .then((res) => {
        if (
          appSelectionRequestRef.current !== appRequestID ||
          insightsRequestRef.current !== insightsRequestID
        ) {
          return;
        }
        if (res.error) {
          setSectionCache((current) => ({
            ...current,
            insights: { loading: false, error: res.error, items: [] },
          }));
          return;
        }
        try {
          const parsed = JSON.parse(res.data);
          setSectionCache((current) => ({
            ...current,
            insights: { loading: false, items: (parsed.metrics ?? []).map((metric: Record<string, unknown>) => metric) },
          }));
        } catch {
          setSectionCache((current) => ({
            ...current,
            insights: { loading: false, error: "Failed to parse", items: [] },
          }));
        }
      })
      .catch((error) => {
        if (
          appSelectionRequestRef.current !== appRequestID ||
          insightsRequestRef.current !== insightsRequestID
        ) {
          return;
        }
        setSectionCache((current) => ({
          ...current,
          insights: { loading: false, error: String(error), items: [] },
        }));
      });
  }

  return {
    sectionCache,
    appStatus,
    reviews,
    subscriptions,
    pricingOverview,
    selectedSub,
    financeRegions,
    offerCodes,
    feedbackData,
    setSelectedSub,
    setSectionCache,
    resetSelection,
    prefetchSections,
    loadStandaloneSection,
    loadStandaloneSectionIfNeeded,
    loadOfferCodesIfNeeded,
    loadInsightsIfNeeded,
  };
}
