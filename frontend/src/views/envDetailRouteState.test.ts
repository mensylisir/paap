import { describe, expect, it } from 'vitest'
import { numericRouteParam, routeEnvironmentKey } from './envDetailRouteState'

describe('envDetailRouteState', () => {
  it('parses reused route params without keeping the previous environment id', () => {
    expect(numericRouteParam('1')).toBe(1)
    expect(numericRouteParam(['4'])).toBe(4)
    expect(numericRouteParam(undefined)).toBe(0)
  })

  it('changes the environment state key when the route env id changes', () => {
    expect(routeEnvironmentKey(1, 1)).toBe('1:1')
    expect(routeEnvironmentKey(1, 4)).toBe('1:4')
    expect(routeEnvironmentKey(1, 1)).not.toBe(routeEnvironmentKey(1, 4))
  })
})
