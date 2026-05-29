<template>
  <div class="app-wrapper">
    <header class="bx--header" role="banner" aria-label="PAAP">
      <a class="bx--header__name" href="/">
        <span class="bx--header__name--prefix">PAAP</span>&nbsp;应用管理平台
      </a>
      <nav class="bx--header__nav" aria-label=" main nav">
        <ul class="bx--header__menu-bar">
          <li><router-link to="/apps" class="bx--header__menu-item">我的应用</router-link></li>
        </ul>
      </nav>
      <div class="bx--header__global">
        <button class="bx--header__menu-item" title="退出" @click="logout">
          <svg focusable="false" preserveAspectRatio="xMidYMid meet" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="20" height="20" viewBox="0 0 32 32" aria-hidden="true"><path d="M12 4H6v24h6v-2H8V6h4zM21.4 22.6L26 18l-4.6-4.6L22 12l6 6-6 6zM13 17h10v2H13z"></path></svg>
        </button>
      </div>
    </header>
    <main class="bx--content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
const logout = () => {
  location.href = '/login'
}
</script>

<style scoped>
.app-wrapper {
  min-height: 100vh;
}

.bx--content {
  margin-top: 48px;
  padding: 0;
  min-height: calc(100vh - 48px);
  background: #f4f4f4;
}
</style>
