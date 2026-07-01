export interface CatalogNavigationItem {
  category?: unknown
  catalogSource?: unknown
  detailType?: unknown
  type?: unknown
}

export const catalogRouteForItem = (item: CatalogNavigationItem) => {
  const type = String(item.detailType || item.type || '').trim()
  return `/catalog/${encodeURIComponent(type)}`
}
