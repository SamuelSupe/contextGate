import { Component, Suspense, type ReactNode } from "react";
import { Button, ErrorNote, Loading } from "./components";
import { canNavigate } from "./useNavigationGuard";
import { t } from "./i18n";

export class PageBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    if (this.state.failed)
      return (
        <div>
          <ErrorNote
            error={t(
              "This page could not be loaded. Check your connection and reload when your changes are saved.",
            )}
          />
          <Button
            onClick={() => {
              if (canNavigate()) location.reload();
            }}
          >
            {t("Reload page")}
          </Button>
        </div>
      );
    return <Suspense fallback={<Loading />}>{this.props.children}</Suspense>;
  }
}
