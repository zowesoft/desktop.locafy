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
  ChevronDown,
  Copy,
  Eye,
  EyeOff,
  ExternalLink,
  Info,
  Layers,
  MoreHorizontal,
  Plus,
  Terminal as TerminalIcon,
  Trash2,
} from "lucide-react";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { projectsKey } from "@/lib/projects-query";
import { notificationsKey } from "@/components/top-bar-status";
import { cn } from "@/lib/utils";

// ─── Shared primitives ────────────────────────────────────────────────────────

function StatusPill({ status }: { status: "running" | "stopped" | "error" }) {
  const styles = {
    running: "text-success",
    stopped: "text-muted-foreground",
    error: "text-destructive",
  };
  const dotStyles = {
    running: "bg-success",
    stopped: "bg-muted-foreground/40",
    error: "bg-destructive",
  };
  const labels = { running: "Running", stopped: "Stopped", error: "Error" };

  return (
    <span className={cn("inline-flex items-center gap-1.5 text-xs", styles[status])}>
      <span className={cn("size-1.5 rounded-full", dotStyles[status])} />
      {labels[status]}
    </span>
  );
}

function SectionWrap({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-6 py-6">{children}</div>
  );
}

// ─── Services tab ─────────────────────────────────────────────────────────────

interface ServiceRow {
  id: string;
  name: string;
  type: string;
  version: string;
  port: number;
  status: "running" | "stopped" | "error";
  connection?: { host: string; db: string; user: string };
}

const MOCK_SERVICES: ServiceRow[] = [
  {
    id: "postgres",
    name: "PostgreSQL",
    type: "Database",
    version: "17",
    port: 5432,
    status: "running",
    connection: { host: "127.0.0.1:5432", db: "etugora_dev", user: "locafy" },
  },
  {
    id: "redis",
    name: "Redis",
    type: "Cache",
    version: "7.4",
    port: 6379,
    status: "running",
  },
  {
    id: "mailpit",
    name: "Mail",
    type: "SMTP / Mailpit",
    version: "latest",
    port: 2525,
    status: "running",
  },
  {
    id: "minio",
    name: "Object Storage",
    type: "S3 / MinIO",
    version: "latest",
    port: 9000,
    status: "running",
  },
];

