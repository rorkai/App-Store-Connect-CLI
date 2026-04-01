import { startTransition, useRef, useState } from "react";

import { useAppMetadata } from "./appSelection/useAppMetadata";
import { useAppSectionData } from "./appSelection/useAppSectionData";
import { useTestFlightData } from "./appSelection/useTestFlightData";

export function useAppSelection() {
  const [selectedAppId, setSelectedAppId] = useState<string | null>(null);

  const appSelectionRequestRef = useRef(0);
  const metadata = useAppMetadata(appSelectionRequestRef);
  const sectionData = useAppSectionData(appSelectionRequestRef);
  const testFlight = useTestFlightData(appSelectionRequestRef);

  function handleSelectApp(id: string, activeSectionId?: string) {
    const requestID = appSelectionRequestRef.current + 1;
    appSelectionRequestRef.current = requestID;

    startTransition(() => {
      setSelectedAppId(id);
      metadata.resetSelection();
      sectionData.resetSelection();
      testFlight.resetSelection();
    });

    sectionData.prefetchSections(id, requestID);
    testFlight.loadGroups(id, requestID);
    metadata.loadAppDetail(id, requestID);
    if (activeSectionId) {
      sectionData.loadOfferCodesIfNeeded(activeSectionId, id, true);
      sectionData.loadInsightsIfNeeded(activeSectionId, id, true);
    }
  }

  return {
    selectedAppId,
    appDetail: metadata.appDetail,
    allLocalizations: metadata.allLocalizations,
    selectedLocale: metadata.selectedLocale,
    metadataLoading: metadata.metadataLoading,
    metadataError: metadata.metadataError,
    screenshotSets: metadata.screenshotSets,
    screenshotsLoading: metadata.screenshotsLoading,
    screenshotsError: metadata.screenshotsError,
    sectionCache: sectionData.sectionCache,
    appStatus: sectionData.appStatus,
    reviews: sectionData.reviews,
    subscriptions: sectionData.subscriptions,
    pricingOverview: sectionData.pricingOverview,
    selectedSub: sectionData.selectedSub,
    financeRegions: sectionData.financeRegions,
    offerCodes: sectionData.offerCodes,
    feedbackData: sectionData.feedbackData,
    testflightData: testFlight.testflightData,
    selectedGroup: testFlight.selectedGroup,
    groupTesters: testFlight.groupTesters,
    setSelectedSub: sectionData.setSelectedSub,
    handleSelectApp,
    handleLocaleChange: metadata.handleLocaleChange,
    handleSelectGroup: testFlight.handleSelectGroup,
    handleBackToGroups: testFlight.handleBackToGroups,
    loadStandaloneSection: sectionData.loadStandaloneSection,
    loadStandaloneSectionIfNeeded: sectionData.loadStandaloneSectionIfNeeded,
    loadOfferCodesIfNeeded: sectionData.loadOfferCodesIfNeeded,
    loadInsightsIfNeeded: sectionData.loadInsightsIfNeeded,
  };
}
