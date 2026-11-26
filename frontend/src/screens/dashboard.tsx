import { useQueries, useQuery } from "@tanstack/react-query";
import {
  ArrowRight,
  ArrowUpRight,
  Box,
  Boxes,
  Clock,
  FileText,
  FolderKanban,
  FolderOpen,
  Plus,
  Puzzle,
  Server,
  Terminal,
  Zap,
} from "lucide-react";
import { Link } from "react-router-dom";

import * as ApplicationsService from "../../bindings/locafy.dev/desktop/internal/applications/service";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import { PlannedTag } from "@/components/app-shell";
import { LifecycleBadge } from "@/components/lifecycle-badge";
import { notificationsKey } from "@/components/top-bar-status";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useProjectLifecycles } from "@/lib/lifecycle-query";
import { useProjects } from "@/lib/projects-query";
import { useMachineStatus } from "@/lib/usage-query";
import { cn } from "@/lib/utils";

const quietActions = [
  { label: "Start Tunnel", icon: Zap, planned: true },
  { label: "Manage Extensions", icon: Puzzle, planned: true },
];

function uptime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
}

function startedAt(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function ago(iso: string): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const seconds = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

export function Dashboard() {
  const projects = useProjects();
  const lifecycles = useProjectLifecycles();
  const health = useMachineStatus();
  const notifications = useQuery({
    queryKey: [...notificationsKey, "dashboard"],
    queryFn: async () => (await StatusService.Notifications(6)) ?? [],
    refetchInterval: 30_000,
  });

  const projectList = projects.data ?? [];
  const applicationQueries = useQueries({
    queries: projectList.map((project) => ({
      queryKey: ["applications", project.ID],
      queryFn: async () => (await ApplicationsService.List(project.ID)) ?? [],
    })),
  });
  const applicationsByProject = new Map(
    projectList.map((project, index) => [project.ID, applicationQueries[index]?.data ?? []]),
  );
  const applications = projectList.flatMap((project) => applicationsByProject.get(project.ID) ?? []);
  const runningApplications = applications.filter((app) => app.Status === "running").length;

  const stateOf = (project: (typeof projectList)[number]) =>
    lifecycles.data?.[project.ID] ?? project.LifecycleState;
  const runningProjects = projectList.filter((project) =>
    ["running", "partial"].includes(stateOf(project)),
  ).length;

  const engine = health.data;
  const activity = notifications.data ?? [];

  return (
    <div className="space-y-6 px-6 py-6">
      <section className="relative overflow-hidden rounded-2xl border border-border bg-card px-8 py-10">
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-primary/25 via-transparent to-accent/25" />
        <div className="pointer-events-none absolute -right-16 -top-24 size-72 rounded-full bg-primary/25 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-24 right-40 size-56 rounded-full bg-accent/20 blur-3xl" />
        <div className="relative flex flex-wrap items-center justify-between gap-8">
          <div className="max-w-2xl space-y-3">
            <h1 className="text-[clamp(1.5rem,3vw,2.25rem)] font-semibold leading-[1.1] tracking-tight">
              Your Local VPS.
              <br />
              Your Development Environment.
            </h1>
            <p className="text-sm text-muted-foreground">
              Develop, test, and share your applications with ease using Locafy.
            </p>
          </div>
          <div className="hidden shrink-0 rotate-[-6deg] rounded-xl border border-border bg-background/90 p-4 shadow-2xl min-[560px]:block">
            <div className="mb-3 flex gap-1.5">
              <span className="size-2.5 rounded-full bg-destructive" />
              <span className="size-2.5 rounded-full bg-accent" />
              <span className="size-2.5 rounded-full bg-success" />
            </div>
            <div className="flex items-center gap-2 font-mono text-sm text-muted-foreground">
              <span className="text-success">{'>'}</span>
              <span className="tracking-[0.2em]">locafy up</span>
            </div>
          </div>
        </div>
      </section>

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary">
              <FolderKanban className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">Projects</p>
              <p className="text-xl font-semibold tracking-tight">
                {projects.isPending ? "—" : projectList.length}
              </p>
              <p
                className={cn(
                  "text-xs",
                  runningProjects > 0 ? "text-success" : "text-muted-foreground",
                )}
              >
                {projects.isPending ? "Reading projects" : `${runningProjects} running`}
              </p>
            </div>
          </CardContent>
        </Card>
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-success/15 text-success">
              <Boxes className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">Applications</p>
              <p className="text-xl font-semibold tracking-tight">
                {projects.isPending ? "—" : applications.length}
              </p>
              <p
                className={cn(
                  "text-xs",
                  runningApplications > 0 ? "text-success" : "text-muted-foreground",
                )}
              >
                {projects.isPending ? "Reading applications" : `${runningApplications} running`}
              </p>
            </div>
          </CardContent>
        </Card>
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground">
              <Zap className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">Tunnels</p>
              <p className="text-xl font-semibold tracking-tight">—</p>
              <p className="text-xs text-muted-foreground">Planned</p>
            </div>
          </CardContent>
        </Card>
        <Card className="py-0">
          <CardContent className="flex items-center gap-3 p-4">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-accent/15 text-accent">
              <Clock className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted-foreground">Uptime</p>
              <p className="text-xl font-semibold tracking-tight">
                {engine ? uptime(engine.UptimeSeconds) : "—"}
              </p>
              <p className="text-xs text-muted-foreground">
                {engine ? `Engine started ${startedAt(engine.StartedAt)}` : "Checking the engine"}
              </p>
            </div>
          </CardContent>
        </Card>
      </section>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_18rem]">
        <div className="space-y-4">
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <h2 className="text-2xl font-semibold tracking-tight">Your Projects</h2>
              <p className="text-sm text-muted-foreground">
                Manage your development projects and their applications.
              </p>
            </div>
            <Link to="/projects" className={cn(buttonVariants({ size: "lg" }))}>
              <Plus />
              New Project
            </Link>
          </div>

          {projects.error ? (
            <p role="alert" className="text-sm text-destructive">
              Could not read projects: {String(projects.error)}
            </p>
          ) : projects.isPending ? (
            <p className="text-sm text-muted-foreground">Reading projects…</p>
          ) : projectList.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border px-4 py-10 text-center">
              <p className="text-sm font-medium">No projects yet</p>
              <p className="mx-auto mt-1 max-w-sm text-xs text-muted-foreground">
                Create a project in the workspace or link an existing folder to see it here.
              </p>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2">
              {projectList.map((project) => {
                const apps = applicationsByProject.get(project.ID) ?? [];
                const running = apps.filter((app) => app.Status === "running").length;
                const shown = apps.slice(0, 3);
                return (
                  <Card key={project.ID} className="py-0">
                    <CardContent className="space-y-3 p-4">
                      <div className="flex items-start gap-3">
                        <span
                          className={cn(
                            "flex size-9 shrink-0 items-center justify-center rounded-lg",
                            project.Kind === "managed"
                              ? "bg-primary/15 text-primary"
                              : "bg-accent/15 text-accent",
                          )}
                        >
                          {project.Kind === "managed" ? (
                            <Box className="size-4.5" />
                          ) : (
                            <Terminal className="size-4.5" />
                          )}
                        </span>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center justify-between gap-2">
                            <p className="truncate font-medium">{project.Name}</p>
                            <LifecycleBadge state={stateOf(project)} />
                          </div>
                          <p className="truncate text-xs text-muted-foreground">
                            {project.Description ||
                              (project.Kind === "managed"
                                ? "A project Locafy created in its workspace."
                                : "An existing folder Locafy points at.")}
                          </p>
                        </div>
                      </div>

                      <div className="flex items-center justify-between gap-3">
                        <div className="flex flex-wrap gap-1.5">
                          <Badge variant="outline" className="font-normal capitalize">
                            {project.Kind}
                          </Badge>
                          {shown.map((app) => (
                            <Badge key={app.ID} variant="secondary" className="font-normal">
                              {app.Name}
                            </Badge>
                          ))}
                          {apps.length > shown.length && (
                            <Badge variant="secondary" className="font-normal">
                              +{apps.length - shown.length}
                            </Badge>
                          )}
                        </div>
                        <span className="shrink-0 text-right text-xs text-muted-foreground">
                          {apps.length === 0 ? "No applications" : `${running} of ${apps.length} running`}
                        </span>
                      </div>

                      <div className="flex items-center justify-between gap-2 border-t border-border pt-3">
                        <span className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                          <FolderOpen className="size-3.5 shrink-0" />
                          <span className="truncate font-mono" title={project.RootPath}>
                            {project.RootPath}
                          </span>
                        </span>
                        <Link
                          to={`/projects/${project.ID}`}
                          className={cn(buttonVariants({ variant: "secondary", size: "sm" }), "shrink-0")}
                        >
                          Manage
                          <ArrowRight />
                        </Link>
                      </div>
                    </CardContent>
                  </Card>
                );
              })}
            </div>
          )}

          <Card className="py-0">
            <CardHeader className="flex flex-row items-center justify-between gap-3 p-4 pb-0">
              <CardTitle className="text-base">Applications</CardTitle>
              <Link
                to="/projects"
                className={cn(buttonVariants({ variant: "ghost", size: "sm" }), "shrink-0")}
              >
                Open a project
                <ArrowUpRight />
              </Link>
            </CardHeader>
            <CardContent className="space-y-4 p-4">
              {projects.isPending ? (
                <p className="text-sm text-muted-foreground">Reading applications…</p>
              ) : applications.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No applications yet. Add one inside a project to see it here.
                </p>
              ) : (
                <ul className="divide-y divide-border">
                  {applications.map((app) => (
                    <li key={app.ID} className="flex items-center gap-3 py-2.5">
                      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-accent/15 text-accent">
                        <Server className="size-4" />
                      </span>
                      <div className="min-w-24 shrink-0">
                        <p className="truncate text-sm font-medium">{app.Name}</p>
                      </div>
                      <p className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                        {projectList.find((project) => project.ID === app.ProjectID)?.Name ??
                          "Unknown project"}
                        {app.Directory ? ` · ${app.Directory}` : ""}
                      </p>
                      <span className="shrink-0 text-xs capitalize text-muted-foreground">
                        {app.Status}
                      </span>
                      <span className="hidden w-16 shrink-0 text-right font-mono text-xs text-muted-foreground sm:block">
                        {app.Port === null ? "—" : `:${app.Port}`}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </div>

        <aside className="space-y-4">
          <Card className="py-0">
            <CardHeader className="p-4 pb-0">
              <CardTitle className="text-base">Quick Actions</CardTitle>
            </CardHeader>
            <CardContent className="space-y-0.5 p-2">
              <Link
                to="/projects"
                className={cn(
                  buttonVariants({ variant: "ghost" }),
                  "h-9 w-full justify-start gap-2.5 px-2 font-normal text-muted-foreground",
                )}
              >
                <FolderKanban className="size-4" />
                Create New Project
              </Link>
              {quietActions.slice(0, 1).map((action) => (
                <span
                  key={action.label}
                  aria-disabled
                  title={`${action.label} is planned for a later phase`}
                  className="flex h-9 w-full items-center gap-2.5 rounded-md px-2 text-sm text-muted-foreground opacity-60"
                >
                  <action.icon className="size-4" />
                  {action.label}
                  <PlannedTag />
                </span>
              ))}
              <Link
                to="/logs"
                className={cn(
                  buttonVariants({ variant: "ghost" }),
                  "h-9 w-full justify-start gap-2.5 px-2 font-normal text-muted-foreground",
                )}
              >
                <FileText className="size-4" />
                View Logs
              </Link>
              {quietActions.slice(1).map((action) => (
                <span
                  key={action.label}
                  aria-disabled
                  title={`${action.label} is planned for a later phase`}
                  className="flex h-9 w-full items-center gap-2.5 rounded-md px-2 text-sm text-muted-foreground opacity-60"
                >
                  <action.icon className="size-4" />
                  {action.label}
                  <PlannedTag />
                </span>
              ))}
            </CardContent>
          </Card>

          <Card className="py-0">
            <CardContent className="space-y-3 p-4">
              <div className="flex items-center justify-between gap-2">
                <p className="text-base font-medium">Local Tunnel</p>
                <PlannedTag className="ml-0 border-border" />
              </div>
              <p className="text-xs text-muted-foreground">
                Sharing a local app on a public address arrives in a later phase. Nothing is
                exposed today.
              </p>
              <span
                aria-disabled
                className={cn(
                  buttonVariants({ variant: "outline" }),
                  "w-full cursor-not-allowed opacity-60",
                )}
              >
                Manage Tunnel
                <ArrowRight />
              </span>
            </CardContent>
          </Card>

          <Card className="py-0">
            <CardHeader className="p-4 pb-0">
              <CardTitle className="text-base">Recent Activity</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 p-4">
              {notifications.isPending ? (
                <p className="text-xs text-muted-foreground">Reading activity…</p>
              ) : notifications.error ? (
                <p role="alert" className="text-xs text-destructive">
                  Could not read activity: {String(notifications.error)}
                </p>
              ) : activity.length === 0 ? (
                <p className="text-xs text-muted-foreground">Nothing recorded yet.</p>
              ) : (
                <ul className="space-y-3">
                  {activity.map((entry) => (
                    <li key={entry.ID} className="flex gap-2.5">
                      <span
                        aria-hidden
                        className={cn(
                          "mt-1.5 size-1.5 shrink-0 rounded-full",
                          entry.Severity === "warning" ? "bg-destructive" : "bg-primary",
                        )}
                      />
                      <div className="min-w-0">
                        <p className="truncate text-xs">
                          {entry.Action}
                          {entry.Detail && (
                            <span className="text-muted-foreground"> — {entry.Detail}</span>
                          )}
                        </p>
                        <p className="text-[0.7rem] text-muted-foreground">
                          {ago(entry.CreatedAt)}
                        </p>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </aside>
      </div>
    </div>
  );
}
