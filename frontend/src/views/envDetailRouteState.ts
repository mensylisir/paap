export function numericRouteParam(value: unknown): number {
  const raw = Array.isArray(value) ? value[0] : value
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : 0
}

export function routeEnvironmentKey(appId: number, envId: number): string {
  return `${appId}:${envId}`
}
