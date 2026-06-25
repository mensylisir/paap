import { createRouter, createWebHistory } from 'vue-router'

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
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('../views/PlatformUsersView.vue'),
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
      },
      {
        path: 'members',
        name: 'AppMembers',
        component: () => import('../views/AppMembersView.vue'),
      },
      {
        path: 'environments',
        name: 'AppEnvironments',
        component: () => import('../views/AppEnvironmentsView.vue'),
      },
      {
        path: 'environments/:envId',
        name: 'EnvDetail',
        component: () => import('../views/EnvDetailView.vue'),
      },
      {
        path: 'environments/:envId/components/:compId',
        name: 'ComponentDetail',
        component: () => import('../views/ComponentDetailView.vue'),
      },
      {
        path: 'deploy',
        name: 'AppDeploy',
        component: () => import('../views/AppDeployView.vue'),
      },
      {
        path: 'ci',
        name: 'AppCI',
        component: () => import('../views/AppCIView.vue'),
      },
      {
        path: 'monitor',
        name: 'AppMonitor',
        component: () => import('../views/AppMonitorView.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const token = typeof localStorage === 'undefined' ? '' : localStorage.getItem('paap_token')
  if (to.path !== '/login' && !token) return '/login'
  if (to.path === '/login' && token) return '/apps?auto=true'
})

export default router
