import { FormEvent, useState } from "react";
import { ChatMessage } from "../types";

export function useChatDock() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [dockExpanded, setDockExpanded] = useState(false);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = draft.trim();
    if (!trimmed) return;

    setMessages((current) => [
      ...current,
      { id: `user-${current.length}`, role: "user", content: trimmed, timestamp: "Now" },
      {
        id: `assistant-${current.length}`,
        role: "assistant",
        content: "Bootstrap mode recorded the prompt. Live ACP transport is not wired yet.",
        timestamp: "Now",
      },
    ]);
    setDraft("");
    setDockExpanded(true);
  }

  return { messages, draft, dockExpanded, setDraft, setDockExpanded, handleSubmit };
}
