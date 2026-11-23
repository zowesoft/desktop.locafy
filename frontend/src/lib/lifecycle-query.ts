import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";

export const lifecyclesKey = ["applications", "lifecycles"] as const;

export const lifecycleKey = (projectID: string) => ["applications", "lifecycle", projectID] as const;

export const projectUsageKey = (projectID: string) => ["applications", "usage", projectID] as const;

/*
 * Process state changes arrive as application:state and service:state events;
 * a project's derived lifecycle covers both, so any surface that displays it
 * reacts to either instead of polling.
 */
function useApplicationStateInvalidation(keys: ReadonlyArray<ReadonlyArray<unknown>>) {
  const queryClient = useQueryClient();

  useEffect(() => {
    const invalidate = () => {
      for (const key of keys) {
        void queryClient.invalidateQueries({ queryKey: key });
      }
    };
    const offApplication = Events.On("application:state", (event) => {
      const data = event.data as { applicationId?: string };
      if (!data.applicationId) return;
      invalidate();
    });
    const offService = Events.On("service:state", (event) => {
      const data = event.data as { ID?: string };
      if (!data.ID) return;
      invalidate();
    });
    return () => {
      offApplication();
      offService();
    };
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

/*
 * The project's managed usage: what the applications Locafy runs for this
 * project consume right now. Per-application readings arrive as
 * application:usage events; the aggregate is refetched when they change.
 */
export function useProjectUsage(projectID: string) {
  const queryClient = useQueryClient();

  useEffect(() => {
    const off = Events.On("application:usage", (event) => {
      const data = event.data as { applicationId?: string };
      if (!data.applicationId) return;
      void queryClient.invalidateQueries({ queryKey: projectUsageKey(projectID) });
    });
    return off;
  }, [queryClient, projectID]);

  return useQuery({
    queryKey: projectUsageKey(projectID),
    queryFn: async () =>
      (await ApplicationsService.ProjectUsage(projectID)) ?? {
        cpuPercent: 0,
        memoryBytes: 0,
        runningApplications: 0,
      },
  });
}
