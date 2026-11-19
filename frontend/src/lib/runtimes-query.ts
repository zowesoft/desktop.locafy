import { useQuery } from "@tanstack/react-query";

import type { CatalogEntry } from "../../bindings/locafy.dev/desktop/internal/runtimes/models";
import * as RuntimesService from "../../bindings/locafy.dev/desktop/internal/runtimes/service";

export const runtimesKey = ["runtimes"] as const;

/*
 * The machine's runtime registry: every kind and the copies found for it.
 * Discovery reads the machine and stores nothing, so one shared query keeps
 * the Runtimes screen and every application's runtime picker on the same
 * answer.
 */
export function useRuntimeCatalog() {
  return useQuery<CatalogEntry[]>({
    queryKey: runtimesKey,
    queryFn: async () => {
      const entries = await RuntimesService.Catalog();
      return entries ?? [];
    },
    staleTime: 60_000,
  });
}
