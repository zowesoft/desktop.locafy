import { useQuery } from "@tanstack/react-query";

import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";

export const projectsKey = ["projects"] as const;

/** Live projects, shared by the dashboard and every project workspace. */
export function useProjects() {
  return useQuery({
    queryKey: projectsKey,
    queryFn: async () => (await ProjectsService.List()) ?? [],
  });
}
