import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Check,
  ChevronRight,
  Clock,
  Download,
  Globe,
  HardDrive,
  Info,
  Layers,
  Monitor,
  Network,
  RefreshCw,
  RotateCcw,
  Save,
  Server,
  Settings,
  Trash2,
  Zap,
} from "lucide-react";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import * as SettingsService from "../../bindings/locafy.dev/desktop/internal/settings/service";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import type { NotificationPreferences } from "../../bindings/locafy.dev/desktop/internal/status/models";
import { notificationPreferencesKey } from "@/components/top-bar-status";
import { useRuntimeCatalog } from "@/lib/runtimes-query";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

// ─── Query keys ──────────────────────────────────────────────────────────────

const workspaceKey = ["projects", "workspace"] as const;
const preferencesKey = notificationPreferencesKey;
const settingsKey = ["settings"] as const;

// ─── Types ───────────────────────────────────────────────────────────────────

type SettingsTab =
  | "general"
  | "services"
  | "runtimes"
  | "domains"
  | "tunnels"
  | "network"
  | "storage"
  | "appearance"
  | "updates";

interface NavItem {
  id: SettingsTab;
  label: string;
  icon: React.ElementType;
}

// ─── Primitives ──────────────────────────────────────────────────────────────

function Switch({
  checked,
  onChange,
  disabled,
  id,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  id?: string;
}) {
  return (
    <button
      id={id}
      role="switch"
      type="button"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50",
        checked ? "bg-primary" : "bg-muted-foreground/30",
      )}
    >
      <span
        className={cn(
          "pointer-events-none block h-4 w-4 rounded-full bg-white shadow-lg ring-0 transition-transform",
          checked ? "translate-x-4" : "translate-x-0",
        )}
      />
    </button>
  );
}

function ToggleGroup({
  options,
  value,
  onChange,
  disabled,
}: {
  options: { value: string; label: string }[];
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="inline-flex rounded-md border border-border bg-muted/40 p-0.5">
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          disabled={disabled}
          onClick={() => onChange(opt.value)}
          className={cn(
            "rounded px-3 py-1 text-sm transition-colors disabled:opacity-50",
            value === opt.value
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

function SwitchRow({
  label,
  description,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-4 py-3">
      <div className="min-w-0 flex-1 space-y-0.5">
        <p className="text-sm font-medium leading-none">{label}</p>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      <Switch checked={checked} onChange={onChange} disabled={disabled} />
    </div>
  );
}

function SectionHeader({ title, description }: { title: string; description?: string }) {
  return (
    <div className="mb-4 space-y-1">
      <h3 className="text-sm font-semibold tracking-tight">{title}</h3>
      {description && <p className="text-xs text-muted-foreground">{description}</p>}
    </div>
  );
}

function InfoCallout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex gap-2.5 rounded-lg border border-blue-500/20 bg-blue-500/5 px-3 py-2.5 text-xs text-muted-foreground">
      <Info className="mt-0.5 size-3.5 shrink-0 text-blue-400" />
      <span>{children}</span>
    </div>
  );
}

function WarnCallout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex gap-2.5 rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2.5 text-xs text-muted-foreground">
      <Info className="mt-0.5 size-3.5 shrink-0 text-amber-400" />
      <span>{children}</span>
    </div>
  );
}

function PlannedOverlay() {
  return (
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 rounded-lg bg-background/80 backdrop-blur-sm">
      <Badge variant="secondary" className="gap-1.5 px-3 py-1 text-xs">
        <Clock className="size-3" />
        Planned
      </Badge>
      <p className="max-w-xs text-center text-sm text-muted-foreground">
        Public tunnels are planned for a later phase and are not yet available.
      </p>
    </div>
  );
}

// ─── Nav sidebar ─────────────────────────────────────────────────────────────

const navItems: NavItem[] = [
  { id: "general", label: "General", icon: Settings },
  { id: "services", label: "Services & Instances", icon: Server },
  { id: "runtimes", label: "Runtimes", icon: Layers },
  { id: "domains", label: "Domains & HTTPS", icon: Globe },
  { id: "tunnels", label: "Tunnels", icon: Network },
  { id: "network", label: "Network", icon: Network },
  { id: "storage", label: "Storage", icon: HardDrive },
  { id: "appearance", label: "Appearance", icon: Monitor },
  { id: "updates", label: "Updates", icon: Download },
];

// ─── Tab: General ────────────────────────────────────────────────────────────

