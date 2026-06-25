<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="modal-overlay"
      role="dialog"
      aria-modal="true"
      @click.self="emit('close')"
    >
      <div class="modal-container">
        <div class="modal-header">
          <div>
            <p class="modal-label">创建环境</p>
            <p class="modal-heading">新建环境</p>
          </div>
          <button
            class="modal-close"
            type="button"
            aria-label="关闭"
            :disabled="creating"
            @click="emit('close')"
          >
            <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-name`">环境名称 <span class="required">*</span></label>
            <input
              :id="`${dialogIdPrefix}-environment-name`"
              :value="form.name"
              class="rail-input"
              placeholder="例如：开发环境"
              @input="updateField('name', ($event.target as HTMLInputElement).value.trim())"
              @keyup.enter="emit('submit')"
            />
          </div>
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-identifier`">环境标识</label>
            <input
              :id="`${dialogIdPrefix}-environment-identifier`"
              :value="form.identifier"
              class="rail-input"
              placeholder="留空由后台生成"
              @input="updateField('identifier', ($event.target as HTMLInputElement).value.trim())"
            />
            <div class="form-helper">当前预览：{{ identifierPreview }}</div>
          </div>
          <div class="form-item">
            <label class="form-label">创建方式</label>
            <div class="radio-group">
              <label class="radio-item" :class="{ active: form.mode === 'blank' }">
                <input type="radio" value="blank" :checked="form.mode === 'blank'" @change="updateField('mode', 'blank')" />
                <span>创建空环境</span>
              </label>
              <label class="radio-item" :class="{ active: form.mode === 'empty' }">
                <input type="radio" value="empty" :checked="form.mode === 'empty'" @change="updateField('mode', 'empty')" />
                <span>创建基础环境</span>
              </label>
              <label class="radio-item" :class="{ active: form.mode === 'template' }">
                <input type="radio" value="template" :checked="form.mode === 'template'" @change="updateField('mode', 'template')" />
                <span>从模板创建</span>
              </label>
            </div>
          </div>
          <div v-if="form.mode === 'template'" class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-template`">选择模板</label>
            <select
              :id="`${dialogIdPrefix}-environment-template`"
              :value="form.templateId"
              class="rail-select"
              @change="updateField('templateId', ($event.target as HTMLSelectElement).value)"
            >
              <option v-for="template in templates" :key="template.id" :value="String(template.id)">{{ template.name }}</option>
            </select>
          </div>
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-additional-namespaces`">附加命名空间</label>
            <textarea
              :id="`${dialogIdPrefix}-additional-namespaces`"
              :value="form.additionalNamespacesInput"
              class="rail-textarea"
              rows="3"
              placeholder="database:database&#10;cache:cache"
              @input="updateField('additionalNamespacesInput', ($event.target as HTMLTextAreaElement).value.trim())"
            ></textarea>
            <div class="form-helper">每行一个后缀，可写成 suffix:purpose；默认保留 app 工作负载空间。</div>
          </div>
          <div class="form-item">
            <label class="form-label" :for="`${dialogIdPrefix}-environment-ip-pool`">网络地址池</label>
            <input
              :id="`${dialogIdPrefix}-environment-ip-pool`"
              class="rail-input environment-ip-pool-state"
              value="暂未启用"
              readonly
              disabled
              :aria-describedby="`${dialogIdPrefix}-environment-ip-pool-helper`"
            />
            <div :id="`${dialogIdPrefix}-environment-ip-pool-helper`" class="form-helper">当前环境创建使用平台默认网络规划，自定义 IP 池将在后续版本启用。</div>
          </div>
          <div v-if="error" class="form-error" role="alert">{{ error }}</div>
        </div>
        <div class="modal-footer">
          <button type="button" class="rail-btn rail-btn--ghost" :disabled="creating" @click="emit('close')">取消</button>
          <button type="button" class="rail-btn rail-btn--primary" :disabled="!form.name || creating" @click="emit('submit')">
            {{ creating ? '创建中...' : '创建' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { toIdentifier } from '../utils/identifier'

type EnvironmentForm = {
  name: string
  identifier: string
  mode: string
  templateId: string
  additionalNamespacesInput: string
}

const props = defineProps<{
  visible: boolean
  creating: boolean
  error: string
  templates: Array<{ id: number | string; name: string }>
  form: EnvironmentForm
  dialogIdPrefix: string
}>()

const emit = defineEmits<{
  (event: 'update:form', value: EnvironmentForm): void
  (event: 'close'): void
  (event: 'submit'): void
}>()

const identifierPreview = computed(() => toIdentifier(props.form.identifier || props.form.name, 'env'))

const updateField = (key: keyof EnvironmentForm, value: string) => {
  emit('update:form', { ...props.form, [key]: value })
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 9000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--paap-space-6);
  background: rgba(17, 19, 24, 0.46);
  backdrop-filter: blur(10px);
}

.modal-container {
  width: min(520px, 100%);
  max-height: 90vh;
  overflow-y: auto;
  background: var(--cds-layer-01, var(--paap-panel));
  border: 1px solid var(--cds-border-subtle-01, var(--paap-border));
  border-radius: 0;
  box-shadow: none;
}

.modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-4);
  padding: var(--paap-space-5) var(--paap-space-6);
  border-bottom: 1px solid var(--cds-border-subtle-01, var(--paap-border));
}

