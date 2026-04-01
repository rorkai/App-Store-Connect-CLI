import { useCallback, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { allSections, scopes } from "../constants";
import { AuthState, NavSection } from "../types";

type ContextBarProps = {
  authStatus: AuthState;
  activeScope: string;
  handleRefresh: () => void;
  setActiveScope: (scope: string) => void;
  setActiveSection: (section: NavSection) => void;
};

export function ContextBar({
  authStatus,
  activeScope,
  handleRefresh,
  setActiveScope,
  setActiveSection,
}: ContextBarProps) {
  const authConfigured = authStatus.authenticated;
  const tooltipId = useId();
  const dotRef = useRef<HTMLSpanElement>(null);
  const [tooltipPos, setTooltipPos] = useState<{ top: number; left: number } | null>(null);

  const showTooltip = useCallback(() => {
    const el = dotRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    setTooltipPos({
      top: rect.bottom + 6,
      left: rect.left + rect.width / 2,
    });
  }, []);

  const hideTooltip = useCallback(() => setTooltipPos(null), []);

  const tooltipText = [authStatus.storage, authStatus.profile].filter(Boolean).join(" · ");

  return (
    <header className="context-bar">
      <div className="context-app">
        {authConfigured ? (
          <>
            <span
              ref={dotRef}
              className="context-dot-wrap"
              tabIndex={0}
              role="img"
              aria-label={`Connected via ${[authStatus.storage, authStatus.profile].filter(Boolean).join(", ")}`}
              aria-describedby={tooltipPos ? tooltipId : undefined}
              onMouseEnter={showTooltip}
              onMouseLeave={hideTooltip}
              onFocus={showTooltip}
              onBlur={hideTooltip}
            >
              <span className="context-dot state-ready" />
            </span>
            {tooltipPos && createPortal(
              <span
                id={tooltipId}
                className="context-dot-tooltip"
                role="tooltip"
                style={{ top: tooltipPos.top, left: tooltipPos.left }}
              >
                {tooltipText}
              </span>,
              document.body,
            )}
            <span className="context-title">ASC Studio</span>
          </>
        ) : (
          <>
            <span className="context-status state-processing">Not authenticated</span>
            <span className="context-title">ASC Studio</span>
          </>
        )}
      </div>
      <div className="toolbar-right">
        <div className="scope-tabs" role="tablist" aria-label="Scope">
          {scopes.map((scope) => (
            <button
              key={scope.id}
              type="button"
              role="tab"
              aria-selected={activeScope === scope.id}
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
          aria-label="Refresh (⌘R)"
          title="Refresh (⌘R)"
        >
          <span aria-hidden="true">↻</span>
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
  );
}