function GeneralTab() {
  const queryClient = useQueryClient();
  const [workspaceDraft, setWorkspaceDraft] = useState("");
  const [editor, setEditor] = useState("vscode");
  const [terminal, setTerminal] = useState("builtin");
  const [launchAtLogin, setLaunchAtLogin] = useState(true);
  const [restoreRunning, setRestoreRunning] = useState(true);
  const [keepInTray, setKeepInTray] = useState(true);
  const [shareUsage, setShareUsage] = useState(false);
  // Developer section state
  const [devOpen, setDevOpen] = useState(false);
  const [settingKey, setSettingKey] = useState("");
  const [settingValue, setSettingValue] = useState("");

  const workspace = useQuery({
    queryKey: workspaceKey,
    queryFn: () => ProjectsService.WorkspaceDir(),
  });

  useEffect(() => {
    if (workspace.data !== undefined) setWorkspaceDraft(workspace.data);
  }, [workspace.data]);

  const saveWorkspace = useMutation({
    mutationFn: (path: string) => ProjectsService.SetWorkspaceDir(path),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: workspaceKey }),
  });

  const preferences = useQuery({
    queryKey: preferencesKey,
    queryFn: () => StatusService.NotificationPreferences(),
  });

  const savePreferences = useMutation({
    mutationFn: (prefs: NotificationPreferences) =>
      StatusService.SetNotificationPreferences(prefs),
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey: preferencesKey });
      const previous = queryClient.getQueryData<NotificationPreferences>(preferencesKey);
      queryClient.setQueryData(preferencesKey, next);
      return { previous };
    },
    onError: (_error, _next, context) => {
      if (context?.previous) queryClient.setQueryData(preferencesKey, context.previous);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: preferencesKey });
      void queryClient.invalidateQueries({ queryKey: ["notifications"] });
    },
  });

  const prefs = preferences.data;

  // SettingsService — raw key/value store (developer utility)
  const storedSettings = useQuery({
    queryKey: settingsKey,
    queryFn: async () => (await SettingsService.All()) ?? [],
  });

  const saveSetting = useMutation({
    mutationFn: (s: { key: string; value: string }) =>
      SettingsService.SetString(s.key, s.value),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: settingsKey });
      setSettingKey("");
      setSettingValue("");
    },
  });

  const canSubmitSetting =
    settingKey.trim() !== "" && settingValue.trim() !== "" && !saveSetting.isPending;

  const submitSetting = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmitSetting) return;
    saveSetting.mutate({ key: settingKey.trim(), value: settingValue.trim() });
  };

  return (
    <div className="space-y-8">
      {/* Workspace */}
      <div>
        <SectionHeader
          title="Workspace"
          description="Where Locafy stores managed project folders."
        />
        <div className="space-y-3">
          <div className="flex gap-2">
            <Input
              aria-label="Workspace folder"
              value={workspaceDraft}
              onChange={(e) => setWorkspaceDraft(e.target.value)}
              className="font-mono text-xs"
              placeholder="~/Projects"
            />
            <Button
              variant="outline"
              onClick={() => saveWorkspace.mutate(workspaceDraft)}
              disabled={
                saveWorkspace.isPending ||
                workspaceDraft === "" ||
                workspaceDraft === workspace.data
              }
            >
              {saveWorkspace.isPending ? "Saving…" : "Save workspace folder"}
            </Button>
            <Button
              variant="ghost"
              onClick={() => saveWorkspace.mutate("")}
              disabled={saveWorkspace.isPending}
              className="text-xs text-muted-foreground"
            >
              Use default
            </Button>
          </div>
          {saveWorkspace.error && (
            <p role="alert" className="text-xs text-destructive">
              {String(saveWorkspace.error)}
            </p>
          )}
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Default editor</Label>
            <ToggleGroup
              options={[
                { value: "vscode", label: "VS Code" },
                { value: "cursor", label: "Cursor" },
                { value: "jetbrains", label: "JetBrains" },
                { value: "zed", label: "Zed" },
              ]}
              value={editor}
              onChange={setEditor}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Terminal</Label>
            <ToggleGroup
              options={[
                { value: "builtin", label: "Built-in" },
                { value: "system", label: "System" },
              ]}
              value={terminal}
              onChange={setTerminal}
            />
          </div>
        </div>
      </div>

      {/* Startup and behavior */}
      <div>
        <SectionHeader title="Startup and behavior" />
        <div className="divide-y divide-border rounded-lg border border-border">
          <div className="px-4">
            <SwitchRow
              label="Launch Locafy at login"
              description="Start automatically when you sign into your computer"
              checked={launchAtLogin}
              onChange={setLaunchAtLogin}
            />
          </div>
          <div className="px-4">
            <SwitchRow
              label="Restore running projects"
              description="Resume projects that were active when Locafy last closed"
              checked={restoreRunning}
              onChange={setRestoreRunning}
            />
          </div>
          <div className="px-4">
            <SwitchRow
              label="Keep running in the system tray"
              description="Closing the window hides Locafy rather than quitting it"
              checked={keepInTray}
              onChange={setKeepInTray}
            />
          </div>
          <div className="px-4">
            <SwitchRow
              label="Share anonymous usage data"
              description="Helps improve Locafy — no project names or file paths are included"
              checked={shareUsage}
              onChange={setShareUsage}
            />
          </div>
        </div>
      </div>

      {/* Notifications */}
      <div>
        <SectionHeader
          title="Notifications"
          description="Which kinds of activity the bell shows"
        />
        {preferences.isPending ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : prefs ? (
          <div className="divide-y divide-border rounded-lg border border-border">
            {[
              {
                key: "Warnings" as keyof NotificationPreferences,
                label: "Warnings",
                desc: "Conditions that need acting on, such as low disk space",
              },
              {
                key: "Project" as keyof NotificationPreferences,
                label: "Project activity",
                desc: "Projects created, linked, renamed or removed",
              },
              {
                key: "Machine" as keyof NotificationPreferences,
                label: "Machine events",
                desc: "Engine starts and other machine-level activity",
              },
              {
                key: "Sound" as keyof NotificationPreferences,
                label: "Play a sound",
                desc: "A short chime when new activity arrives while Locafy is running",
              },
            ].map(({ key, label, desc }) => (
              <label key={key} className="flex items-start justify-between gap-4 px-4 py-3">
                <div className="min-w-0 flex-1 space-y-0.5">
                  <p className="text-sm font-medium leading-none">{label}</p>
                  <p className="text-xs text-muted-foreground">{desc}</p>
                </div>
                <input
                  type="checkbox"
                  aria-label={label}
                  className="mt-1 size-4 accent-primary"
                  checked={!!prefs[key]}
                  disabled={savePreferences.isPending}
                  onChange={(e) =>
                    savePreferences.mutate({ ...prefs, [key]: e.target.checked })
                  }
                />
              </label>
            ))}
          </div>
        ) : null}
        {savePreferences.error && (
          <p role="alert" className="mt-2 text-xs text-destructive">
            {String(savePreferences.error)}
          </p>
        )}
      </div>

      {/* Developer — stored settings key/value store */}
      <div>
        <button
          type="button"
          className="flex w-full items-center gap-2 text-left text-xs text-muted-foreground hover:text-foreground"
          onClick={() => setDevOpen((p) => !p)}
        >
          <ChevronRight
            className={cn("size-3.5 transition-transform", devOpen && "rotate-90")}
          />
          Developer · stored settings
        </button>

        {devOpen && (
          <div className="mt-3 space-y-3">
            {/* Existing entries */}
            {storedSettings.isPending ? (
              <p className="text-xs text-muted-foreground">Loading…</p>
            ) : (storedSettings.data ?? []).length === 0 ? (
              <p className="text-xs text-muted-foreground">No settings stored yet.</p>
            ) : (
              <div className="overflow-hidden rounded-lg border border-border">
                <table className="w-full text-sm">
                  <tbody className="divide-y divide-border">
                    {(storedSettings.data ?? []).map((s) => (
                      <tr key={s.Key}>
                        <td className="px-3 py-2 font-mono text-xs text-accent">{s.Key}</td>
                        <td className="px-3 py-2 text-xs">{s.Value}</td>
                        <td className="px-3 py-2">
                          <Badge variant="secondary" className="text-xs">{s.Type}</Badge>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {storedSettings.error && (
              <p role="alert" className="text-xs text-destructive">
                {String(storedSettings.error)}
              </p>
            )}

            {/* Add entry */}
            <form className="flex gap-2" onSubmit={submitSetting}>
              <Input
                aria-label="Setting key"
                placeholder="Key"
                value={settingKey}
                onChange={(e) => setSettingKey(e.target.value)}
                className="font-mono text-xs"
              />
              <Input
                aria-label="Setting value"
                placeholder="Value"
                value={settingValue}
                onChange={(e) => setSettingValue(e.target.value)}
                className="font-mono text-xs"
              />
              <Button type="submit" size="sm" disabled={!canSubmitSetting}>
                {saveSetting.isPending ? "Saving…" : "Set"}
              </Button>
            </form>
            {saveSetting.error && (
              <p role="alert" className="text-xs text-destructive">
                {String(saveSetting.error)}
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Tab: Services & Instances ────────────────────────────────────────────────

const SERVICES = [
  {
    id: "postgres",
    name: "PostgreSQL 17",
    description: "Full-featured relational database",
    port: 5432,
    mode: "shared" as const,
    recommended: "shared",
  },
  {
    id: "mysql",
    name: "MySQL 8",
    description: "Popular relational database",
    port: 3306,
    mode: "dedicated" as const,
    recommended: "dedicated",
  },
  {
    id: "redis",
    name: "Redis 7",
    description: "In-memory key-value store",
    port: 6379,
    mode: "shared" as const,
    recommended: "shared",
  },
  {
    id: "mailpit",
    name: "Mailpit",
    description: "SMTP catch-all for local email testing",
    port: 2525,
    mode: "shared" as const,
    recommended: "shared",
  },
  {
    id: "minio",
    name: "MinIO",
    description: "S3-compatible object storage",
    port: 9000,
    mode: "shared" as const,
    recommended: "shared",
  },
];

const ACTIVE_INSTANCES = [
  { name: "Shared PostgreSQL 17", mode: "shared", port: 5432, usedBy: "3 projects", status: "running" },
  { name: "Redis (etugora)", mode: "dedicated", port: 6380, usedBy: "Etugora", status: "running" },
  { name: "Mailpit", mode: "shared", port: 2525, usedBy: "5 projects", status: "running" },
];

function ServicesTab() {
  const [modes, setModes] = useState<Record<string, string>>(
    Object.fromEntries(SERVICES.map((s) => [s.id, s.mode])),
  );
  const [lifecycle, setLifecycle] = useState({
    startOnDemand: true,
    stopIdle: true,
    restartCrashed: true,
    backupBeforeSwitching: false,
  });

  return (
    <div className="space-y-8">
      <div className="flex items-center gap-3">
        <h2 className="text-base font-semibold">Service instances</h2>
        <Badge variant="secondary" className="font-mono text-xs">
          {SERVICES.length} services · {3} projects
        </Badge>
      </div>
      <p className="text-sm text-muted-foreground">
        <strong className="text-foreground">Shared</strong> — one instance serves all projects.
        Uses less memory but projects share data.{" "}
        <strong className="text-foreground">Dedicated</strong> — each project gets its own
        isolated instance.
      </p>

      {/* Per-service rows */}
      <div className="divide-y divide-border rounded-lg border border-border">
        {SERVICES.map((svc) => (
          <div key={svc.id} className="flex items-center gap-4 px-4 py-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{svc.name}</span>
                {svc.recommended === modes[svc.id] && (
                  <Badge variant="secondary" className="text-xs">
                    Recommended
                  </Badge>
                )}
              </div>
              <p className="text-xs text-muted-foreground">
                {svc.description} · port {svc.port}
              </p>
            </div>
            <ToggleGroup
              options={[
                { value: "shared", label: "Shared" },
                { value: "dedicated", label: "Dedicated" },
              ]}
              value={modes[svc.id]}
              onChange={(v) => setModes((prev) => ({ ...prev, [svc.id]: v }))}
            />
          </div>
        ))}
      </div>

      <InfoCallout>
        Switching a service between shared and dedicated will migrate existing data. Back up your
        data before switching in a project with important data.
      </InfoCallout>

      {/* Active instances table */}
      <div>
        <SectionHeader title="Active instances" />
        <div className="overflow-hidden rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="border-b border-border bg-muted/40">
              <tr>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Instance
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Mode
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Port
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Used by
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Status
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {ACTIVE_INSTANCES.map((inst) => (
                <tr key={inst.name}>
                  <td className="px-4 py-2.5 font-medium">{inst.name}</td>
                  <td className="px-4 py-2.5">
                    <Badge variant="secondary" className="text-xs capitalize">
                      {inst.mode}
                    </Badge>
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs">{inst.port}</td>
                  <td className="px-4 py-2.5 text-muted-foreground">{inst.usedBy}</td>
                  <td className="px-4 py-2.5">
                    <span className="inline-flex items-center gap-1.5 text-xs text-success">
                      <span className="size-1.5 rounded-full bg-success" />
                      Running
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <WarnCallout>
        A shared instance failure affects all projects using it. For production-critical work,
        prefer dedicated instances.
      </WarnCallout>

      {/* Shared PostgreSQL config */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">Shared PostgreSQL 17 configuration</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs">Max connections</Label>
              <Input defaultValue="100" className="font-mono text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">Shared memory</Label>
              <Input defaultValue="128MB" className="font-mono text-xs" />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">Extensions</Label>
            <div className="flex flex-wrap gap-1.5">
              {["uuid-ossp", "pgcrypto", "pg_trgm"].map((ext) => (
                <Badge key={ext} variant="outline" className="font-mono text-xs">
                  {ext}
                </Badge>
              ))}
              <button className="text-xs text-primary hover:underline">+ Add</button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Lifecycle */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">Lifecycle</CardTitle>
        </CardHeader>
        <CardContent className="divide-y divide-border">
          <SwitchRow
            label="Start on demand"
            description="Start a service when a project that needs it launches"
            checked={lifecycle.startOnDemand}
            onChange={(v) => setLifecycle((p) => ({ ...p, startOnDemand: v }))}
          />
          <SwitchRow
            label="Stop idle services"
            description="Shut down shared instances not used by any running project"
            checked={lifecycle.stopIdle}
            onChange={(v) => setLifecycle((p) => ({ ...p, stopIdle: v }))}
          />
          <SwitchRow
            label="Restart crashed services"
            description="Automatically restart a service if it exits unexpectedly"
            checked={lifecycle.restartCrashed}
            onChange={(v) => setLifecycle((p) => ({ ...p, restartCrashed: v }))}
          />
          <SwitchRow
            label="Back up before switching mode"
            description="Create a backup snapshot before changing shared / dedicated"
            checked={lifecycle.backupBeforeSwitching}
            onChange={(v) => setLifecycle((p) => ({ ...p, backupBeforeSwitching: v }))}
          />
        </CardContent>
      </Card>

      {/* Dedicated instance port range */}
      <div>
        <SectionHeader
          title="Dedicated instance ports"
          description="Port range reserved for dedicated service instances"
        />
        <div className="flex items-center gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs">From</Label>
            <Input defaultValue="15000" className="w-28 font-mono text-xs" />
          </div>
          <span className="mt-5 text-muted-foreground">–</span>
          <div className="space-y-1.5">
            <Label className="text-xs">To</Label>
            <Input defaultValue="15999" className="w-28 font-mono text-xs" />
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Tab: Runtimes ────────────────────────────────────────────────────────────


function RuntimesTab() {
  const catalog = useRuntimeCatalog();
  const entries = catalog.data ?? [];
  const copies = entries.reduce((count, entry) => count + (entry.copies?.length ?? 0), 0);
  const goDownloads = () => {
    window.location.hash = "#/downloads";
  };

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold">Language runtimes</h2>
          <p className="text-xs text-muted-foreground">
            {catalog.isPending
              ? "Reading the machine…"
              : `${copies} installed ${copies === 1 ? "copy" : "copies"} across ${entries.length} runtime kinds, read by Locafy`}
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={goDownloads}>
          Install a runtime
        </Button>
      </div>

      {catalog.isError && (
        <p role="alert" className="text-sm text-destructive">
          Could not read the runtimes: {String(catalog.error)}
        </p>
      )}

      {/* Runtime rows: one per kind, copies managed first, newest after */}
      <div className="divide-y divide-border rounded-lg border border-border">
        {entries.map((entry) => (
          <div key={entry.kind.id} className="flex flex-wrap items-start gap-4 px-4 py-3">
            <div className="min-w-[80px]">
              <p className="text-sm font-medium">{entry.kind.label}</p>
              {entry.copies && entry.copies.length > 0 ? (
                <p className="text-xs text-muted-foreground">
                  {entry.copies.length === 1 ? "1 copy" : `${entry.copies.length} copies`}
                </p>
              ) : (
                <p className="text-xs text-muted-foreground">Not found</p>
              )}
            </div>
            <div className="min-w-0 flex-1 space-y-1">
              {entry.copies && entry.copies.length > 0 ? (
                entry.copies.map((copy) => (
                  <div key={copy.id} className="flex flex-wrap items-center gap-2">
                    <Badge variant="outline" className="font-mono text-xs">
                      {copy.version || "unknown version"}
                    </Badge>
                    {copy.source === "managed" && <Badge className="text-xs">Locafy-managed</Badge>}
                    {copy.source === "PATH" && <Badge variant="outline" className="text-xs">PATH</Badge>}
                    <span
                      className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground"
                      title={copy.path}
                    >
                      {copy.path}
                    </span>
                  </div>
                ))
              ) : (
                <p className="text-xs text-muted-foreground">
                  No {entry.kind.label} executable on this machine.{" "}
                  <button className="text-primary hover:underline" onClick={goDownloads}>
                    Install from Downloads
                  </button>
                </p>
              )}
            </div>
          </div>
        ))}
      </div>

      <p className="text-xs text-muted-foreground">
        Reading runtimes never changes the machine. An application can pin one of these copies from
        its card in the project's Applications tab; a Locafy-managed copy installs from the
        Downloads screen and is preferred in the picker.
      </p>
    </div>
  );
}

// ─── Tab: Domains & HTTPS ─────────────────────────────────────────────────────

const DOMAIN_ROWS = [
  { domain: "api.etugora.test", port: 8000, https: true, project: "Etugora", status: "running" },
  { domain: "auth.etugora.test", port: 3001, https: true, project: "Etugora", status: "running" },
  { domain: "ml.etugora.test", port: 5000, https: false, project: "Etugora", status: "running" },
  { domain: "app.folio.test", port: 4000, https: true, project: "Folio", status: "stopped" },
];

function DomainsTab() {
  const [suffix, setSuffix] = useState(".test");
  const [resolver, setResolver] = useState("hosts");
  const [httpsToggles, setHttpsToggles] = useState({
    autoCreate: true,
    redirectHttp: true,
    oneCertPerProject: false,
  });

  return (
    <div className="space-y-8">
      {/* Local domains */}
      <div>
        <SectionHeader
          title="Local domains"
          description="The suffix appended to your app and project names to form local hostnames"
        />
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Domain suffix</Label>
            <ToggleGroup
              options={[
                { value: ".test", label: ".test" },
                { value: ".localhost", label: ".localhost" },
                { value: ".internal", label: ".internal" },
              ]}
              value={suffix}
              onChange={setSuffix}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">How domains resolve</Label>
            <ToggleGroup
              options={[
                { value: "hosts", label: "Hosts file" },
                { value: "dns", label: "Local DNS" },
              ]}
              value={resolver}
              onChange={setResolver}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Address pattern</Label>
            <Input
              value="{app}.{project}.test"
              readOnly
              className="max-w-xs font-mono text-xs text-muted-foreground"
            />
          </div>
        </div>
      </div>

      {/* HTTPS */}
      <div>
        <SectionHeader title="HTTPS" />
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-3">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-success/10 px-3 py-1 text-xs font-medium text-success">
              <Check className="size-3" />
              Certificate authority trusted
            </span>
            <Button size="sm" variant="outline" className="text-xs">
              Reinstall CA
            </Button>
            <Button size="sm" variant="outline" className="text-xs">
              Export certificate
            </Button>
          </div>
          <div className="divide-y divide-border rounded-lg border border-border">
            <div className="px-4">
              <SwitchRow
                label="Create domain for every new app"
                description={`Automatically generate a {app}.{project}${suffix} domain when adding an app`}
                checked={httpsToggles.autoCreate}
                onChange={(v) => setHttpsToggles((p) => ({ ...p, autoCreate: v }))}
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="Redirect HTTP to HTTPS"
                description="All HTTP requests on port 80 are upgraded automatically"
                checked={httpsToggles.redirectHttp}
                onChange={(v) => setHttpsToggles((p) => ({ ...p, redirectHttp: v }))}
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="One certificate per project"
                description="Issues a wildcard cert per project instead of per app"
                checked={httpsToggles.oneCertPerProject}
                onChange={(v) => setHttpsToggles((p) => ({ ...p, oneCertPerProject: v }))}
              />
            </div>
          </div>
          <InfoCallout>
            Firefox uses its own certificate store. After installing the CA, visit{" "}
            <strong>about:config</strong> and enable{" "}
            <code className="rounded bg-muted/60 px-1">security.enterprise_roots.enabled</code> to
            trust it in Firefox.
          </InfoCallout>
        </div>
      </div>

      {/* Domains in use */}
      <div>
        <div className="mb-4 flex items-center justify-between">
          <SectionHeader title="Domains in use" />
          <Button size="sm" variant="outline" className="text-xs">
            + Add domain
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
                  Port
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  HTTPS
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Project
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Status
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {DOMAIN_ROWS.map((row) => (
                <tr key={row.domain}>
                  <td className="px-4 py-2.5 font-mono text-xs">{row.domain}</td>
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
                  <td className="px-4 py-2.5 text-muted-foreground">{row.project}</td>
                  <td className="px-4 py-2.5">
                    <span
                      className={cn(
                        "inline-flex items-center gap-1.5 text-xs",
                        row.status === "running" ? "text-success" : "text-muted-foreground",
                      )}
                    >
                      <span
                        className={cn(
                          "size-1.5 rounded-full",
                          row.status === "running" ? "bg-success" : "bg-muted-foreground",
                        )}
                      />
                      {row.status === "running" ? "Running" : "Stopped"}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Reverse proxy */}
      <div>
        <SectionHeader
          title="Reverse proxy"
          description="Ports Locafy's built-in proxy listens on"
        />
        <div className="flex items-center gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs">HTTP port</Label>
            <Input defaultValue="80" className="w-24 font-mono text-xs" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">HTTPS port</Label>
            <Input defaultValue="443" className="w-24 font-mono text-xs" />
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Tab: Tunnels (PLANNED) ───────────────────────────────────────────────────

function TunnelsTab() {
  return (
    <div className="relative">
      <PlannedOverlay />
      {/* Blurred/disabled content to show layout */}
      <div className="pointer-events-none select-none space-y-8 opacity-30 blur-sm">
        <div>
          <SectionHeader title="Tunnel provider" />
          <div className="space-y-3">
            <ToggleGroup
              options={[
                { value: "cloudflare", label: "Cloudflare Tunnel" },
                { value: "ngrok", label: "ngrok" },
                { value: "locafy", label: "Locafy Cloud" },
              ]}
              value="locafy"
              onChange={() => {}}
              disabled
            />
          </div>
        </div>
        <div>
          <SectionHeader title="Public address" />
          <Input value="https://your-project.locafy.dev" readOnly className="font-mono text-xs" />
        </div>
        <div>
          <SectionHeader title="Active tunnels" />
          <div className="divide-y divide-border rounded-lg border border-border">
            <div className="flex items-center gap-4 px-4 py-3">
              <span className="text-sm">api.etugora.locafy.dev</span>
              <Badge variant="secondary">Planned</Badge>
            </div>
          </div>
        </div>
        <div>
          <SectionHeader title="Safety defaults" />
          <div className="divide-y divide-border rounded-lg border border-border">
            <div className="px-4">
              <SwitchRow
                label="Require authentication for all tunnels"
                checked={false}
                onChange={() => {}}
                disabled
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="Stop tunnels when project stops"
                checked={true}
                onChange={() => {}}
                disabled
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Tab: Network ─────────────────────────────────────────────────────────────

function NetworkTab() {
  const [access, setAccess] = useState("local");
  const [portToggles, setPortToggles] = useState({
    warnPortInUse: true,
    keepPortsStopped: false,
    showQrCode: false,
  });

  return (
    <div className="space-y-8">
      <div>
        <SectionHeader
          title="Who can reach your projects"
          description="Controls which network interface Locafy binds to"
        />
        <div className="space-y-3">
          <ToggleGroup
            options={[
              { value: "local", label: "This computer only" },
              { value: "network", label: "Local network" },
            ]}
            value={access}
            onChange={setAccess}
          />
          <p className="text-xs text-muted-foreground">
            {access === "local"
              ? "Apps bind to 127.0.0.1 — only reachable from this computer."
              : "Apps bind to 0.0.0.0 — reachable from other devices on your local network."}
          </p>
        </div>
      </div>

      <div>
        <SectionHeader title="Ports" description="Port ranges Locafy assigns automatically" />
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <div className="space-y-1.5">
              <Label className="text-xs">App ports start at</Label>
              <Input defaultValue="8000" className="font-mono text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">Dedicated services from</Label>
              <Input defaultValue="15000" className="font-mono text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">Dedicated services to</Label>
              <Input defaultValue="15999" className="font-mono text-xs" />
            </div>
          </div>
          <div className="divide-y divide-border rounded-lg border border-border">
            <div className="px-4">
              <SwitchRow
                label="Warn when port is already in use"
                description="Show an alert before starting if the assigned port is taken"
                checked={portToggles.warnPortInUse}
                onChange={(v) => setPortToggles((p) => ({ ...p, warnPortInUse: v }))}
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="Keep ports for stopped projects"
                description="Reserve the same port when a project is stopped so it restarts on the same address"
                checked={portToggles.keepPortsStopped}
                onChange={(v) => setPortToggles((p) => ({ ...p, keepPortsStopped: v }))}
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="Show QR code for local network access"
                description="Only available when local network mode is on"
                checked={portToggles.showQrCode}
                onChange={(v) => setPortToggles((p) => ({ ...p, showQrCode: v }))}
                disabled={access !== "network"}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Tab: Storage ─────────────────────────────────────────────────────────────

const STORAGE_SEGMENTS = [
  { label: "Runtimes", size: "3.2 GB", color: "bg-primary", width: "w-[57%]" },
  { label: "Service data", size: "1.4 GB", color: "bg-blue-400", width: "w-[25%]" },
  { label: "Backups", size: "0.9 GB", color: "bg-amber-400", width: "w-[16%]" },
  { label: "Logs", size: "0.1 GB", color: "bg-success", width: "w-[2%]" },
];

const FOLDER_ROWS = [
  { name: "Projects", path: "D:/Projects", size: null },
  { name: "Runtimes", path: "~/.locafy/runtimes", size: "3.2 GB" },
  { name: "Service data", path: "~/.locafy/data", size: "1.4 GB" },
  { name: "Logs", path: "~/.locafy/logs", size: "0.1 GB" },
];

function StorageTab() {
  const [keepBackups, setKeepBackups] = useState("7");
  const [backupToggles, setBackupToggles] = useState({
    backupServiceData: true,
    compressBackups: true,
  });

  return (
    <div className="space-y-8">
      {/* Disk usage */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <SectionHeader title="Disk usage" />
          <span className="text-xs font-medium text-foreground">5.6 GB used by Locafy</span>
        </div>
        <div className="space-y-3">
          <div className="flex h-3 overflow-hidden rounded-full bg-muted">
            {STORAGE_SEGMENTS.map((seg) => (
              <div key={seg.label} className={cn("h-full", seg.color, seg.width)} />
            ))}
          </div>
          <div className="flex flex-wrap gap-4">
            {STORAGE_SEGMENTS.map((seg) => (
              <div key={seg.label} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span className={cn("size-2 rounded-full", seg.color)} />
                {seg.label}
                <span className="font-medium text-foreground">{seg.size}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Folders */}
      <div>
        <SectionHeader title="Folders" />
        <div className="overflow-hidden rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="border-b border-border bg-muted/40">
              <tr>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Name
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Location
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">
                  Size
                </th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {FOLDER_ROWS.map((row) => (
                <tr key={row.name}>
                  <td className="px-4 py-2.5 font-medium">{row.name}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">
                    {row.path}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-muted-foreground">
                    {row.size ?? "—"}
                  </td>
                  <td className="px-4 py-2.5">
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant="ghost" className="h-7 px-2 text-xs">
                        Open
                      </Button>
                      <Button size="sm" variant="ghost" className="h-7 px-2 text-xs">
                        Change
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Backups and cleanup */}
      <div>
        <SectionHeader title="Backups and cleanup" />
        <div className="space-y-4">
          <div className="divide-y divide-border rounded-lg border border-border">
            <div className="px-4">
              <SwitchRow
                label="Back up service data"
                description="Snapshot database and service data before destructive operations"
                checked={backupToggles.backupServiceData}
                onChange={(v) => setBackupToggles((p) => ({ ...p, backupServiceData: v }))}
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="Compress backups"
                description="Reduces backup size by about 60-80% with minimal CPU overhead"
                checked={backupToggles.compressBackups}
                onChange={(v) => setBackupToggles((p) => ({ ...p, compressBackups: v }))}
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Keep backups for</Label>
            <ToggleGroup
              options={[
                { value: "3", label: "3 days" },
                { value: "7", label: "7 days" },
                { value: "14", label: "14 days" },
                { value: "30", label: "30 days" },
              ]}
              value={keepBackups}
              onChange={setKeepBackups}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" className="text-xs">
              <Trash2 className="mr-1.5 size-3.5" />
              Remove unused runtimes
            </Button>
            <Button variant="outline" size="sm" className="text-xs">
              <Trash2 className="mr-1.5 size-3.5" />
              Clear logs
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Tab: Appearance ──────────────────────────────────────────────────────────

function AppearanceTab() {
  // Detect current theme from document
  const [theme, setTheme] = useState<"dark" | "light" | "system">(() =>
    document.documentElement.classList.contains("dark") ? "dark" : "light",
  );
  const [accent, setAccent] = useState("violet");
  const [density, setDensity] = useState("comfortable");
  const [textSize, setTextSize] = useState("default");
  const [behavior, setBehavior] = useState({
    reduceMotion: false,
    showResourceCharts: true,
  });

  const applyTheme = (t: "dark" | "light" | "system") => {
    setTheme(t);
    const effective = t === "system"
      ? window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : t;
    document.documentElement.classList.toggle("dark", effective === "dark");
  };

  const ACCENT_OPTIONS = [
    { id: "violet", color: "bg-[#8B3DFF]", label: "Electric Violet" },
    { id: "blue", color: "bg-blue-500", label: "Blue" },
    { id: "emerald", color: "bg-emerald-500", label: "Emerald" },
    { id: "amber", color: "bg-amber-500", label: "Amber" },
    { id: "rose", color: "bg-rose-500", label: "Rose" },
  ];

  const THEME_CARDS = [
    { id: "dark", label: "Dark", preview: "bg-[#111116]", text: "text-[#F5F3F8]" },
    { id: "light", label: "Light", preview: "bg-[#F5F3F8]", text: "text-[#111116]" },
    { id: "system", label: "System", preview: "bg-gradient-to-br from-[#111116] to-[#F5F3F8]", text: "text-foreground" },
  ] as const;

  return (
    <div className="space-y-8">
      {/* Theme */}
      <div>
        <SectionHeader title="Theme" />
        <div className="flex gap-3">
          {THEME_CARDS.map((card) => (
            <button
              key={card.id}
              type="button"
              onClick={() => applyTheme(card.id as typeof theme)}
              className={cn(
                "group relative overflow-hidden rounded-xl border-2 p-0 transition-all",
                theme === card.id ? "border-primary shadow-sm shadow-primary/20" : "border-border hover:border-muted-foreground/50",
              )}
            >
              <div className={cn("flex h-24 w-28 items-center justify-center", card.preview)}>
                <div className="space-y-1.5">
                  <div className="mx-auto h-1.5 w-12 rounded-full bg-current opacity-30" />
                  <div className="mx-auto h-1.5 w-8 rounded-full bg-current opacity-20" />
                  <div className="mx-auto h-1.5 w-10 rounded-full bg-current opacity-20" />
                </div>
              </div>
              <div className="border-t border-border bg-card px-2 py-1.5 text-center">
                <span className="text-xs font-medium">{card.label}</span>
              </div>
              {theme === card.id && (
                <span className="absolute right-1.5 top-1.5 flex size-4 items-center justify-center rounded-full bg-primary">
                  <Check className="size-2.5 text-white" />
                </span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Colors and density */}
      <div>
        <SectionHeader title="Colors and density" />
        <div className="space-y-4">
          <div className="space-y-2">
            <Label className="text-xs text-muted-foreground">Accent color</Label>
            <div className="flex gap-2">
              {ACCENT_OPTIONS.map((opt) => (
                <button
                  key={opt.id}
                  type="button"
                  title={opt.label}
                  onClick={() => setAccent(opt.id)}
                  className={cn(
                    "size-7 rounded-full transition-all",
                    opt.color,
                    accent === opt.id ? "ring-2 ring-offset-2 ring-offset-background ring-primary" : "opacity-70 hover:opacity-100",
                  )}
                />
              ))}
            </div>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Density</Label>
            <ToggleGroup
              options={[
                { value: "comfortable", label: "Comfortable" },
                { value: "compact", label: "Compact" },
              ]}
              value={density}
              onChange={setDensity}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Text size</Label>
            <ToggleGroup
              options={[
                { value: "small", label: "Small" },
                { value: "default", label: "Default" },
                { value: "large", label: "Large" },
              ]}
              value={textSize}
              onChange={setTextSize}
            />
          </div>
        </div>
      </div>

      {/* Interface behavior */}
      <div>
        <SectionHeader title="Interface behavior" />
        <div className="divide-y divide-border rounded-lg border border-border">
          <div className="px-4">
            <SwitchRow
              label="Reduce motion"
              description="Minimizes animations for accessibility or preference"
              checked={behavior.reduceMotion}
              onChange={(v) => setBehavior((p) => ({ ...p, reduceMotion: v }))}
            />
          </div>
          <div className="px-4">
            <SwitchRow
              label="Show resource charts on project pages"
              description="CPU, memory and disk mini-charts in project overviews"
              checked={behavior.showResourceCharts}
              onChange={(v) => setBehavior((p) => ({ ...p, showResourceCharts: v }))}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Tab: Updates ─────────────────────────────────────────────────────────────

const RECENT_RELEASES = [
  {
    version: "0.9.2",
    label: "Current",
    date: "Nov 14, 2026",
    summary: "Fixed project linking on Windows with paths containing spaces. Improved startup time by 40%.",
  },
  {
    version: "0.9.1",
    label: "Previous",
    date: "Nov 1, 2026",
    summary: "Added Runtimes screen, improved domain routing, fixed Redis reconnect loop.",
  },
  {
    version: "0.9.0",
    label: "Earlier",
    date: "Oct 15, 2026",
    summary: "Introduced the Projects dashboard, service management, and dark-mode overhaul.",
  },
];

function UpdatesTab() {
  const [channel, setChannel] = useState("stable");
  const [updateToggles, setUpdateToggles] = useState({
    checkAutomatically: true,
    includeRuntimeUpdates: true,
    notifyWhenReady: true,
  });

  return (
    <div className="space-y-8">
      {/* Current version card */}
      <Card>
        <CardContent className="flex items-center gap-4 pt-5">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary">
            <Zap className="size-6" />
          </div>
          <div className="flex-1">
            <p className="font-semibold">Locafy 0.9.2</p>
            <p className="text-xs text-muted-foreground">Last checked: Today at 09:14</p>
          </div>
          <Button variant="outline" size="sm">
            <RefreshCw className="mr-1.5 size-3.5" />
            Check for updates
          </Button>
        </CardContent>
      </Card>

      {/* Update available banner */}
      <div className="rounded-lg border border-primary/30 bg-primary/5 p-4">
        <div className="flex items-start gap-3">
          <Download className="mt-0.5 size-4 shrink-0 text-primary" />
          <div className="flex-1 space-y-1">
            <p className="text-sm font-medium">Locafy 0.9.3 is available</p>
            <p className="text-xs text-muted-foreground">
              Fixes a memory leak in the service watcher and improves PHP 8.4 compatibility.
            </p>
          </div>
          <div className="flex shrink-0 gap-2">
            <Button variant="ghost" size="sm" className="text-xs">
              View release notes
            </Button>
            <Button size="sm" className="text-xs">
              Install and restart
            </Button>
          </div>
        </div>
      </div>

      {/* Update preferences */}
      <div>
        <SectionHeader title="Update preferences" />
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Release channel</Label>
            <ToggleGroup
              options={[
                { value: "stable", label: "Stable" },
                { value: "beta", label: "Beta" },
              ]}
              value={channel}
              onChange={setChannel}
            />
          </div>
          <div className="divide-y divide-border rounded-lg border border-border">
            <div className="px-4">
              <SwitchRow
                label="Check for updates automatically"
                description="Checks once a day in the background"
                checked={updateToggles.checkAutomatically}
                onChange={(v) => setUpdateToggles((p) => ({ ...p, checkAutomatically: v }))}
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="Include runtime and service updates"
                description="Notify when new PHP, Node.js, Python versions or service images are available"
                checked={updateToggles.includeRuntimeUpdates}
                onChange={(v) => setUpdateToggles((p) => ({ ...p, includeRuntimeUpdates: v }))}
              />
            </div>
            <div className="px-4">
              <SwitchRow
                label="Notify me when update is ready"
                description="Show a banner when a downloaded update is waiting to install"
                checked={updateToggles.notifyWhenReady}
                onChange={(v) => setUpdateToggles((p) => ({ ...p, notifyWhenReady: v }))}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Recent releases */}
      <div>
        <SectionHeader title="Recent releases" />
        <div className="divide-y divide-border rounded-lg border border-border">
          {RECENT_RELEASES.map((rel) => (
            <div key={rel.version} className="flex gap-4 px-4 py-3">
              <div className="min-w-[80px]">
                <p className="font-mono text-sm font-medium">{rel.version}</p>
                <Badge
                  variant={rel.label === "Current" ? "default" : "secondary"}
                  className="mt-1 text-xs"
                >
                  {rel.label}
                </Badge>
              </div>
              <div className="flex-1 space-y-0.5">
                <p className="text-xs text-muted-foreground">{rel.date}</p>
                <p className="text-xs">{rel.summary}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ─── Main SettingsScreen ──────────────────────────────────────────────────────

export function SettingsScreen() {
  const [activeTab, setActiveTab] = useState<SettingsTab>("general");

  const TAB_COMPONENTS: Record<SettingsTab, React.ReactNode> = {
    general: <GeneralTab />,
    services: <ServicesTab />,
    runtimes: <RuntimesTab />,
    domains: <DomainsTab />,
    tunnels: <TunnelsTab />,
    network: <NetworkTab />,
    storage: <StorageTab />,
    appearance: <AppearanceTab />,
    updates: <UpdatesTab />,
  };

  return (
    <div className="flex min-h-full flex-col">
      {/* Page header */}
      <div className="flex items-center justify-between border-b border-border px-6 py-4">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Settings</h1>
          <p className="text-xs text-muted-foreground">Configure Locafy's behaviour and defaults</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm">
            <RotateCcw className="mr-1.5 size-3.5" />
            Reset to defaults
          </Button>
          <Button size="sm">
            <Save className="mr-1.5 size-3.5" />
            Save changes
          </Button>
        </div>
      </div>

      {/* Two-column layout */}
      <div className="flex flex-1 overflow-hidden">
        {/* Sidebar nav */}
        <nav className="w-52 shrink-0 border-r border-border py-3">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = activeTab === item.id;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setActiveTab(item.id)}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-none px-4 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-primary/10 text-primary font-medium"
                    : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                )}
              >
                <Icon className="size-4 shrink-0" />
                {item.label}
              </button>
            );
          })}
        </nav>

        {/* Tab content */}
        <main className="flex-1 overflow-y-auto px-8 py-6">
          {TAB_COMPONENTS[activeTab]}
        </main>
      </div>
    </div>
  );
}
