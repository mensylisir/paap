type CanvasResource = {
  id?: string | number | null
  serviceType?: string
  type?: string
}

const resourceId = (item: CanvasResource | null | undefined) => {
  const id = item?.id
  return id === undefined || id === null || id === '' ? '' : String(id)
}

export function mergeCreatedCanvasResource<T extends CanvasResource>(items: T[], created: T | null | undefined): T[] {
  if (!created) return items
  const id = resourceId(created)
  if (id && items.some((item) => resourceId(item) === id)) return items
  return [...items, created]
}

export function selectCreatedCanvasResource<T extends CanvasResource>(
  items: T[],
  created: T | null | undefined,
  serviceType?: string,
): T | undefined {
  const id = resourceId(created)
  if (id) {
    const byId = items.find((item) => resourceId(item) === id)
    if (byId) return byId
  }
  const expectedType = String(serviceType || created?.serviceType || created?.type || '')
  return items.find((item) => String(item.serviceType || item.type || '') === expectedType) || created || undefined
}
