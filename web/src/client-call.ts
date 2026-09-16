import type { Readiness } from "./readiness";

export async function waitForClientCall(
  load: (signal: AbortSignal) => Promise<Readiness>,
  identity: {
    source: string;
    template: string;
    version: string;
    agent: string;
  },
  signal: AbortSignal,
  timing = { interval: 3000, timeout: 120000 },
): Promise<"confirmed" | "changed" | "timeout"> {
  const deadline = new AbortController();
  const timer = setTimeout(() => deadline.abort(), timing.timeout);
  const combined = AbortSignal.any([signal, deadline.signal]);
  try {
    for (;;) {
      combined.throwIfAborted();
      const ready = await load(combined);
      combined.throwIfAborted();
      const template = ready.templates.find((t) => t.id === identity.template);
      if (
        ready.source_id !== identity.source ||
        !ready.active_agents.includes(identity.agent) ||
        !template?.executable ||
        template.execution_version !== identity.version
      )
        return "changed";
      const activity = ready.template_activity;
      if (
        activity?.agent_id === identity.agent &&
        activity.template_id === identity.template &&
        activity.execution_version === identity.version &&
        activity.successful_calls > 0
      )
        return "confirmed";
      await new Promise<void>((resolve, reject) => {
        const abort = () => {
          clearTimeout(pause);
          reject(combined.reason);
        };
        const pause = setTimeout(() => {
          combined.removeEventListener("abort", abort);
          resolve();
        }, timing.interval);
        combined.addEventListener("abort", abort, { once: true });
      });
    }
  } catch (error) {
    if (deadline.signal.aborted && !signal.aborted) return "timeout";
    throw error;
  } finally {
    clearTimeout(timer);
  }
}
