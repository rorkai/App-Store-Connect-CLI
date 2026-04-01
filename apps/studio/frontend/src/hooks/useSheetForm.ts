import { useReducer } from "react";
import { shellQuote } from "../utils";
import { RunASCCommand } from "../../wailsjs/go/main/App";

type SheetState = {
  open: boolean;
  name: string;
  identifier: string;
  platform: string;
  error: string;
  creating: boolean;
};

type SheetAction =
  | { type: "open" }
  | { type: "close" }
  | { type: "setName"; value: string }
  | { type: "setIdentifier"; value: string }
  | { type: "setPlatform"; value: string }
  | { type: "setError"; value: string }
  | { type: "setCreating"; value: boolean };

const initialState: SheetState = {
  open: false, name: "", identifier: "", platform: "IOS", error: "", creating: false,
};

function sheetReducer(state: SheetState, action: SheetAction): SheetState {
  switch (action.type) {
    case "open": return { ...initialState, open: true };
    case "close": return { ...state, open: false, error: "", creating: false };
    case "setName": return { ...state, name: action.value };
    case "setIdentifier": return { ...state, identifier: action.value };
    case "setPlatform": return { ...state, platform: action.value };
    case "setError": return { ...state, error: action.value };
    case "setCreating": return { ...state, creating: action.value };
  }
}

export function useBundleIDSheet(onCreated: () => void) {
  const [state, dispatch] = useReducer(sheetReducer, initialState);
  const quotedPlatform = shellQuote(state.platform);

  const commandPreview =
    `bundle-ids create --identifier ${shellQuote(state.identifier.trim())} --name ${shellQuote(state.name.trim())} --platform ${quotedPlatform} --output json`;

  function handleCreate() {
    const trimmedName = state.name.trim();
    const trimmedIdentifier = state.identifier.trim();
    if (!trimmedName || !trimmedIdentifier) {
      dispatch({ type: "setError", value: "Name and identifier are required." });
      return;
    }
    dispatch({ type: "setCreating", value: true });
    dispatch({ type: "setError", value: "" });

    RunASCCommand(
      `bundle-ids create --identifier ${shellQuote(trimmedIdentifier)} --name ${shellQuote(trimmedName)} --platform ${quotedPlatform} --output json`,
    )
      .then((res) => {
        if (res.error) { dispatch({ type: "setError", value: res.error }); return; }
        dispatch({ type: "close" });
        onCreated();
      })
      .catch((err) => { dispatch({ type: "setError", value: String(err) }); })
      .finally(() => { dispatch({ type: "setCreating", value: false }); });
  }

  return { state, dispatch, commandPreview, handleCreate };
}

export function useDeviceSheet(onCreated: () => void) {
  const [state, dispatch] = useReducer(sheetReducer, initialState);
  const quotedPlatform = shellQuote(state.platform);

  const commandPreview =
    `devices register --name ${shellQuote(state.name.trim())} --udid ${shellQuote(state.identifier.trim())} --platform ${quotedPlatform} --output json`;

  function handleCreate() {
    const trimmedName = state.name.trim();
    const trimmedUDID = state.identifier.trim();
    if (!trimmedName || !trimmedUDID) {
      dispatch({ type: "setError", value: "Name and UDID are required." });
      return;
    }
    dispatch({ type: "setCreating", value: true });
    dispatch({ type: "setError", value: "" });

    RunASCCommand(
      `devices register --name ${shellQuote(trimmedName)} --udid ${shellQuote(trimmedUDID)} --platform ${quotedPlatform} --output json`,
    )
      .then((res) => {
        if (res.error) { dispatch({ type: "setError", value: res.error }); return; }
        dispatch({ type: "close" });
        onCreated();
      })
      .catch((err) => { dispatch({ type: "setError", value: String(err) }); })
      .finally(() => { dispatch({ type: "setCreating", value: false }); });
  }

  return { state, dispatch, commandPreview, handleCreate };
}
