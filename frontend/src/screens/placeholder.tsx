export function Placeholder({ name, planned }: { name: string; planned?: boolean }) {
  return (
    <div className="mx-auto max-w-md px-6 py-24 text-center">
      <h1 className="text-lg font-medium">{name}</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        {planned
          ? "Planned for a later phase. Nothing here is available yet."
          : "This screen is not built yet."}
      </p>
    </div>
  );
}
