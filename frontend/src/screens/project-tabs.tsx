/**
 * Project-scoped tab content components for the project workspace.
 * Each exported component is wired to a route in project-workspace.tsx.
 * All data is mock/static — Go backend wiring comes later.
 */
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useOutletContext } from "react-router-dom";
import {
  AlertTriangle,
  
  Copy,
  Eye,
  EyeOff,
  ExternalLink,
  Info,
  Layers,
  Terminal as TerminalIcon,
  Trash2,
} from "lucide-react";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import { LiveLogs } from "@/components/live-logs";
import { ProjectServices } from "@/components/project-services";
import { ProjectDomains } from "@/components/project-domains";
import type { Project } from "../../bindings/locafy.dev/desktop/internal/projects/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { projectsKey } from "@/lib/projects-query";
import { notificationsKey } from "@/components/top-bar-status";
import { cn } from "@/lib/utils";

// ─── Shared primitives ────────────────────────────────────────────────────────

function SectionWrap({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-6 py-6">{children}</div>
  );
}

// ─── Services tab ─────────────────────────────────────────────────────────────

export function ProjectServicesTab() {
  const project = useOutletContext<Project>();
  return <ProjectServices projectID={project.ID} />;
}

// ─── Domains tab ──────────────────────────────────────────────────────────────

export function ProjectDomainsTab() {
  const project = useOutletContext<Project>();
  return (
    <SectionWrap>
      <ProjectDomains projectID={project.ID} />
    </SectionWrap>
  );
}

// ─── Environment tab ──────────────────────────────────────────────────────────

type EnvScope = "project" | "application" | "service";
type EnvSource = "project" | "generated" | "system";

interface EnvVar {
  id: string;
  key: string;
  value: string;
  scope: EnvScope;
  source: EnvSource;
  masked?: boolean;
}

const MOCK_ENV_VARS: EnvVar[] = [
  { id: "1", key: "APP_ENV", value: "local", scope: "project", source: "project" },
  { id: "2", key: "APP_DEBUG", value: "true", scope: "project", source: "project" },
  { id: "3", key: "DB_HOST", value: "127.0.0.1", scope: "project", source: "generated" },
  { id: "4", key: "DB_PORT", value: "5432", scope: "project", source: "generated" },
  { id: "5", key: "DB_DATABASE", value: "etugora_dev", scope: "project", source: "generated" },
  { id: "6", key: "DB_USERNAME", value: "locafy", scope: "project", source: "generated" },
  { id: "7", key: "DB_PASSWORD", value: "locafy-dev-secret", scope: "project", source: "generated", masked: true },
  { id: "8", key: "REDIS_HOST", value: "127.0.0.1", scope: "project", source: "generated" },
  { id: "9", key: "REDIS_PORT", value: "6379", scope: "project", source: "generated" },
  { id: "10", key: "MAIL_HOST", value: "127.0.0.1", scope: "project", source: "generated" },
  { id: "11", key: "MAIL_PORT", value: "2525", scope: "project", source: "generated" },
];

function SourceBadge({ source }: { source: EnvSource }) {
  if (source === "generated") {
    return (
      <Badge variant="secondary" className="text-xs font-normal text-muted-foreground">
        Generated
      </Badge>
    );
  }
  if (source === "system") {
    return (
      <Badge variant="secondary" className="text-xs font-normal text-muted-foreground">
        System
      </Badge>
    );
  }
  return (
    <Badge
      variant="secondary"
      className="bg-primary/10 text-primary border-primary/20 text-xs font-normal"
    >
      Project
    </Badge>
  );
}

