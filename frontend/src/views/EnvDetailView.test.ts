import { describe, it, expect } from 'vitest'
import { buildPickerTemplates, createPickerSessionState, pickerNotice } from './envDetailServicePicker'

describe('EnvDetailView service picker', () => {
  it('picks the first installable template for immediate modal interaction', () => {
    const result = createPickerSessionState(
      [
        { type: 'deploy', category: 'tool', name: 'ArgoCD' },
        { type: 'ci', category: 'tool', name: 'Jenkins' },
      ],
      [{ serviceType: 'deploy', status: 'running' }],
      'tool',
    )

    expect(result.selectedType).toBe('ci')
    expect(result.loading).toBe(false)
  })

  it('keeps installed templates visible but disabled', () => {
    const result = buildPickerTemplates(
      [
        { type: 'deploy', category: 'tool', name: 'ArgoCD' },
        { type: 'ci', category: 'tool', name: 'Jenkins' },
      ],
      [{ serviceType: 'deploy', status: 'running' }],
      'tool',
    )

    expect(result).toHaveLength(2)
    expect(result[0].disabled).toBe(true)
    expect(result[0].statusText).toBe('已安装')
    expect(result[1].disabled).toBe(false)
    expect(result[1].statusText).toBe('可添加')
  })

  it('keeps draft and failed canvas service cards from being added twice', () => {
    const result = buildPickerTemplates(
      [
        { type: 'redis', category: 'infra', name: 'Redis' },
        { type: 'rabbitmq', category: 'infra', name: 'RabbitMQ' },
        { type: 'mysql', category: 'infra', name: 'MySQL' },
      ],
      [
        { serviceType: 'redis', status: 'draft' },
        { serviceType: 'rabbitmq', status: 'failed' },
      ],
      'infra',
    )

    expect(result.map((item) => `${item.type}:${item.disabled}:${item.statusText}`)).toEqual([
      'redis:true:已添加',
      'rabbitmq:true:安装失败',
      'mysql:false:可添加',
    ])
  })

  it('selects wording for adding services instead of installing them on the canvas', () => {
    const result = createPickerSessionState(
      [
        { type: 'deploy', category: 'tool', name: 'ArgoCD' },
        { type: 'ci', category: 'tool', name: 'Jenkins' },
      ],
      [{ serviceType: 'deploy', status: 'draft' }],
      'tool',
    )

    expect(result.selectedType).toBe('ci')
    expect(result.availableServices[0].statusText).toBe('已添加')
    expect(result.availableServices[1].statusText).toBe('可添加')
  })

  it('allows installing mysql when postgresql is already installed', () => {
    const result = buildPickerTemplates(
      [
        { type: 'postgresql', category: 'infra', name: 'PostgreSQL' },
        { type: 'mysql', category: 'infra', name: 'MySQL' },
      ],
      [{ serviceType: 'postgresql', status: 'running' }],
      'infra',
    )

    expect(result.map((item) => `${item.type}:${item.statusText}`)).toEqual([
      'postgresql:已安装',
      'mysql:可添加',
    ])
    expect(result.find((item) => item.type === 'mysql')?.disabled).toBe(false)
  })

  it('prepares a visible picker state before async refresh completes', () => {
    const result = createPickerSessionState(
      [
        { type: 'deploy', category: 'tool', name: 'ArgoCD' },
        { type: 'ci', category: 'tool', name: 'Jenkins' },
      ],
      [{ serviceType: 'deploy', status: 'running' }],
      'tool',
    )

    expect(result.loading).toBe(false)
    expect(result.availableServices.map((item) => item.type)).toEqual(['deploy', 'ci'])
    expect(result.selectedType).toBe('ci')
    expect(result.notice).toBe('')
  })

  it('uses loading only while no templates have been loaded yet', () => {
    const result = createPickerSessionState([], [], 'tool')

    expect(result.loading).toBe(true)
    expect(result.notice).toContain('正在加载')
  })

  it('shows a notice when no selectable services remain', () => {
    expect(pickerNotice('tool', 0, '')).toContain('没有可用的工具模板')
    expect(pickerNotice('infra', 0, '')).toContain('没有可用的中间件模板')
    expect(pickerNotice('tool', 2, '')).toContain('工具模板')
  })
})
