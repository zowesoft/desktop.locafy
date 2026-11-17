import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import * as SettingsService from "../../bindings/locafy.dev/desktop/internal/settings/service";
import * as StatusService from "../../bindings/locafy.dev/desktop/internal/status/service";
import type { NotificationPreferences } from "../../bindings/locafy.dev/desktop/internal/status/models";
import { notificationPreferencesKey } from "@/components/top-bar-status";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

const settingsKey = ["settings"] as const;
const workspaceKey = ["projects", "workspace"] as const;
const preferencesKey = notificationPreferencesKey;

/*
 * The kinds of notification the bell can show. Each is a separate choice so an
 * operator who does not care about project churn still sees machine warnings.
 */
const notificationTypes: {
  key: keyof NotificationPreferences;
  label: string;
  description: string;
}[] = [
  {
    key: "Warnings",
    label: "Warnings",
    description: "Conditions that need acting on, such as low disk space",
  },
  {
    key: "Project",
    label: "Project activity",
    description: "Projects created, linked, renamed, re-pointed or removed",
  },
  {
    key: "Machine",
    label: "Machine events",
    description: "Engine starts and other machine-level activity",
  },
];

export function SettingsScreen() {
  const queryClient = useQueryClient();
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [workspaceDraft, setWorkspaceDraft] = useState("");

  const workspace = useQuery({
    queryKey: workspaceKey,
    queryFn: () => ProjectsService.WorkspaceDir(),
  });

  // The field shows the effective location, so it fills in once resolved and
  // again after a save or reset returns the new answer.
  useEffect(() => {
    if (workspace.data !== undefined) setWorkspaceDraft(workspace.data);
  }, [workspace.data]);

  const saveWorkspace = useMutation({
    mutationFn: (path: string) => ProjectsService.SetWorkspaceDir(path),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: workspaceKey }),
  });

  const settings = useQuery({
    queryKey: settingsKey,
    queryFn: async () => (await SettingsService.All()) ?? [],
  });

  const preferences = useQuery({
    queryKey: preferencesKey,
    queryFn: () => StatusService.NotificationPreferences(),
  });

  // The bell's cached list is filtered on the server, so changing a type has to
  // refetch it rather than only refresh this screen.
  const savePreferences = useMutation({
    mutationFn: (prefs: NotificationPreferences) =>
      StatusService.SetNotificationPreferences(prefs),
    // The checkboxes read this cache, so writing the choice before the round
    // trip both makes the box respond immediately and keeps a quick second
    // toggle from sending preferences built on the previous answer.
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

  const saveSetting = useMutation({
    mutationFn: (setting: { key: string; value: string }) =>
      SettingsService.SetString(setting.key, setting.value),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: settingsKey }),
  });

  const canSubmit = key.trim() !== "" && value.trim() !== "" && !saveSetting.isPending;
  const rows = settings.data ?? [];
  const error = settings.error ?? saveSetting.error;

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    saveSetting.mutate(
      { key: key.trim(), value: value.trim() },
      {
        onSuccess: () => {
          setKey("");
          setValue("");
        },
      },
    );
  };

  return (
    <div className="mx-auto w-full max-w-2xl px-6 py-10">
      <header className="mb-8 space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground">
          Local-first development environment control plane
        </p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Projects workspace</CardTitle>
          <CardDescription>
            New Locafy-managed projects are created in this folder
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="flex flex-col gap-2 sm:flex-row"
            onSubmit={(event) => {
              event.preventDefault();
              if (workspaceDraft.trim() === "" || saveWorkspace.isPending) return;
              saveWorkspace.mutate(workspaceDraft.trim());
            }}
          >
            <Input
              aria-label="Workspace folder"
              className="font-mono text-xs"
              placeholder="C:\\locafy\\projects"
              value={workspaceDraft}
              onChange={(event) => setWorkspaceDraft(event.target.value)}
            />
            <Button
              type="submit"
              aria-label="Save workspace folder"
              disabled={
                workspaceDraft.trim() === "" ||
                workspaceDraft.trim() === workspace.data ||
                saveWorkspace.isPending
              }
            >
              {saveWorkspace.isPending ? "Saving…" : "Save"}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={saveWorkspace.isPending}
              onClick={() => saveWorkspace.mutate("")}
            >
              Use default
            </Button>
          </form>
          <p className="mt-2 text-xs text-muted-foreground">
            Projects already created keep their folders. Linked projects are always wherever you
            pointed them.
          </p>
          {saveWorkspace.error && (
            <p role="alert" className="mt-2 text-sm text-destructive">
              Could not save the workspace folder: {String(saveWorkspace.error)}
            </p>
          )}
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Notifications</CardTitle>
          <CardDescription>Which kinds of activity the bell shows</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {preferences.isPending ? (
            <p className="text-sm text-muted-foreground">Loading notification settings…</p>
          ) : preferences.isError ? (
            <p role="alert" className="text-sm text-destructive">
              Could not read notification settings: {String(preferences.error)}
            </p>
          ) : prefs ? (
            <>
              {notificationTypes.map((type) => (
              <label key={type.key} className="flex items-start gap-3">
                <input
                  type="checkbox"
                  className="mt-1 size-4 accent-primary"
                  aria-label={type.label}
                  checked={prefs[type.key]}
                  disabled={savePreferences.isPending}
                  onChange={(event) =>
                    savePreferences.mutate({ ...prefs, [type.key]: event.target.checked })
                  }
                />
                <span className="space-y-0.5">
                  <span className="block text-sm font-medium">{type.label}</span>
                  <span className="block text-xs text-muted-foreground">{type.description}</span>
                </span>
              </label>
              ))}
              {/*
               * Sound is not a type: it changes how arrivals are announced, so it
               * sits apart from the choices about which ones appear.
               */}
              <label className="flex items-start gap-3 border-t border-border pt-3">
                <input
                  type="checkbox"
                  className="mt-1 size-4 accent-primary"
                  aria-label="Play a sound"
                  checked={prefs.Sound}
                  disabled={savePreferences.isPending}
                  onChange={(event) =>
                    savePreferences.mutate({ ...prefs, Sound: event.target.checked })
                  }
                />
                <span className="space-y-0.5">
                  <span className="block text-sm font-medium">Play a sound</span>
                  <span className="block text-xs text-muted-foreground">
                    A short chime when new activity arrives while Locafy is running
                  </span>
                </span>
              </label>
            </>
          ) : null}
          <p className="text-xs text-muted-foreground">
            Turning everything off leaves the bell empty; recorded activity is still kept for
            diagnostics.
          </p>
          {savePreferences.error && (
            <p role="alert" className="text-sm text-destructive">
              Could not save notification settings: {String(savePreferences.error)}
            </p>
          )}
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Stored settings</CardTitle>
          <CardDescription>Persisted in SQLite</CardDescription>
        </CardHeader>
        <CardContent>
          {settings.isPending ? (
            <p className="py-2 text-sm text-muted-foreground">Loading settings…</p>
          ) : rows.length === 0 ? (
            <p className="py-2 text-sm text-muted-foreground">No settings stored yet.</p>
          ) : (
            <ul className="divide-y divide-border">
              {rows.map((setting) => (
                <li key={setting.Key} className="flex items-center gap-4 py-2.5">
                  <span className="font-mono text-sm text-accent">{setting.Key}</span>
                  <span className="min-w-0 flex-1 truncate text-right text-sm">{setting.Value}</span>
                  <Badge variant="secondary">{setting.Type}</Badge>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <form className="mt-6 flex flex-col gap-2 sm:flex-row" onSubmit={submit}>
        <Input
          aria-label="Setting key"
          placeholder="Key"
          value={key}
          onChange={(event) => setKey(event.target.value)}
        />
        <Input
          aria-label="Setting value"
          placeholder="Value"
          value={value}
          onChange={(event) => setValue(event.target.value)}
        />
        <Button type="submit" disabled={!canSubmit}>
          {saveSetting.isPending ? "Saving…" : "Save"}
        </Button>
      </form>

      {error && (
        <p role="alert" className="mt-4 text-sm text-destructive">
          {String(error)}
        </p>
      )}
    </div>
  );
}
