import { useCallback } from "react";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

export function useFocusTrap(open: boolean, onClose: () => void) {
  const trapRef = useCallback((node: HTMLElement | null) => {
    if (!node) return;
    const focusables = node.querySelectorAll<HTMLElement>(FOCUSABLE);
    if (focusables.length > 0) focusables[0].focus();
  }, []);

  const onTrapKeyDown = useCallback((event: ReactKeyboardEvent<HTMLElement>) => {
    if (!open) return;

    if (event.key === "Escape") {
      onClose();
      return;
    }

    if (event.key !== "Tab") return;

    const focusables = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(FOCUSABLE));
    if (focusables.length === 0) return;

    const first = focusables[0];
    const last = focusables[focusables.length - 1];

    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }, [open, onClose]);

  return { trapRef, onTrapKeyDown };
}
