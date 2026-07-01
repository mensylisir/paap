import { describe, expect, it } from 'vitest'
import { catalogRouteForItem } from './catalogNavigation'

describe('catalog navigation helpers', () => {
  it('routes normal service products to their catalog detail page', () => {
    expect(catalogRouteForItem({ type: 'postgresql' })).toBe('/catalog/postgresql')
    expect(catalogRouteForItem({ type: 'kubevirt-postgresql', detailType: 'postgresql' })).toBe('/catalog/postgresql')
  })

  it('routes environment service catalog cards to their catalog detail page', () => {
    expect(catalogRouteForItem({ type: 'environment:1', category: 'environment' })).toBe('/catalog/environment%3A1')
    expect(catalogRouteForItem({ type: 'environment-template-1', catalogSource: 'environment-template' })).toBe('/catalog/environment-template-1')
  })
})
