import {
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { Info } from "lucide-react";
import { t } from "./i18n";

export function HelpTip({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  const id = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);

  useLayoutEffect(() => {
    if (!open) return;
    const position = () => {
      if (!trigger.current || !panel.current) return;
      const anchor = trigger.current.getBoundingClientRect();
      const popup = panel.current;
      const gap = 8;
      const margin = 12;
      const left = Math.max(
        margin,
        Math.min(anchor.left, window.innerWidth - popup.offsetWidth - margin),
      );
      const below = anchor.bottom + gap;
      const top =
        below + popup.offsetHeight <= window.innerHeight - margin
          ? below
          : Math.max(margin, anchor.top - popup.offsetHeight - gap);
      popup.style.left = `${left}px`;
      popup.style.top = `${top}px`;
    };
    position();
    window.addEventListener("resize", position);
    document.addEventListener("scroll", position, true);
    return () => {
      window.removeEventListener("resize", position);
      document.removeEventListener("scroll", position, true);
    };
  }, [open]);

  return (
    <>
      <button
        ref={trigger}
        type="button"
        className="help-tip-trigger"
        aria-label={t("About {subject}", { subject: title })}
        aria-expanded={open}
        aria-controls={id}
        aria-describedby={open ? id : undefined}
        popoverTarget={id}
        onKeyDown={(event) => {
          if (event.key === "Escape" && open) {
            event.stopPropagation();
            panel.current?.hidePopover();
          }
        }}
      >
        <Info size={16} aria-hidden="true" />
      </button>
      {createPortal(
        <div
          ref={panel}
          id={id}
          className="help-tip-panel"
          popover="auto"
          role="note"
          aria-labelledby={`${id}-title`}
          onToggle={(event) => setOpen(event.newState === "open")}
        >
          <strong id={`${id}-title`}>{title}</strong>
          {children}
        </div>,
        document.body,
      )}
    </>
  );
}
