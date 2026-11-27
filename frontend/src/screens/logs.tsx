import { LiveLogs } from "@/components/live-logs";

/*
 * Machine-wide output: every application and service Locafy has started, with
 * the source named on each line. Project-scoped output is the same stream
 * filtered in the project's Logs tab.
 */
export function LogsScreen() {
  return (
    <div className="flex min-h-full flex-col">
      <div className="border-b border-border px-6 py-5">
        <h1 className="text-lg font-semibold tracking-tight">Logs</h1>
        <p className="mt-0.5 text-xs text-muted-foreground">
          Output from every application and service Locafy started in this session, with its source
          named on each line. A project's own lines are in that project's Logs tab.
        </p>
      </div>
      <LiveLogs />
    </div>
  );
}
