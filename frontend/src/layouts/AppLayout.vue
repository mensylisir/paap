<template>
  <div class="app-wrapper">
    <header class="bx--header" role="banner" aria-label="PAAP">
      <button class="bx--header__menu-toggle bx--header__menu-button" aria-label="Open menu" @click="toggleNav">
        <svg focusable="false" preserveAspectRatio="xMidYMid meet" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="20" height="20" viewBox="0 0 16 16" aria-hidden="true"><path d="M2 3h12v2H2zm0 4h12v2H2zm0 4h12v2H2z"></path></svg>
      </button>
      <router-link to="/apps" class="bx--header__name">
        <span class="bx--header__name--prefix">PAAP</span>&nbsp;{{ appName || '应用' }}
      </router-link>
      <nav class="bx--header__nav" aria-label="app nav">
        <ul class="bx--header__menu-bar">
          <li><router-link to="/apps" class="bx--header__menu-item">返回应用列表</router-link></li>
        </ul>
      </nav>
    </header>

    <aside :class="['bx--side-nav bx--side-nav--fixed', { 'bx--side-nav--expanded': navOpen }]">
      <nav class="bx--side-nav__navigation" role="navigation" aria-label="app nav">
        <ul class="bx--side-nav__items">
          <li class="bx--side-nav__item" :class="{ 'bx--side-nav__item--active': active === 'overview' }">
            <router-link :to="`/apps/${appId}/overview`" class="bx--side-nav__link">
              <span class="bx--side-nav__link-text">应用概览</span>
            </router-link>
          </li>
          <li class="bx--side-nav__item" :class="{ 'bx--side-nav__item--active': active === 'environments' }">
            <router-link :to="`/apps/${appId}/environments`" class="bx--side-nav__link">
              <span class="bx--side-nav__link-text">环境管理</span>
            </router-link>
          </li>
          <li class="bx--side-nav__item" :class="{ 'bx--side-nav__item--active': active === 'deploy' }">
            <router-link :to="`/apps/${appId}/deploy`" class="bx--side-nav__link">
              <span class="bx--side-nav__link-text">部署服务</span>
            </router-link>
          </li>
          <li class="bx--side-nav__item" :class="{ 'bx--side-nav__item--active': active === 'ci' }">
            <router-link :to="`/apps/${appId}/ci`" class="bx--side-nav__link">
              <span class="bx--side-nav__link-text">CI 流水线</span>
            </router-link>
          </li>
          <li class="bx--side-nav__item" :class="{ 'bx--side-nav__item--active': active === 'registry' }">
            <router-link :to="`/apps/${appId}/registry`" class="bx--side-nav__link">
              <span class="bx--side-nav__link-text">镜像仓库</span>
            </router-link>
          </li>
          <li class="bx--side-nav__item" :class="{ 'bx--side-nav__item--active': active === 'monitor' }">
            <router-link :to="`/apps/${appId}/monitor`" class="bx--side-nav__link">
              <span class="bx--side-nav__link-text">监控服务</span>
            </router-link>
          </li>
        </ul>
      </nav>
    </aside>

    <main class="bx--content app-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'

const route = useRoute()
const appId = ref(0)
const appName = ref('')
const navOpen = ref(true)
const active = ref('overview')

const toggleNav = () => {
  navOpen.value = !navOpen.value
}

watch(() => route.params.id, async (id) => {
  if (id) {
    appId.value = Number(id)
    try { appName.value = (await api.getApp(appId.value)).data.application?.name || '应用' }
    catch (e) {}
  }
}, { immediate: true })

watch(() => route.path, (path) => {
  if (path.includes('/deploy')) active.value = 'deploy'
  else if (path.includes('/ci')) active.value = 'ci'
  else if (path.includes('/registry')) active.value = 'registry'
  else if (path.includes('/monitor')) active.value = 'monitor'
  else if (path.includes('/environments')) active.value = 'environments'
  else active.value = 'overview'
}, { immediate: true })
</script>

<style scoped>
.app-wrapper {
  min-height: 100vh;
}

.app-content {
  margin-top: 48px;
  margin-left: 256px;
  padding: 0;
  min-height: calc(100vh - 48px);
  background: #f4f4f4;
}

@media (max-width: 768px) {
  .app-content {
    margin-left: 0;
  }
}
</style>
