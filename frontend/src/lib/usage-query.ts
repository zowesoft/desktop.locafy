import { useQuery } from "@tanstack/react-query";

import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";

export const machineStatusKey = ["engine-status"] as const;

/*
 * One shared reading of engine health and machine usage. The engine samples
 * the machine every two seconds, so any two surfaces that fetch at their own
 * moments can show numbers seconds apart; one query makes the top bar and
 * every overview show the same reading.
 */
export function useMachineStatus(refetchInterval = 15_000) {
  return useQuery({
    queryKey: machineStatusKey,
    queryFn: () => StatusService.Health(),
    refetchInterval,
  });
}
