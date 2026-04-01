import { FormEvent } from "react";
import { ChatMessage } from "../types";

type ChatDockProps = {
  messages: ChatMessage[];
  draft: string;
  dockExpanded: boolean;
  handleSubmit: (event: FormEvent<HTMLFormElement>) => void;
  setDraft: (draft: string) => void;
  setDockExpanded: (expanded: boolean) => void;
};

export function ChatDock({
  messages,
  draft,
  dockExpanded,
  handleSubmit,
  setDraft,
  setDockExpanded,
}: ChatDockProps) {
  return (
    <>
      <section className={`dock ${dockExpanded ? "dock-expanded" : ""}`}>
        {dockExpanded && (
          <div className="dock-header">
            <span className="dock-title">ACP Chat</span>
            <button
              className="dock-collapse"
              type="button"
              onClick={() => setDockExpanded(false)}
              aria-label="Collapse chat"
            >
              &#x25BE;
            </button>
          </div>
        )}

        <div className="dock-body">
          {messages.length > 0 && (
            <div className="message-list" role="log" aria-label="Chat messages" aria-live="polite">
              {messages.map((message) => (
                <article key={message.id} className={`message-row role-${message.role}`}>
                  <p>{message.content}</p>
                </article>
              ))}
            </div>
          )}
        </div>

        <form className="composer" onSubmit={handleSubmit}>
          <div
            className="composer-card"
            role={!dockExpanded ? "button" : undefined}
            tabIndex={!dockExpanded ? 0 : undefined}
            aria-label={!dockExpanded ? "Expand chat" : undefined}
            onClick={() => !dockExpanded && setDockExpanded(true)}
            onKeyDown={(e) => { if (!dockExpanded && (e.key === "Enter" || e.key === " ")) { e.preventDefault(); setDockExpanded(true); } }}
          >
            <textarea
              aria-label="Chat prompt"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Ask Studio to inspect builds, explain blockers, or draft a command…"
              rows={2}
            />
            <div className="composer-bar">
              <div />
              <button className="send-btn" type="submit" aria-label="Send">&#x2B06;</button>
            </div>
          </div>
        </form>
      </section>
      <div className="provider-buttons">
        <button type="button" className="provider-btn">Codex</button>
        <button type="button" className="provider-btn">Cursor</button>
        <button type="button" className="provider-btn">Custom ACP</button>
      </div>
    </>
  );
}
