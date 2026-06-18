import { describe, expect, it } from 'vitest'
import { parseNativeConfigTemplate } from './configTemplateSyntax'

describe('config template native syntax', () => {
  it('parses simple template tokens into fields and generated config content', () => {
    const parsed = parseNativeConfigTemplate(
      [
        'spring:',
        '  datasource:',
        '    url: __TEMPLATE__JDBC_URL__数据库地址__DEFAULT__jdbc:postgresql://postgresql:5432/postgres__',
        '    username: __TEMPLATE__JDBC_USER__数据库用户__',
        '    password: ${SPRING_DATASOURCE_PASSWORD}',
        '  data:',
        '    redis:',
        '      host: __TEMPLATE__REDIS_HOST__Redis 地址__',
        '      port: __TEMPLATE__REDIS_PORT__Redis 端口__DEFAULT__6379__',
      ].join('\n'),
      { framework: 'springboot' },
    )

    expect(parsed.fields).toEqual(expect.arrayContaining([
      expect.objectContaining({ key: 'JDBC_URL', label: '数据库地址', default: 'jdbc:postgresql://postgresql:5432/postgres', type: 'serviceRef', target: 'postgresql|mysql', format: 'jdbcUrl' }),
      expect.objectContaining({ key: 'JDBC_USER', label: '数据库用户', type: 'text' }),
      expect.objectContaining({ key: 'REDIS_HOST', label: 'Redis 地址', type: 'serviceRef', target: 'redis', format: 'host' }),
      expect.objectContaining({ key: 'REDIS_PORT', label: 'Redis 端口', default: '6379', type: 'number' }),
    ]))
    expect(parsed.configMaps[0].data['application-paap.yml']).toContain('[[paap:JDBC_URL default=jdbc:postgresql://postgresql:5432/postgres]]')
    expect(parsed.files[0]).toEqual(expect.objectContaining({
      key: 'application-paap.yml',
      recommendedMountPath: '/etc/paap/application-paap.yml',
    }))
    expect(parsed.files[0]).not.toHaveProperty('mountPath')
    expect(parsed.env).toEqual(expect.arrayContaining([
      expect.objectContaining({ name: 'SPRING_CONFIG_ADDITIONAL_LOCATION', value: 'file:/etc/paap/' }),
    ]))
  })

  it('parses loop markers as list fields without requiring users to write schema JSON', () => {
    const parsed = parseNativeConfigTemplate(
      [
        'server {',
        '  listen __TEMPLATE__LISTEN_PORT__监听端口__DEFAULT__80__;',
        '  __TEMPLATE__FOR__LOCATION_LIST__位置块列表__',
        '  location __TEMPLATE__ITEM_API_PATH__匹配路径__ {',
        '    proxy_pass __TEMPLATE__ITEM_PROXY_PASS__路由转发__;',
        '  }',
        '  __TEMPLATE__END__LOCATION_LIST__',
        '}',
      ].join('\n'),
      { framework: 'nginx' },
    )

    expect(parsed.fields).toEqual(expect.arrayContaining([
      expect.objectContaining({ key: 'LISTEN_PORT', label: '监听端口', type: 'number', default: '80' }),
      expect.objectContaining({
        key: 'LOCATION_LIST',
        label: '位置块列表',
        type: 'list',
        itemFields: expect.arrayContaining([
          expect.objectContaining({ key: 'API_PATH', label: '匹配路径' }),
          expect.objectContaining({ key: 'PROXY_PASS', label: '路由转发' }),
        ]),
      }),
    ]))
    expect(parsed.configMaps[0].data['default.conf']).toContain('[[paap:for LOCATION_LIST]]')
    expect(parsed.configMaps[0].data['default.conf']).toContain('[[paap:item.API_PATH]]')
    expect(parsed.files[0]).toEqual(expect.objectContaining({
      key: 'default.conf',
      recommendedMountPath: '/etc/nginx/conf.d/default.conf',
    }))
    expect(parsed.files[0]).not.toHaveProperty('mountPath')
  })
})
