import { describe, expect, it } from 'vitest'
import { shouldPollTemplateInstallations } from './envInstallPolling'

describe('envInstallPolling', () => {
  it('polls a template-created environment until service installations appear', () => {
    expect(shouldPollTemplateInstallations({ templateId: 2, status: 'creating' }, [])).toBe(true)
    expect(shouldPollTemplateInstallations({ templateId: 2, status: 'running' }, [])).toBe(true)
  })

  it('does not poll empty environments or environments that already have services', () => {
    expect(shouldPollTemplateInstallations({ templateId: 0, status: 'empty' }, [])).toBe(false)
    expect(shouldPollTemplateInstallations({ templateId: 2, status: 'creating' }, [{ serviceType: 'git' }])).toBe(false)
  })
})
