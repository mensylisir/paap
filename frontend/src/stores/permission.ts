import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '../api/client'

export type PermissionScopeType = 'system' | 'app' | 'env'

export type PermissionScope = {
  scopeType?: PermissionScopeType
  scopeId?: number
}

const scopeKey = (scope: PermissionScope = {}) =>
  `${scope.scopeType || 'system'}:${scope.scopeId || 0}`

export const usePermissionStore = defineStore('permission', () => {
  const roles = ref<string[]>([])
  const permissions = ref<string[]>([])
  const loadedScope = ref('')
  const loading = ref(false)
  const loaded = computed(() => loadedScope.value !== '')
  const permissionSet = computed(() => new Set(permissions.value))

  async function fetchPermissions(scope: PermissionScope = {}) {
    const nextScope = scopeKey(scope)
    if (loadedScope.value === nextScope && permissions.value.length > 0) return
    loading.value = true
    try {
      const response = await api.currentPermissions(scope.scopeType, scope.scopeId)
      roles.value = Array.isArray(response?.data?.roles) ? response.data.roles : []
      permissions.value = Array.isArray(response?.data?.permissions) ? response.data.permissions : []
      loadedScope.value = nextScope
    } finally {
      loading.value = false
    }
  }

  function reset() {
    roles.value = []
    permissions.value = []
    loadedScope.value = ''
  }

  function has(permission: string) {
    return permissionSet.value.has(permission)
  }

  function hasAny(required: string | string[] | undefined | null) {
    if (!required) return true
    const values = Array.isArray(required) ? required : [required]
    return values.some((value) => permissionSet.value.has(value))
  }

  return {
    roles,
    permissions,
    loaded,
    loadedScope,
    loading,
    fetchPermissions,
    reset,
    has,
    hasAny,
  }
})