export function ProjectServicesTab() {
  const [expanded, setExpanded] = useState<string | null>(null);

  return (
    <SectionWrap>
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold">Services</h2>
          <p className="text-xs text-muted-foreground">
            Databases, caches and other backing services for this project
          </p>
        </div>
        <Button size="sm">
          <Plus className="mr-1.5 size-3.5" />
          Add service
        </Button>
      </div>

      {MOCK_SERVICES.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border py-16 text-center">
          <Layers className="mx-auto mb-3 size-8 text-muted-foreground/40" />
          <p className="text-sm font-medium">No services yet</p>
          <p className="text-xs text-muted-foreground">
            Add a database, cache or other service to this project.
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="border-b border-border bg-muted/40">
              <tr>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Name
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Type
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Version
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Port
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Status
                </th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {MOCK_SERVICES.map((svc) => (
                <>
                  <tr
                    key={svc.id}
                    className="cursor-pointer hover:bg-muted/30"
                    onClick={() => setExpanded(expanded === svc.id ? null : svc.id)}
                  >
                    <td className="px-4 py-2.5 font-medium">{svc.name}</td>
                    <td className="px-4 py-2.5 text-muted-foreground text-xs">{svc.type}</td>
                    <td className="px-4 py-2.5 font-mono text-xs">{svc.version}</td>
                    <td className="px-4 py-2.5 font-mono text-xs">{svc.port}</td>
                    <td className="px-4 py-2.5">
                      <StatusPill status={svc.status} />
                    </td>
                    <td className="px-4 py-2.5">
                      <div className="flex justify-end gap-1">
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2 text-xs"
                          onClick={(e) => {
                            e.stopPropagation();
                            setExpanded(expanded === svc.id ? null : svc.id);
                          }}
                        >
                          <ChevronDown
                            className={cn(
                              "size-3.5 transition-transform",
                              expanded === svc.id && "rotate-180",
                            )}
                          />
                        </Button>
                        <DropdownMenu>
                          <DropdownMenuTrigger
                            className="inline-flex h-7 cursor-pointer items-center justify-center rounded-md px-2 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <MoreHorizontal className="size-3.5" />
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem>Restart service</DropdownMenuItem>
                            <DropdownMenuItem>View logs</DropdownMenuItem>
                            <DropdownMenuItem className="text-destructive">
                              Remove service
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </td>
                  </tr>
                  {expanded === svc.id && svc.connection && (
                    <tr key={`${svc.id}-expanded`} className="bg-muted/20">
                      <td colSpan={6} className="px-4 py-3">
                        <div className="flex flex-wrap gap-4 text-xs">
                          <div className="space-y-0.5">
                            <span className="text-muted-foreground">Host</span>
                            <p className="font-mono">{svc.connection.host}</p>
                          </div>
                          <div className="space-y-0.5">
                            <span className="text-muted-foreground">Database</span>
                            <p className="font-mono">{svc.connection.db}</p>
                          </div>
                          <div className="space-y-0.5">
                            <span className="text-muted-foreground">User</span>
                            <p className="font-mono">{svc.connection.user}</p>
                          </div>
                          <div className="space-y-0.5">
                            <span className="text-muted-foreground">Password</span>
                            <p className="font-mono text-muted-foreground">
                              (from environment)
                            </p>
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </SectionWrap>
  );
}

// ─── Domains tab ──────────────────────────────────────────────────────────────

const MOCK_DOMAINS = [
  { domain: "api.etugora.test", port: 8000, https: true, app: "API", status: "running" as const },
  { domain: "auth.etugora.test", port: 3001, https: true, app: "Auth", status: "running" as const },
  { domain: "ml.etugora.test", port: 5000, https: false, app: "ML Service", status: "running" as const },
];

export function ProjectDomainsTab() {
  return (
    <SectionWrap>
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold">Domains</h2>
          <p className="text-xs text-muted-foreground">
            Local hostnames assigned to this project's applications
          </p>
        </div>
        <Button size="sm">
          <Plus className="mr-1.5 size-3.5" />
          Add domain
        </Button>
      </div>

      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead className="border-b border-border bg-muted/40">
            <tr>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Domain
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                App
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Port
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                HTTPS
              </th>
              <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                Status
              </th>
              <th className="px-4 py-2.5" />
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {MOCK_DOMAINS.map((row) => (
              <tr key={row.domain} className="hover:bg-muted/30">
                <td className="px-4 py-2.5">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs">{row.domain}</span>
                    <button
                      title="Open in browser"
                      className="text-muted-foreground hover:text-foreground"
                    >
                      <ExternalLink className="size-3.5" />
                    </button>
                  </div>
                </td>
                <td className="px-4 py-2.5 text-muted-foreground">{row.app}</td>
                <td className="px-4 py-2.5 font-mono text-xs">{row.port}</td>
                <td className="px-4 py-2.5">
                  {row.https ? (
                    <Badge className="bg-success/10 text-success border-success/20 text-xs">
                      HTTPS
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-xs text-muted-foreground">
                      HTTP
                    </Badge>
                  )}
                </td>
                <td className="px-4 py-2.5">
                  <StatusPill status={row.status} />
                </td>
                <td className="px-4 py-2.5">
                  <div className="flex justify-end gap-1">
                    <DropdownMenu>
                      <DropdownMenuTrigger className="inline-flex h-7 cursor-pointer items-center justify-center rounded-md px-2 text-muted-foreground hover:bg-muted hover:text-foreground">
                        <MoreHorizontal className="size-3.5" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem>Edit domain</DropdownMenuItem>
                        <DropdownMenuItem className="text-destructive">
                          Remove domain
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-start gap-2 rounded-lg border border-border p-3 text-xs text-muted-foreground">
        <Info className="mt-0.5 size-3.5 shrink-0" />
        Domains resolve via your system hosts file. Changes take effect immediately. Domains use
        the .test suffix reserved for local testing — they never leave this machine.
      </div>
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

type LogSeverity = "error" | "warn" | "info" | "debug";

interface ProjectLogEntry {
  id: string;
  time: string;
  source: string;
  severity: LogSeverity;
  message: string;
}

const MOCK_PROJECT_LOGS: ProjectLogEntry[] = [
  { id: "1", time: "16:47:04.213", source: "api", severity: "info", message: "Laravel application started — serving on http://api.etugora.test" },
  { id: "2", time: "16:47:05.001", source: "auth", severity: "info", message: "NestJS server started on port 3001" },
  { id: "3", time: "16:47:06.337", source: "ml", severity: "warn", message: "CUDA not available — falling back to CPU inference" },
  { id: "4", time: "16:47:08.892", source: "api", severity: "error", message: "DB_PASSWORD env var is not set — using empty password" },
  { id: "5", time: "16:47:14.012", source: "api", severity: "info", message: "GET /api/v1/users 200 42ms" },
  { id: "6", time: "16:47:14.880", source: "api", severity: "info", message: "POST /api/v1/auth/login 200 88ms" },
  { id: "7", time: "16:47:15.233", source: "ml", severity: "error", message: "RuntimeError: ONNX model weights not found at models/sentiment-v2.onnx" },
  { id: "8", time: "16:47:18.441", source: "api", severity: "warn", message: "Slow query detected (>500ms): SELECT * FROM audit_logs WHERE project_id = 42" },
  { id: "9", time: "16:47:22.100", source: "auth", severity: "info", message: "JWT signing key rotated successfully" },
  { id: "10", time: "16:47:25.004", source: "api", severity: "info", message: "GET /api/v1/projects 200 15ms" },
];

function SeverityBadge({ severity }: { severity: LogSeverity }) {
  const styles: Record<LogSeverity, string> = {
    error: "bg-destructive/10 text-destructive border-destructive/20",
    warn: "bg-amber-500/10 text-amber-400 border-amber-500/20",
    info: "bg-blue-500/10 text-blue-400 border-blue-500/20",
    debug: "bg-muted text-muted-foreground border-transparent",
  };
  return (
    <span className={cn("inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium border", styles[severity])}>
      {severity}
    </span>
  );
}

export function ProjectLogsTab() {
  const [sourceFilter, setSourceFilter] = useState("all");
  const [severityFilter, setSeverityFilter] = useState<LogSeverity | "all">("all");

  const filtered = MOCK_PROJECT_LOGS.filter((entry) => {
    if (sourceFilter !== "all" && entry.source !== sourceFilter) return false;
    if (severityFilter !== "all" && entry.severity !== severityFilter) return false;
    return true;
  });

  const sources = Array.from(new Set(MOCK_PROJECT_LOGS.map((e) => e.source)));

  return (
    <div className="flex min-h-full flex-col">
      {/* Filter bar */}
      <div className="border-b border-border px-6 py-3">
        <div className="flex flex-wrap items-center gap-2">
          {/* Source */}
          <div className="relative">
            <select
              value={sourceFilter}
              onChange={(e) => setSourceFilter(e.target.value)}
              className="h-8 appearance-none rounded-md border border-border bg-background pl-3 pr-7 text-xs focus:outline-none"
            >
              <option value="all">All apps</option>
              {sources.map((s) => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
            <ChevronDown className="pointer-events-none absolute right-2 top-1/2 size-3 -translate-y-1/2 text-muted-foreground" />
          </div>

          {/* Severity */}
          <div className="flex rounded-md border border-border bg-muted/40 p-0.5">
            {(["all", "error", "warn", "info", "debug"] as (LogSeverity | "all")[]).map((s) => (
              <button
                key={s}
                type="button"
                onClick={() => setSeverityFilter(s)}
                className={cn(
                  "rounded px-2.5 py-1 text-xs capitalize transition-colors",
                  severityFilter === s
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {s}
              </button>
            ))}
          </div>

          <span className="ml-auto text-xs text-muted-foreground">
            Project-scoped · machine-wide logs in{" "}
            <span className="text-primary">Logs</span> sidebar
          </span>
        </div>
      </div>

      {/* Log lines */}
      <div className="flex-1 overflow-y-auto font-mono">
        <div className="divide-y divide-border/50">
          {filtered.map((entry) => (
            <div
              key={entry.id}
              className={cn(
                "flex items-start gap-3 px-6 py-2 text-xs hover:bg-muted/30",
                entry.severity === "error" && "bg-destructive/5",
                entry.severity === "warn" && "bg-amber-500/5",
              )}
            >
              <span className="shrink-0 text-muted-foreground/60 tabular-nums">{entry.time}</span>
              <span className="min-w-[40px] text-primary">{entry.source}</span>
              <SeverityBadge severity={entry.severity} />
              <span
                className={cn(
                  "flex-1 break-all",
                  entry.severity === "error"
                    ? "text-destructive"
                    : entry.severity === "debug"
                      ? "text-muted-foreground/70"
                      : "text-foreground",
                )}
              >
                {entry.message}
              </span>
            </div>
          ))}
        </div>
      </div>
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

