import { describe, expect, it } from 'vitest'
import { compareCatalogVersions, semanticVersionParts, stripCatalogVersionPrefix } from './catalogVersions'

describe('catalog version helpers', () => {
  it('sorts semantic versions newest first', () => {
    const versions = ['v1.2.2', 'v1.2.10', 'v2.0.0', 'v1.10.0']

    expect(versions.sort(compareCatalogVersions)).toEqual([
      'v2.0.0',
      'v1.10.0',
      'v1.2.10',
      'v1.2.2',
    ])
  })

  it('normalizes v-prefixed versions into numeric parts', () => {
    expect(stripCatalogVersionPrefix('v2024.1.11')).toBe('2024.1.11')
    expect(semanticVersionParts('v2024.1.11')).toEqual([2024, 1, 11])
  })
})
