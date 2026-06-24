export const stripCatalogVersionPrefix = (version: string) => String(version || '').replace(/^v/i, '')

export const semanticVersionParts = (version: string) =>
  stripCatalogVersionPrefix(version)
    .split(/[.-]/)
    .map(part => Number.parseInt(part, 10))
    .map(part => (Number.isFinite(part) ? part : 0))

export const compareCatalogVersions = (left: string, right: string) => {
  const leftParts = semanticVersionParts(left)
  const rightParts = semanticVersionParts(right)
  const partCount = Math.max(leftParts.length, rightParts.length, 3)

  for (let i = 0; i < partCount; i += 1) {
    const diff = (rightParts[i] || 0) - (leftParts[i] || 0)
    if (diff !== 0) return diff
  }

  return String(right || '').localeCompare(String(left || ''))
}
