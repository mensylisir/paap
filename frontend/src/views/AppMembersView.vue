<template>
  <div class="rail-page">
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">成员</h1>
        <p class="page-desc">邀请已有用户加入应用，并维护应用内角色</p>
      </div>
    </header>

    <section class="section-card member-section slide-up">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">应用成员</h2>
          <p class="rail-section-desc">成员权限只作用于当前应用</p>
        </div>
      </div>

      <form v-has-perm="'app.member.manage'" class="member-invite-form" @submit.prevent="inviteMember">
        <div class="member-form-fields">
          <label class="sr-only" for="member-username">用户名</label>
          <input
            id="member-username"
            v-model.trim="memberForm.username"
            class="rail-input"
            placeholder="输入用户名"
            autocomplete="off"
          />
          <label class="sr-only" for="member-role">成员角色</label>
          <select id="member-role" v-model="memberForm.role" class="rail-select member-role-select">
            <option v-for="role in memberRoles" :key="role.value" :value="role.value">{{ role.label }}</option>
          </select>
        </div>
        <button type="submit" class="rail-btn rail-btn--primary" :disabled="!memberForm.username || !memberForm.role || memberSubmitting">
          {{ memberSubmitting ? '邀请中...' : '邀请成员' }}
        </button>
      </form>

      <div v-if="memberError" class="form-error member-error" role="alert">{{ memberError }}</div>

      <div v-if="members.length" class="member-list">
        <div v-for="member in members" :key="member.id" class="member-row">
          <div class="member-main">
            <div class="member-avatar">{{ memberInitial(member) }}</div>
            <div class="member-copy">
              <strong>{{ memberDisplayName(member) }}</strong>
              <span>{{ member.user?.email || `用户 #${member.userId}` }}</span>
            </div>
          </div>
          <div class="member-actions">
            <select
              v-model="member.role"
              class="rail-select member-role-select"
              :disabled="updatingMemberId === Number(member.id) || !permissionStore.has('app.member.manage')"
              @change="updateMemberRole(member, member.role)"
            >
              <option v-for="role in memberRoles" :key="role.value" :value="role.value">{{ role.label }}</option>
            </select>
            <button
              type="button"
              class="rail-btn rail-btn--ghost rail-btn--sm"
              :disabled="removingMemberId === Number(member.id) || !canRemoveMember(member) || !permissionStore.has('app.member.manage')"
              @click="removeMember(member)"
            >
              {{ removingMemberId === Number(member.id) ? '移除中...' : '移除' }}
            </button>
          </div>
        </div>
      </div>
      <div v-else class="rail-empty minimal">
        <p class="rail-empty-desc">暂无成员记录</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'
import { usePermissionStore } from '../stores/permission'

const route = useRoute()
const permissionStore = usePermissionStore()
const appId = Number(route.params.id)
const members = ref<any[]>([])
const memberSubmitting = ref(false)
const updatingMemberId = ref<number | null>(null)
const removingMemberId = ref<number | null>(null)
const memberError = ref('')
const memberForm = ref({ username: '', role: '' })
const memberRoles = ref<{ value: string; label: string }[]>([])

async function loadAppRoles() {
  try {
    const res = await api.listAssignableRoles('app')
    memberRoles.value = Array.isArray(res.data)
      ? res.data
          .map((role: any) => ({ value: String(role?.code || ''), label: String(role?.name || role?.code || '') }))
          .filter((role: { value: string }) => role.value)
      : []
    const defaultRole = memberRoles.value.find((role) => role.value === 'member') || memberRoles.value[0]
    if (!memberForm.value.role && defaultRole) memberForm.value.role = defaultRole.value
  } catch (e: any) {
    memberError.value = '加载角色失败：' + (e?.message || '未知错误')
  }
}

async function loadAppMembers() {
  try {
    const res = await api.listAppMembers(appId)
    members.value = res.data || []
  } catch (e: any) {
    memberError.value = '加载成员失败：' + (e?.message || '未知错误')
  }
}