export function ProjectEnvironmentTab() {
  const [scope, setScope] = useState<EnvScope>("project");
  const [revealed, setRevealed] = useState<Set<string>>(new Set());

  const toggleReveal = (id: string) => {
    setRevealed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const filtered = MOCK_ENV_VARS.filter((v) => v.scope === scope);

  return (
    <SectionWrap>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="text-base font-semibold">Environment</h2>
          <p className="text-xs text-muted-foreground">
            Variables injected into this project's processes
          </p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="outline">
            Import .env
          </Button>
          <Button size="sm" variant="outline">
            Export
          </Button>
        </div>
      </div>

      {/* Scope selector */}
      <div className="inline-flex rounded-md border border-border bg-muted/40 p-0.5">
        {(["project", "application", "service"] as EnvScope[]).map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => setScope(s)}
            className={cn(
              "rounded px-3 py-1 text-sm capitalize transition-colors",
              scope === s
                ? "bg-card text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {s}
          </button>
        ))}
      </div>

      {/* Env var table */}
      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead className="border-b border-border bg-muted/40">
            <tr>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Key
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Value
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Source
              </th>
              <th className="px-4 py-2.5" />
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {filtered.map((env) => {
              const isRevealed = revealed.has(env.id);
              return (
                <tr key={env.id} className="hover:bg-muted/30">
                  <td className="px-4 py-2.5 font-mono text-xs font-medium">{env.key}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">
                    {env.masked && !isRevealed
                      ? "••••••••••••"
                      : env.value}
                  </td>
                  <td className="px-4 py-2.5">
                    <SourceBadge source={env.source} />
                  </td>
                  <td className="px-4 py-2.5">
                    <div className="flex justify-end gap-1">
                      {env.masked && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2"
                          onClick={() => toggleReveal(env.id)}
                          title={isRevealed ? "Hide" : "Reveal"}
                        >
                          {isRevealed ? (
                            <EyeOff className="size-3.5" />
                          ) : (
                            <Eye className="size-3.5" />
                          )}
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-7 px-2"
                        title="Copy value"
                        onClick={() => void navigator.clipboard.writeText(env.value)}
                      >
                        <Copy className="size-3.5" />
                      </Button>
                      {env.source === "project" && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2 text-destructive hover:text-destructive"
                          title="Delete variable"
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Precedence note */}
      <div className="flex items-start gap-2 rounded-lg border border-border p-3 text-xs text-muted-foreground">
        <Info className="mt-0.5 size-3.5 shrink-0" />
        <span>
          <strong className="text-foreground">Precedence:</strong> machine → project → application
          → service. Locafy-generated variables (host, port, credentials) are injected at runtime
          and do not overwrite your source .env files.
        </span>
      </div>
    </SectionWrap>
  );
}

// ─── Logs tab (project-scoped) ────────────────────────────────────────────────

export function ProjectLogsTab() {
  const project = useOutletContext<Project>();

  return (
    <div className="flex min-h-full flex-col">
      <div className="border-b border-border px-6 py-3">
        <p className="text-xs text-muted-foreground">
          Output from this project's applications and services, oldest first. Credentials are
          replaced before a line is stored; every project's output is in the Logs screen.
        </p>
      </div>
      <LiveLogs projectID={project.ID} />
    </div>
  );
}

// ─── Terminal tab (project-scoped) ────────────────────────────────────────────

export function ProjectTerminalTab() {
  const project = useOutletContext<Project>();

  return (
    <SectionWrap>
      <Card className="overflow-hidden">
        <div className="flex items-center gap-1.5 border-b border-border bg-muted/40 px-4 py-2">
          <span className="size-3 rounded-full bg-destructive/50" />
          <span className="size-3 rounded-full bg-amber-400/50" />
          <span className="size-3 rounded-full bg-success/50" />
          <span className="ml-3 font-mono text-xs text-muted-foreground">
            {project.Name} — project terminal
          </span>
        </div>
        <CardContent className="flex min-h-[280px] flex-col items-center justify-center gap-4 bg-[#0D0D12] py-14">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary/15 text-primary">
            <TerminalIcon className="size-6" />
          </div>
          <div className="text-center space-y-2">
            <p className="font-medium text-white">Built-in terminal is coming soon</p>
            <p className="text-sm text-muted-foreground max-w-xs">
              Will open in <span className="text-muted-foreground/80 font-mono">{project.RootPath}</span> using your preferred shell.
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="mt-2 bg-transparent border-muted-foreground/30 text-muted-foreground hover:text-foreground hover:border-muted-foreground"
          >
            <ExternalLink className="mr-1.5 size-3.5" />
            Open in system terminal
          </Button>
        </CardContent>
      </Card>
    </SectionWrap>
  );
}

// ─── Processes tab ────────────────────────────────────────────────────────────

export function ProjectProcessesTab() {
  return (
    <SectionWrap>
      <div>
        <h2 className="text-base font-semibold">Processes</h2>
        <p className="text-xs text-muted-foreground">
          Only processes Locafy launched for this project — unrelated system processes are never
          shown
        </p>
      </div>

      <div className="rounded-lg border border-dashed border-border py-16 text-center">
        <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-muted">
          <Layers className="size-5 text-muted-foreground/60" />
        </div>
        <p className="text-sm font-medium">Process supervision coming later</p>
        <p className="mt-1 max-w-xs mx-auto text-xs text-muted-foreground">
          Per-process CPU, memory and status monitoring arrives with application lifecycle (Phase
          3). For now, check the Logs tab for application output.
        </p>
      </div>
    </SectionWrap>
  );
}

// ─── Project Settings tab ─────────────────────────────────────────────────────

function formatDate(iso: string): string {
  const ts = Date.parse(iso);
  return Number.isNaN(ts) ? iso : new Date(ts).toLocaleString();
}

function OverrideCard({ project }: { project: Project }) {
  const queryClient = useQueryClient();
  const key = ["projects", project.ID, "override"] as const;
  const status = useQuery({
    queryKey: key,
    queryFn: () => ProjectsService.OverrideStatus(project.ID),
  });
  const toggle = useMutation({
    mutationFn: (enabled: boolean) => ProjectsService.SetOverrideFile(project.ID, enabled),
    onSuccess: (state) => {
      queryClient.setQueryData(key, state);
      void queryClient.invalidateQueries({ queryKey: projectsKey, exact: true });
      void queryClient.invalidateQueries({ queryKey: notificationsKey });
    },
  });

  const present = status.data?.Present ?? false;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Project settings file</CardTitle>
        <CardDescription>
          Locafy keeps this project's settings in its own database. Add an override file only when
          you want to change them from inside the project folder.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-start justify-between gap-4 rounded-lg border border-border p-4">
          <div className="min-w-0 flex-1 space-y-1">
            <p className="text-sm font-medium">Override settings from locafy.project.json</p>
            <p className="text-xs text-muted-foreground">
              While the file exists, its name and description replace the stored values. Nothing is
              written into the folder until you turn this on, and turning it off removes the file.
            </p>
          </div>
          <Switch
            aria-label="Override settings from locafy.project.json"
            checked={present}
            disabled={status.isPending || toggle.isPending}
            onCheckedChange={(next) => toggle.mutate(next)}
          />
        </div>

        {present && status.data && (
          <p className="break-all font-mono text-xs text-muted-foreground">{status.data.Path}</p>
        )}

        {(status.error ?? toggle.error) && (
          <p role="alert" className="text-xs text-destructive">
            {String(status.error ?? toggle.error)}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

export function ProjectSettingsTab() {
  const project = useOutletContext<Project>();
  const [name, setName] = useState(project.Name);
  const [description, setDescription] = useState(project.Description ?? "");

  return (
    <SectionWrap>
      {/* Basic info */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Project details</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-xs">Project name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="max-w-sm"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">Description</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional short description"
              className="max-w-sm"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Root directory</Label>
            <Input
              value={project.RootPath}
              readOnly
              className="max-w-sm font-mono text-xs text-muted-foreground"
            />
          </div>
          <div className="grid grid-cols-2 gap-4 max-w-sm">
            <div>
              <Label className="text-xs text-muted-foreground">Kind</Label>
              <p className="mt-0.5 text-sm capitalize">{project.Kind}</p>
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">Created</Label>
              <p className="mt-0.5 text-xs">{formatDate(project.CreatedAt)}</p>
            </div>
          </div>
          <Button size="sm">Save changes</Button>
        </CardContent>
      </Card>

      <OverrideCard project={project} />

      {/* Danger zone */}
      <Card className="border-destructive/30">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm text-destructive">
            <AlertTriangle className="size-4" />
            Danger zone
          </CardTitle>
          <CardDescription>
            These actions cannot be undone from within Locafy.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-start justify-between gap-4 rounded-lg border border-destructive/20 bg-destructive/5 p-4">
            <div className="space-y-1">
              <p className="text-sm font-medium">Remove project from Locafy</p>
              <p className="text-xs text-muted-foreground">
                Removes <strong className="text-foreground">{project.Name}</strong> from Locafy's
                list. The folder and all its files are <strong className="text-foreground">never
                deleted</strong> — only the Locafy record is removed. Services and runtimes for
                this project are stopped first.
              </p>
            </div>
            <Button
              variant="destructive"
              size="sm"
              className="shrink-0"
            >
              <Trash2 className="mr-1.5 size-3.5" />
              Remove from Locafy
            </Button>
          </div>
        </CardContent>
      </Card>
    </SectionWrap>
  );
}

