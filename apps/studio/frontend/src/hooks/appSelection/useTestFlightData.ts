import { useRef, useState, type MutableRefObject } from "react";

import { GroupTestersState, TestFlightState } from "../../types";
import { GetTestFlight, GetTestFlightTesters } from "../../../wailsjs/go/main/App";

export function useTestFlightData(appSelectionRequestRef: MutableRefObject<number>) {
  const [testflightData, setTestflightData] = useState<TestFlightState>({ loading: false, groups: [] });
  const [selectedGroup, setSelectedGroup] = useState<string | null>(null);
  const [groupTesters, setGroupTesters] = useState<GroupTestersState>({ loading: false, testers: [] });

  const groupTesterRequestRef = useRef(0);

  function resetSelection() {
    groupTesterRequestRef.current += 1;
    setSelectedGroup(null);
    setGroupTesters({ loading: false, testers: [] });
  }

  function loadGroups(appId: string, requestID: number) {
    setTestflightData({ loading: true, groups: [] });
    resetSelection();

    GetTestFlight(appId)
      .then((res) => {
        if (appSelectionRequestRef.current !== requestID) return;
        if (res.error) {
          setTestflightData({ loading: false, error: res.error, groups: [] });
          return;
        }
        setTestflightData({ loading: false, groups: res.groups ?? [] });
      })
      .catch((error) => {
        if (appSelectionRequestRef.current !== requestID) return;
        setTestflightData({ loading: false, error: String(error), groups: [] });
      });
  }

  function handleSelectGroup(groupId: string) {
    const testerRequestID = groupTesterRequestRef.current + 1;
    groupTesterRequestRef.current = testerRequestID;

    setSelectedGroup(groupId);
    setGroupTesters({ loading: true, testers: [] });

    GetTestFlightTesters(groupId)
      .then((res) => {
        if (groupTesterRequestRef.current !== testerRequestID) return;
        if (res.error) {
          setGroupTesters({ loading: false, error: res.error, testers: [] });
          return;
        }
        setGroupTesters({ loading: false, testers: res.testers ?? [] });
      })
      .catch((error) => {
        if (groupTesterRequestRef.current !== testerRequestID) return;
        setGroupTesters({ loading: false, error: String(error), testers: [] });
      });
  }

  function handleBackToGroups() {
    setSelectedGroup(null);
  }

  return {
    testflightData,
    selectedGroup,
    groupTesters,
    resetSelection,
    loadGroups,
    handleSelectGroup,
    handleBackToGroups,
  };
}
