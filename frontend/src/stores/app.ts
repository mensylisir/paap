import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAppStore = defineStore('app', () => {
  const isSideNavOpen = ref(true)

  const toggleSideNav = () => {
    isSideNavOpen.value = !isSideNavOpen.value
  }

  return { isSideNavOpen, toggleSideNav }
})
