import { useEffect, useState } from "react";
import * as SettingsService from "../bindings/locafy.dev/desktop/internal/settings/service";
import type { Setting } from "../bindings/locafy.dev/desktop/internal/settings/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

function App() {
  const [settings, setSettings] = useState<Setting[]>([]);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);

  const refresh = () => {
    SettingsService.All()
      .then((result) => setSettings(result ?? []))
      .catch((err) => setError(String(err)));
  };

  useEffect(refresh, []);

  const addSetting = () => {
    if (!key.trim() || !value.trim()) return;
    SettingsService.SetString(key.trim(), value.trim())
      .then(() => {
        setKey("");
        setValue("");
        refresh();
      })
      .catch((err) => setError(String(err)));
  };

  return (
    <main className="mx-auto min-h-screen w-full max-w-2xl px-6 py-12">
      <header className="mb-8 space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Locafy</h1>
        <p className="text-sm text-muted-foreground">
          Local-first development environment control plane
        </p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Stored settings</CardTitle>
          <CardDescription>Persisted in SQLite</CardDescription>
        </CardHeader>
        <CardContent>
          {settings.length === 0 ? (
            <p className="py-2 text-sm text-muted-foreground">No settings stored yet.</p>
          ) : (
            <ul className="divide-y divide-border">
              {settings.map((setting) => (
                <li
                  key={setting.Key}
                  className="flex items-center gap-4 py-2.5"
                >
                  <span className="font-mono text-sm text-accent">{setting.Key}</span>
                  <span className="min-w-0 flex-1 truncate text-right text-sm">
                    {setting.Value}
                  </span>
                  <Badge variant="secondary">{setting.Type}</Badge>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <form
        className="mt-6 flex flex-col gap-2 sm:flex-row"
        onSubmit={(event) => {
          event.preventDefault();
          addSetting();
        }}
      >
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
        <Button type="submit">Save</Button>
      </form>

      {error && (
        <p role="alert" className="mt-4 text-sm text-destructive">
          {error}
        </p>
      )}
    </main>
  );
}

export default App;
