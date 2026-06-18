import { describe, expect, it } from 'vitest'
import {
  componentTemplateFileMountPath,
  mergeComponentConfigFile,
  normalizeComponentTemplateFiles,
} from './componentConfigTemplateFiles'

describe('componentConfigTemplateFiles', () => {
  it('treats template mountPath as a recommendation instead of a runtime binding', () => {
    const files = normalizeComponentTemplateFiles([{
      name: 'default.conf',
      configMapName: '{{configMapName}}',
      key: 'default.conf',
      mountPath: '/etc/nginx/conf.d/default.conf',
      readOnly: true,
    }])

    expect(files).toEqual([{
      name: 'default.conf',
      configMapName: '{{configMapName}}',
      key: 'default.conf',
      recommendedMountPath: '/etc/nginx/conf.d/default.conf',
      readOnly: true,
    }])
    expect(files[0]).not.toHaveProperty('mountPath')
  })

  it('keeps component-specific mount path overrides when a template is re-applied', () => {
    const mountPath = componentTemplateFileMountPath({
      templateFile: {
        name: 'default.conf',
        configMapName: '{{configMapName}}',
        key: 'default.conf',
        recommendedMountPath: '/etc/nginx/conf.d/default.conf',
      },
      configMapName: 'frontend-1-config',
      key: 'default.conf',
      existingFiles: [{
        name: 'default.conf',
        configMapName: 'frontend-1-config',
        key: 'default.conf',
        mountPath: '/custom/nginx/default.conf',
        readOnly: true,
      }],
      render: (value) => value.replace('{{configMapName}}', 'frontend-1-config'),
    })

    expect(mountPath).toBe('/custom/nginx/default.conf')
  })

  it('updates existing component file mounts by config object and key instead of duplicating rows', () => {
    const files = mergeComponentConfigFile([{
      name: 'default.conf',
      configMapName: 'frontend-1-config',
      key: 'default.conf',
      mountPath: '/custom/nginx/default.conf',
      readOnly: true,
    }], {
      name: 'default.conf',
      configMapName: 'frontend-1-config',
      key: 'default.conf',
      mountPath: '/custom/nginx/default.conf',
      readOnly: false,
    })

    expect(files).toHaveLength(1)
    expect(files[0]).toMatchObject({ mountPath: '/custom/nginx/default.conf', readOnly: false })
  })
})
