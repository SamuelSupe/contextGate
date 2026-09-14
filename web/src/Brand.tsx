import { t } from "./i18n";

export function Brand({ tagline = false }: { tagline?: boolean }) {
  return (
    <span className="brand-lockup">
      <img
        className="brand-mark"
        src="/contextgate-mark.svg"
        alt=""
        width="36"
        height="36"
      />
      <span className="brand-copy">
        <span className="brand-name">ContextGate</span>
        {tagline && (
          <span className="brand-tagline">
            {t("Semantic Data Gateway for AI Agents")}
          </span>
        )}
      </span>
    </span>
  );
}
