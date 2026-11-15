import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as ProjectsService from "../../bindings/locafy.dev/desktop/internal/projects/service";
import * as SettingsService from "../../bindings/locafy.dev/desktop/internal/settings/service";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

const settingsKey = ["settings"] as const;
const workspaceKey = ["projects", "workspace"] as const;

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
