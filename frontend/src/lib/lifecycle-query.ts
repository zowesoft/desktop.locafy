import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";

export const lifecyclesKey = ["applications", "lifecycles"] as const;

export const lifecycleKey = (projectID: string) => ["applications", "lifecycle", projectID] as const;

/*
 * Process state changes arrive as application:state events; any surface that
 * displays a derived lifecycle reacts to them instead of polling.
 */
function useApplicationStateInvalidation(keys: ReadonlyArray<ReadonlyArray<unknown>>) {
  const queryClient = useQueryClient();

  useEffect(() => {
    const off = Events.On("application:state", (event) => {
      const data = event.data as { applicationId?: string };
      if (!data.applicationId) return;
      for (const key of keys) {
        void queryClient.invalidateQueries({ queryKey: key });
      }
    });
    return off;
    // The keys are static for a mounted surface.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queryClient]);
}

/** One project's derived lifecycle state. */
export function useProjectLifecycle(projectID: string) {
  useApplicationStateInvalidation([lifecycleKey(projectID)]);
  return useQuery({
    queryKey: lifecycleKey(projectID),
    queryFn: async () => (await ApplicationsService.Lifecycle(projectID)) ?? "stopped",
    enabled: projectID !== "",
  });
}

/** Derived lifecycle states for every project that owns applications. */
export function useProjectLifecycles() {
  useApplicationStateInvalidation([lifecyclesKey]);
  return useQuery({
    queryKey: lifecyclesKey,
    queryFn: async () => (await ApplicationsService.Lifecycles()) ?? {},
  });
}
