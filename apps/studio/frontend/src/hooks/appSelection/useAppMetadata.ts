import { useRef, useState, type MutableRefObject } from "react";

import { AppDetail, LocalizationEntry, ScreenshotSet } from "../../types";
import { GetAppDetail, GetScreenshots, GetVersionMetadata } from "../../../wailsjs/go/main/App";

export function useAppMetadata(appSelectionRequestRef: MutableRefObject<number>) {
  const [appDetail, setAppDetail] = useState<AppDetail | null>(null);
  const [allLocalizations, setAllLocalizations] = useState<LocalizationEntry[]>([]);
  const [selectedLocale, setSelectedLocale] = useState("");
  const [metadataLoading, setMetadataLoading] = useState(false);
  const [metadataError, setMetadataError] = useState("");
  const [screenshotSets, setScreenshotSets] = useState<ScreenshotSet[]>([]);
  const [screenshotsLoading, setScreenshotsLoading] = useState(false);
  const [screenshotsError, setScreenshotsError] = useState("");

  const screenshotRequestRef = useRef(0);

  function resetSelection() {
    screenshotRequestRef.current += 1;
    setAppDetail(null);
    setAllLocalizations([]);
    setSelectedLocale("");
    setMetadataError("");
    setScreenshotSets([]);
    setMetadataLoading(false);
    setScreenshotsLoading(false);
    setScreenshotsError("");
  }

  function loadScreenshots(localizationId: string, requestID: number, clearCurrent = false) {
    const screenshotRequestID = screenshotRequestRef.current + 1;
    screenshotRequestRef.current = screenshotRequestID;

    setScreenshotsLoading(true);
    setScreenshotsError("");
    if (clearCurrent) {
      setScreenshotSets([]);
    }

    GetScreenshots(localizationId)
      .then((res) => {
        if (
          appSelectionRequestRef.current !== requestID ||
          screenshotRequestRef.current !== screenshotRequestID
        ) {
          return;
        }
        if (res.error) {
          setScreenshotSets([]);
          setScreenshotsError(res.error);
          return;
        }
        setScreenshotSets(res.sets ?? []);
      })
      .catch((error) => {
        if (
          appSelectionRequestRef.current !== requestID ||
          screenshotRequestRef.current !== screenshotRequestID
        ) {
          return;
        }
        setScreenshotSets([]);
        setScreenshotsError(String(error));
      })
      .finally(() => {
        if (
          appSelectionRequestRef.current !== requestID ||
          screenshotRequestRef.current !== screenshotRequestID
        ) {
          return;
        }
        setScreenshotsLoading(false);
      });
  }

  function loadAppDetail(appId: string, requestID: number) {
    GetAppDetail(appId)
      .then((detail) => {
        if (appSelectionRequestRef.current !== requestID) return;

        setAppDetail({
          id: detail.id,
          name: detail.name,
          subtitle: detail.subtitle,
          bundleId: detail.bundleId,
          sku: detail.sku,
          primaryLocale: detail.primaryLocale,
          versions: detail.versions ?? [],
          error: detail.error,
        });

        const primaryVersion = (detail.versions ?? []).find(
          (version: { platform: string }) => version.platform === "IOS",
        ) ?? (detail.versions ?? [])[0];

        if (!primaryVersion?.id) return;

        setMetadataLoading(true);
        setMetadataError("");
        GetVersionMetadata(primaryVersion.id)
          .then((metadata) => {
            if (appSelectionRequestRef.current !== requestID) return;
            if (metadata.error) {
              setAllLocalizations([]);
              setSelectedLocale("");
              setScreenshotSets([]);
              setScreenshotsError("");
              setMetadataError(metadata.error);
              return;
            }
            if (!metadata.localizations?.length) {
              setAllLocalizations([]);
              setSelectedLocale("");
              setScreenshotSets([]);
              setScreenshotsError("");
              return;
            }

            setAllLocalizations(metadata.localizations);
            const defaultLocalization = metadata.localizations.find(
              (localization: { locale: string }) => localization.locale === detail.primaryLocale,
            ) ?? metadata.localizations[0];
            setSelectedLocale(defaultLocalization.locale);

            if (defaultLocalization.localizationId) {
              loadScreenshots(defaultLocalization.localizationId, requestID);
            }
          })
          .catch((error) => {
            if (appSelectionRequestRef.current !== requestID) return;
            setAllLocalizations([]);
            setSelectedLocale("");
            setScreenshotSets([]);
            setScreenshotsError("");
            setMetadataError(String(error));
          })
          .finally(() => {
            if (appSelectionRequestRef.current !== requestID) return;
            setMetadataLoading(false);
          });
      })
      .catch((error) => {
        if (appSelectionRequestRef.current !== requestID) return;
        setAppDetail({
          id: appId,
          name: "",
          subtitle: "",
          bundleId: "",
          sku: "",
          primaryLocale: "",
          versions: [],
          error: String(error),
        });
      });
  }

  function handleLocaleChange(locale: string) {
    setSelectedLocale(locale);

    const localization = allLocalizations.find((entry) => entry.locale === locale);
    if (!localization?.localizationId) return;

    loadScreenshots(localization.localizationId, appSelectionRequestRef.current, true);
  }

  return {
    appDetail,
    allLocalizations,
    selectedLocale,
    metadataLoading,
    metadataError,
    screenshotSets,
    screenshotsLoading,
    screenshotsError,
    resetSelection,
    loadAppDetail,
    handleLocaleChange,
  };
}
