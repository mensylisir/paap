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
    redirect: '/apps',
    children: [
      {
        path: 'apps',
        name: 'AppList',
        component: () => import('../views/AppListView.vue'),
      },
      {
        path: 'apps/create',
        name: 'CreateApp',
        component: () => import('../views/CreateAppView.vue'),
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
        path: 'registry',
        name: 'AppRegistry',
        component: () => import('../views/AppRegistryView.vue'),
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

export default router
