import { describe, expect, it } from 'vitest'
import { shouldPollTemplateInstallations } from './envInstallPolling'

describe('envInstallPolling', () => {
  it('polls a template-created environment until service installations appear', () => {
    expect(shouldPollTemplateInstallations({ templateId: 2, status: 'creating' }, [])).toBe(true)
    expect(shouldPollTemplateInstallations({ templateId: 2, status: 'running' }, [])).toBe(true)
  })

  it('polls while service or component runtime state is still pending', () => {
    expect(shouldPollTemplateInstallations({ templateId: 0, status: 'running' }, [{ status: 'installing' }])).toBe(true)
    expect(shouldPollTemplateInstallations({ templateId: 0, status: 'running' }, [], [{ status: 'creating' }])).toBe(true)
  })

  it('does not poll empty environments or stable environments that already have services', () => {
    expect(shouldPollTemplateInstallations({ templateId: 0, status: 'empty' }, [])).toBe(false)
    expect(shouldPollTemplateInstallations({ templateId: 2, status: 'running' }, [{ serviceType: 'git', status: 'running' }])).toBe(false)
  })
})
