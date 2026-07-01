export type RegistryImageTagSearchOption = {
  imageTag?: string
}

const normalizeSearchText = (value: unknown) => String(value || '').trim().toLowerCase()

const imageTagMatchesSearch = (value: string, searchText: string) => {
  if (!searchText) return true
  return normalizeSearchText(value).includes(searchText)
}

export const mergeRegistryImageTagOptions = (
  searchOptions: RegistryImageTagSearchOption[],
  workspaceOptions: string[],
  query: unknown,
) => {
  const searchText = normalizeSearchText(query)
  const seen = new Set<string>()
  const options: string[] = []

  for (const value of [
    ...searchOptions.map(item => item.imageTag || ''),
    ...workspaceOptions,
  ]) {
    const normalized = String(value || '').trim()
    if (!normalized || seen.has(normalized)) continue
    if (!imageTagMatchesSearch(normalized, searchText)) continue
    seen.add(normalized)
    options.push(normalized)
  }

  return options
}
