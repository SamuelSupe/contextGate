import { useEffect, useRef } from "react";

const guards = new Set<() => boolean>();

export function canNavigate() {
  for (const guard of guards) if (!guard()) return false;
  return true;
}

// Browser beforeunload does not run for the app's pushState/popstate navigation.
export function useNavigationGuard(dirty: boolean, busy = false) {
  const current = useRef({ dirty, busy });
  current.current = { dirty, busy };
  useEffect(() => {
    const guard = () => !current.current.busy && !current.current.dirty;
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (current.current.dirty || current.current.busy) {
        event.preventDefault();
        event.returnValue = "";
      }
    };
    guards.add(guard);
    window.addEventListener("beforeunload", beforeUnload);
    return () => {
      guards.delete(guard);
      window.removeEventListener("beforeunload", beforeUnload);
    };
  }, []);
  return () => {
    current.current = { dirty: false, busy: false };
  };
}
