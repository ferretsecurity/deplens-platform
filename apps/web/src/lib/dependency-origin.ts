const originLabels = new Map([
  ["registry", "Registry"],
  ["git", "Git"],
  ["url", "URL"],
  ["path", "Local path"],
  ["workspace", "Workspace"],
  ["unknown", "Unknown"]
]);

export function dependencyOriginLabel(origin: string) {
  return originLabels.get(origin) ?? (origin || "Unknown");
}
