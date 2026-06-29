import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import { usePermissionStore } from './stores/permission'

import './styles/carbon-theme.css'
import './style.scss'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)

app.directive('has-perm', {
  mounted(el, binding) {
    const permissionStore = usePermissionStore()
    if (!permissionStore.hasAny(binding.value)) {
      el.parentNode?.removeChild(el)
    }
  },
  updated(el, binding) {
    const permissionStore = usePermissionStore()
    if (!permissionStore.hasAny(binding.value)) {
      el.parentNode?.removeChild(el)
    }
  },
})

app.mount('#app')
