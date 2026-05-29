import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'

// Carbon Design System v10 global styles
import 'carbon-components/css/carbon-components.css'

import './style.scss'

const app = createApp(App)

app.use(createPinia())
app.use(router)

app.mount('#app')
