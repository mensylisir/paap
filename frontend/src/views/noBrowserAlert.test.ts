import { describe, expect, it } from 'vitest'

describe('view feedback', () => {
  it('does not use blocking browser dialogs in Vue views', () => {
    const viewSources = import.meta.glob('./*.vue', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
    const offenders = Object.entries(viewSources).flatMap(([file, content]) =>
      /\b(alert|confirm|prompt)\s*\(/.test(content) ? [file.replace('./', '')] : [],
    )

    expect(offenders).toEqual([])
  })
})