.modal-label {
  margin: 0 0 var(--paap-space-2);
  color: var(--cds-text-secondary, var(--paap-muted));
  font-size: var(--cds-label-01-font-size, 12px);
  font-weight: var(--cds-font-weight-semibold, 600);
  line-height: var(--cds-label-01-line-height, 1.333);
  letter-spacing: var(--cds-label-01-letter-spacing, 0.32px);
  text-transform: uppercase;
}

.modal-heading {
  margin: 0;
  color: var(--cds-text-primary, var(--paap-text));
  font-size: var(--cds-heading-03-font-size, 20px);
  font-weight: var(--cds-heading-03-font-weight, 400);
  line-height: var(--cds-heading-03-line-height, 1.4);
}

.modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--cds-border-subtle-01, var(--paap-border));
  border-radius: 0;
  background: var(--cds-layer-01, var(--paap-panel));
  color: var(--cds-text-secondary, var(--paap-muted));
  cursor: pointer;
}

.modal-close:hover {
  background: var(--cds-layer-hover-01, var(--paap-panel-subtle));
  color: var(--cds-text-primary, var(--paap-text));
}

.modal-close:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.modal-body {
  padding: var(--paap-space-6);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  padding: var(--paap-space-4) var(--paap-space-6);
  border-top: 1px solid var(--cds-border-subtle-01, var(--paap-border));
}

.form-item {
  display: grid;
  gap: var(--paap-space-2);
  margin-bottom: var(--paap-space-5);
}

.form-item:last-child {
  margin-bottom: 0;
}

.form-label {
  color: var(--cds-text-secondary, var(--paap-muted));
  font-size: var(--cds-label-01-font-size, 12px);
  font-weight: var(--cds-font-weight-regular, 400);
  line-height: var(--cds-label-01-line-height, 1.333);
  letter-spacing: var(--cds-label-01-letter-spacing, 0.32px);
}

.required,
.form-error {
  color: var(--cds-text-error, var(--paap-danger));
}

.form-helper {
  color: var(--cds-text-helper, var(--paap-muted-2));
  font-size: var(--cds-helper-text-01-font-size, 12px);
  line-height: var(--cds-helper-text-01-line-height, 1.333);
  letter-spacing: var(--cds-helper-text-01-letter-spacing, 0.32px);
}

.form-error {
  padding: 10px 12px;
  border: 1px solid var(--cds-border-error, var(--cds-red-60, var(--paap-danger)));
  border-radius: 0;
  background: var(--cds-layer-01, var(--paap-panel));
  font-size: var(--cds-body-compact-01-font-size, 14px);
  line-height: var(--cds-body-compact-01-line-height, 1.285);
}

.radio-group {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--paap-space-2);
}

.radio-item {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 40px;
  padding: 8px 10px;
  border: 1px solid var(--cds-border-subtle-01, var(--paap-border));
  border-radius: 0;
  background: var(--cds-layer-01, var(--paap-panel));
  color: var(--cds-text-secondary, var(--paap-muted));
  cursor: pointer;
}

.radio-item.active {
  border-color: var(--cds-border-interactive, var(--paap-accent));
  box-shadow: inset 0 0 0 1px var(--cds-border-interactive, var(--paap-accent));
  color: var(--cds-text-primary, var(--paap-text));
}

.radio-item input {
  margin: 0;
}

@media (max-width: 672px) {
  .modal-overlay {
    align-items: stretch;
    padding: var(--paap-space-4);
  }

  .radio-group {
    grid-template-columns: 1fr;
  }
}
</style>