const memberDisplayName = (member: any) => member?.user?.username || `用户 #${member?.userId || member?.id || '-'}`
const memberInitial = (member: any) => memberDisplayName(member).slice(0, 1).toUpperCase()
const adminMemberCount = computed(() => members.value.filter((member: any) => member.role === 'admin').length)
const canRemoveMember = (member: any) => !(member?.role === 'admin' && adminMemberCount.value <= 1)

const inviteMember = async () => {
  if (!memberForm.value.username || memberSubmitting.value) return
  memberSubmitting.value = true
  memberError.value = ''
  try {
    await api.inviteAppMember(appId, { ...memberForm.value })
    const defaultRole = memberRoles.value.find((role) => role.value === 'member') || memberRoles.value[0]
    memberForm.value = { username: '', role: defaultRole?.value || '' }
    await loadAppMembers()
  } catch (e: any) {
    memberError.value = '邀请成员失败：' + (e?.message || '未知错误')
  } finally {
    memberSubmitting.value = false
  }
}

const updateMemberRole = async (member: any, role: string) => {
  if (!member?.id || updatingMemberId.value !== null) return
  updatingMemberId.value = Number(member.id)
  memberError.value = ''
  try {
    await api.updateAppMember(appId, Number(member.id), { role })
    await loadAppMembers()
  } catch (e: any) {
    memberError.value = '更新成员角色失败：' + (e?.message || '未知错误')
    await loadAppMembers()
  } finally {
    updatingMemberId.value = null
  }
}

const removeMember = async (member: any) => {
  if (!member?.id || removingMemberId.value !== null || !canRemoveMember(member)) return
  removingMemberId.value = Number(member.id)
  memberError.value = ''
  try {
    await api.removeAppMember(appId, Number(member.id))
    await loadAppMembers()
  } catch (e: any) {
    memberError.value = '移除成员失败：' + (e?.message || '未知错误')
  } finally {
    removingMemberId.value = null
  }
}

onMounted(async () => {
  await loadAppRoles()
  await loadAppMembers()
})
</script>

<style scoped>
.rail-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10);
  max-width: none;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: var(--paap-space-8);
}
.header-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--paap-text);
  letter-spacing: -0.02em;
  line-height: 1.2;
}
.page-desc {
  font-size: var(--paap-fs-body);
  color: var(--paap-muted);
  line-height: 1.4;
  margin-top: var(--paap-space-1);
}
.section-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-6);
  margin-bottom: var(--paap-space-4);
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--paap-space-5);
}
.form-error {
  padding: 10px 12px;
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  font-size: var(--paap-fs-compact);
}
.member-section {
  display: grid;
  gap: var(--paap-space-4);
}
.member-invite-form {
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  gap: var(--paap-space-3);
}
.member-form-fields {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) 148px;
  gap: var(--paap-space-3);
  flex: 1;
  min-width: 0;
}
.member-error {
  margin: 0;
}
.member-list {
  display: grid;
  border-top: 1px solid var(--paap-border);
}
.member-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-4);
  min-height: 64px;
  padding: var(--paap-space-3) 0;
  border-bottom: 1px solid var(--paap-border);
}
.member-main {
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
  min-width: 0;
}
.member-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
}
.member-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.member-copy strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
}
.member-copy span {
  overflow: hidden;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.member-actions {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  flex-shrink: 0;
}
.member-role-select {
  min-width: 128px;
}
.minimal {
  padding: var(--paap-space-8);
}

@media (max-width: 960px) {
  .member-invite-form { flex-direction: column; }
}
@media (max-width: 672px) {
  .rail-page { padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10); }
  .page-header { flex-direction: column; align-items: flex-start; gap: var(--paap-space-4); }
  .section-header { flex-direction: column; gap: var(--paap-space-3); }
  .member-form-fields { grid-template-columns: 1fr; }
  .member-row { align-items: flex-start; flex-direction: column; }
  .member-actions { width: 100%; justify-content: space-between; }
}
</style>
