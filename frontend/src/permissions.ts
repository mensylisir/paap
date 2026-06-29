export const permissions = {
  systemUserRead: 'system.user.read',
  systemUserManage: 'system.user.manage',
  systemRoleManage: 'system.role.manage',
  systemTemplateManage: 'system.template.manage',
  systemSharedPoolManage: 'system.shared_pool.manage',

  appCreate: 'app.create',
  appRead: 'app.read',
  appUpdate: 'app.update',
  appDelete: 'app.delete',
  appMemberRead: 'app.member.read',
  appMemberManage: 'app.member.manage',

  envCreate: 'env.create',
  envRead: 'env.read',
  envManage: 'env.manage',
  envDelete: 'env.delete',

  serviceRead: 'service.read',
  serviceInstall: 'service.install',
  serviceManage: 'service.manage',

  componentRead: 'component.read',
  componentCreate: 'component.create',
  componentDeploy: 'component.deploy',
  componentManage: 'component.manage',
} as const

export type PermissionCode = typeof permissions[keyof typeof permissions]
