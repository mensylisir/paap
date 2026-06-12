import { describe, expect, it } from 'vitest'

describe('frontend diagnostics', () => {
  it('does not leave console log/debug statements in production source', () => {
    const sources = {
      ...import.meta.glob('./**/*.ts', { query: '?raw', import: 'default', eager: true }),
      ...import.meta.glob('./**/*.vue', { query: '?raw', import: 'default', eager: true }),
    } as Record<string, string>

    const offenders = Object.entries(sources).flatMap(([file, content]) => {
      if (file.endsWith('.test.ts')) return []
      return /\bconsole\.(log|debug)\s*\(/.test(content) ? [file.replace('./', '')] : []
    })

    expect(offenders).toEqual([])
  })
})
