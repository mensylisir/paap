<template>
  <div class="rail-page">
    <nav class="breadcrumb">
      <router-link to="/apps" class="breadcrumb-link">我的应用</router-link>
      <span class="breadcrumb-sep">/</span>
      <span class="breadcrumb-current">创建应用</span>
    </nav>

    <header class="page-header">
      <h1 class="page-title">创建应用</h1>
      <p class="page-subtitle">填写基本信息，快速创建一个新的应用</p>
    </header>

    <div class="form-card">
      <form @submit.prevent="submit">
        <div class="form-group">
          <label class="form-label">应用名称 <span class="required">*</span></label>
          <input v-model.trim="form.name" class="form-input" placeholder="例如：订单服务" required />
        </div>

        <div class="form-group">
          <label class="form-label">应用标识</label>
          <input v-model.trim="form.identifier" class="form-input" placeholder="留空由后台生成唯一标识" pattern="[a-z0-9-]+" />
          <p class="form-hint">可选。留空时后台会根据应用名称生成唯一英文标识，当前预览：<code>{{ identifierPreview }}</code></p>
        </div>

        <div class="form-group">
          <label class="form-label">应用描述</label>
          <textarea v-model.trim="form.description" class="form-textarea" rows="3" placeholder="简要描述应用的用途"></textarea>
        </div>

        <div v-if="formError" class="form-error" role="alert">{{ formError }}</div>

        <div class="form-actions">
          <button type="button" class="btn btn--ghost" @click="$router.push('/apps')">取消</button>
          <button type="submit" class="btn btn--primary" :disabled="submitting">
            {{ submitting ? '创建中...' : '创建应用' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'
import { toIdentifier } from '../utils/identifier'

const router = useRouter()
const form = ref({ name: '', identifier: '', description: '' })
const submitting = ref(false)
const formError = ref('')
const identifierPreview = computed(() => toIdentifier(form.value.identifier || form.value.name, 'app'))

const submit = async () => {
  if (!form.value.name) return
  submitting.value = true
  formError.value = ''
  try {
    const res = await api.createApp({
      name: form.value.name,
      identifier: form.value.identifier,
      description: form.value.description,
    })
    router.push(`/apps/${res.data.id}/overview`)
  } catch (e: any) {
    formError.value = '创建失败：' + (e?.message || '未知错误')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.rail-page {
  padding: var(--paap-space-10) var(--paap-space-8) var(--paap-space-16);
  max-width: 640px;
}

/* Breadcrumb */
.breadcrumb {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  margin-bottom: var(--paap-space-6);
  font-size: var(--paap-fs-body);
}
.breadcrumb-link {
  color: var(--paap-muted);
  text-decoration: none;
}
.breadcrumb-link:hover {
  color: var(--paap-text);
  text-decoration: underline;
}
.breadcrumb-sep {
  color: var(--paap-border-strong);
}
.breadcrumb-current {
  color: var(--paap-text);
  font-weight: 500;
}

/* Header */
.page-header {
  margin-bottom: var(--paap-space-8);
}
.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--paap-text);
  letter-spacing: -0.02em;
  line-height: 1.2;
  margin: 0;
}
.page-subtitle {
  font-size: var(--paap-fs-body);
  color: var(--paap-muted);
  margin-top: var(--paap-space-1);
}

/* Form card */
.form-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-8);
}

/* Form groups */
.form-group {
  margin-bottom: var(--paap-space-6);
}
.form-group:last-of-type {
  margin-bottom: 0;
}
.form-label {
  display: block;
  font-size: var(--paap-fs-compact);
  font-weight: 500;
  color: var(--paap-text);
  margin-bottom: var(--paap-space-2);
}
.required {
  color: var(--paap-danger);
}
.form-input {
  width: 100%;
  padding: 10px 12px;
  font-size: var(--paap-fs-body);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  outline: none;
  font-family: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.form-input:focus {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-focus-ring);
}
.form-input::placeholder {
  color: var(--paap-muted);
}
.form-textarea {
  width: 100%;
  resize: vertical;
  padding: 10px 12px;
  font-size: var(--paap-fs-body);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  outline: none;
  font-family: inherit;
  line-height: 1.5;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.form-textarea:focus {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-focus-ring);
}
.form-textarea::placeholder {
  color: var(--paap-muted);
}
.form-hint {
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
  margin-top: var(--paap-space-1);
  line-height: 1.5;
}
.form-hint code {
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-small);
  background: var(--paap-panel-subtle);
  padding: 1px 4px;
  border-radius: var(--paap-radius-xs);
}

/* Error */
.form-error {
  border: 1px solid var(--paap-danger);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  border-radius: var(--paap-radius);
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
  line-height: 1.4;
  margin-bottom: var(--paap-space-6);
}

/* Actions */
.form-actions {
  display: flex;
  gap: var(--paap-space-3);
  margin-top: var(--paap-space-8);
  padding-top: var(--paap-space-6);
  border-top: 1px solid var(--paap-border);
}

/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: inherit;
  font-size: var(--paap-fs-body);
  font-weight: 500;
  cursor: pointer;
  outline: none;
  border: 1px solid transparent;
  height: 40px;
  padding: 0 20px;
  border-radius: var(--paap-radius-sm);
  transition: all 0.15s;
  gap: 6px;
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn--primary {
  background: var(--paap-accent);
  color: #ffffff;
}
.btn--primary:hover:not(:disabled) {
  background: var(--paap-accent-hover);
}
.btn--ghost {
  background: var(--paap-panel);
  color: var(--paap-muted);
  border-color: var(--paap-border);
}
.btn--ghost:hover:not(:disabled) {
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  border-color: var(--paap-border-strong);
}

@media (max-width: 672px) {
  .rail-page {
    padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10);
    max-width: none;
  }
  .form-card { padding: var(--paap-space-6); }
  .form-actions { flex-direction: column; }
  .btn { width: 100%; }
}
</style>
