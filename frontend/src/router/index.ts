import { createRouter, createWebHistory } from 'vue-router'
import { isUnauthorizedError } from '../api/client'
import { permissions } from '../permissions'
import { usePermissionStore, type PermissionScope } from '../stores/permission'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/LoginView.vue'),
  },
  {
    path: '/',
    component: () => import('../layouts/MainLayout.vue'),
    redirect: '/apps?auto=true',
    children: [
      {
        path: 'apps',
        name: 'AppList',
        component: () => import('../views/AppListView.vue'),
      },
      {
        path: 'apps/create',
        name: 'CreateApp',
        redirect: '/apps',
      },
      {
        path: 'templates',
        name: 'Templates',
        component: () => import('../views/TemplatesView.vue'),
        meta: { permission: permissions.systemTemplateManage },
      },
      {
        path: 'catalog',
        name: 'Catalog',
        component: () => import('../views/CatalogView.vue'),
      },
      {
        path: 'shared-resources',
        name: 'PlatformSharedResources',
        component: () => import('../views/PlatformSharedResourcesView.vue'),
        meta: { permission: permissions.systemSharedPoolManage },
      },
      {
        path: 'platform/services',
        name: 'PlatformServices',
        component: () => import('../views/PlatformServicesView.vue'),
        meta: { permission: permissions.systemSharedPoolManage },
      },
      {
        path: 'platform/addons',
        name: 'PlatformAddons',
        component: () => import('../views/PlatformAddonsView.vue'),
        meta: { permission: permissions.systemSharedPoolManage },
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('../views/PlatformUsersView.vue'),
        meta: { permission: permissions.systemUserManage },
      },
      {
        path: 'roles',
        name: 'Roles',
        redirect: '/users?tab=roles',
      },
      {
        path: 'roles/new',
        name: 'RoleCreate',
        component: () => import('../views/PlatformRoleEditorView.vue'),
        meta: { permission: permissions.systemRoleManage },
      },
      {
        path: 'roles/:roleId',
        name: 'RoleEdit',
        component: () => import('../views/PlatformRoleEditorView.vue'),
        meta: { permission: permissions.systemRoleManage },
      },
      {
        path: 'catalog/:type',
        name: 'CatalogServiceDetail',
        component: () => import('../views/CatalogServiceDetailView.vue'),
      },
      {
        path: 'registries',
        name: 'AppRegistries',
        component: () => import('../views/AppRegistryView.vue'),
      },
    ],
  },
  {
    path: '/apps/:id',
    component: () => import('../layouts/AppLayout.vue'),
    redirect: (to: any) => `/apps/${to.params.id}/overview`,
    children: [
      {
        path: 'overview',
        name: 'AppOverview',
        component: () => import('../views/AppOverviewView.vue'),
        meta: { permission: permissions.appRead },
      },
      {
        path: 'members',
        name: 'AppMembers',
        component: () => import('../views/AppMembersView.vue'),
        meta: { permission: permissions.appMemberRead },
      },
      {
        path: 'environments',
        name: 'AppEnvironments',
        component: () => import('../views/AppEnvironmentsView.vue'),
        meta: { permission: permissions.appRead },
      },
      {
        path: 'environments/:envId',
        name: 'EnvDetail',
        component: () => import('../views/EnvDetailView.vue'),
        meta: { permission: permissions.envRead },
      },
      {
        path: 'environments/:envId/components/:compId',
        name: 'ComponentDetail',
        component: () => import('../views/ComponentDetailView.vue'),
        meta: { permission: permissions.componentRead },
      },
      {
        path: 'deploy',
        name: 'AppDeploy',
        component: () => import('../views/AppDeployView.vue'),
        meta: { permission: permissions.componentDeploy },
      },
      {
        path: 'ci',
        name: 'AppCI',
        component: () => import('../views/AppCIView.vue'),
        meta: { permission: permissions.componentDeploy },
      },
      {
        path: 'monitor',
        name: 'AppMonitor',
        component: () => import('../views/AppMonitorView.vue'),
        meta: { permission: permissions.envRead },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

function routePermissionScope(to: any): PermissionScope {
  const envId = Number(to.params?.envId)
  if (Number.isFinite(envId) && envId > 0) {
    return { scopeType: 'env', scopeId: envId }
  }
  const appId = Number(to.params?.id)
  if (Number.isFinite(appId) && appId > 0) {
    return { scopeType: 'app', scopeId: appId }
  }
  return { scopeType: 'system' }
}

router.beforeEach(async (to) => {
  const token = typeof localStorage === 'undefined' ? '' : localStorage.getItem('paap_token')
  if (to.path !== '/login' && !token) return '/login'
  if (!token || to.path === '/login') return

  const permissionStore = usePermissionStore()
  const scope = routePermissionScope(to)
  try {
    await permissionStore.fetchPermissions(scope)
  } catch (err) {
    if (isUnauthorizedError(err)) {
      localStorage.removeItem('paap_token')
      localStorage.removeItem('paap_user')
      permissionStore.reset()
      return '/login'
    }
    throw err
  }

  const required = to.matched
    .map((record) => record.meta.permission)
    .find(Boolean) as string | string[] | undefined
  if (required && !permissionStore.hasAny(required)) {
    return '/apps?auto=true'
  }
})

export default router
