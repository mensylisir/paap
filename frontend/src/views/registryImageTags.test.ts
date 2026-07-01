import { describe, expect, it } from 'vitest'
import { mergeRegistryImageTagOptions } from './registryImageTags'

describe('registry image tag suggestions', () => {
  it('filters workspace suggestions by the current image search text', () => {
    const result = mergeRegistryImageTagOptions(
      [{ imageTag: 'paap-real-frontend:v2.0.1' }],
      ['paap-real-backend:v2.0.1', 'paap-real-frontend:v2.0.1'],
      'front',
    )

    expect(result).toEqual(['paap-real-frontend:v2.0.1'])
  })

  it('shows all unique registry suggestions when no search text is entered', () => {
    const result = mergeRegistryImageTagOptions(
      [{ imageTag: 'paap-real-backend:v2.0.1' }],
      ['paap-real-backend:v2.0.1', 'paap-real-frontend:v2.0.1'],
      '',
    )

    expect(result).toEqual([
      'paap-real-backend:v2.0.1',
      'paap-real-frontend:v2.0.1',
    ])
  })
})
