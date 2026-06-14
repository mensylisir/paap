import { describe, expect, it } from 'vitest'
import {
  imageRefFromRegistryFields,
  imageTagForImageField,
  imageTagVersion,
  registryHostForImageField,
  registryRepositorySuffix,
  splitImageRepositoryAndTag,
} from './componentImageConfig'

describe('componentImageConfig', () => {
  it('splits the environment registry address from image:tag for drawer fields', () => {
    const image = 'registry.shop-dev.paap.local:5000/paap/orders-web:v1.2.3'

    expect(registryHostForImageField(image, 'registry.other.paap.local:5000')).toBe('registry.shop-dev.paap.local:5000')
    expect(imageTagForImageField(image)).toBe('paap/orders-web:v1.2.3')
  })

  it('builds the full image from registry host and image:tag fields', () => {
    expect(imageRefFromRegistryFields('registry.shop-dev.paap.local:5000', 'paap/orders-api:v2')).toBe(
      'registry.shop-dev.paap.local:5000/paap/orders-api:v2',
    )
    expect(imageTagVersion('paap/orders-api:v2')).toBe('v2')
  })

  it('keeps explicitly pasted full image refs intact', () => {
    expect(imageRefFromRegistryFields('registry.shop-dev.paap.local:5000', 'docker.io/library/nginx:1.25')).toBe(
      'docker.io/library/nginx:1.25',
    )
  })

  it('does not mistake a tag on a short image name for a registry host', () => {
    expect(imageRefFromRegistryFields('registry.shop-dev.paap.local:5000', 'nginx:1.25')).toBe(
      'registry.shop-dev.paap.local:5000/nginx:1.25',
    )
    expect(splitImageRepositoryAndTag('nginx:1.25')).toEqual({ repository: 'nginx', tag: '1.25' })
  })

  it('normalizes repository names discovered from registry workspace resources', () => {
    expect(registryRepositorySuffix('registry.shop-dev.paap.local:5000/paap/orders-web:v1', 'registry.shop-dev.paap.local:5000')).toBe(
      'paap/orders-web',
    )
  })
})
