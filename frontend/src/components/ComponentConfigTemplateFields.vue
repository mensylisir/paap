<template>
  <div class="component-template-field-list">
    <label
      v-for="field in visibleFields"
      :key="componentTemplateFieldKey(field)"
      class="component-template-field"
      :class="{ 'component-template-field--list': componentTemplateFieldType(field) === 'list' }"
    >
      <span class="component-template-field-label">
        <span>
          {{ componentTemplateFieldLabel(field) }}
          <em v-if="requiredForUser(field)">必填</em>
        </span>
        <small v-if="fieldHint(field)">{{ fieldHint(field) }}</small>
      </span>

      <span v-if="componentTemplateFieldType(field) === 'list'" class="component-template-control component-template-list-control">
        <span
          v-for="(row, rowIndex) in componentTemplateListRows(field)"
          :key="`${componentTemplateFieldKey(field)}-${rowIndex}`"
          class="component-template-list-row"
        >
          <template v-for="itemField in componentTemplateVisibleListItemFields(field)" :key="componentTemplateFieldKey(itemField)">
            <template v-if="fieldUsesTargetSelect(itemField)">
              <select
                :value="row[componentTemplateFieldKey(itemField)]"
                class="bx--select-input"
                :aria-label="componentTemplateFieldLabel(itemField)"
                @change="updateComponentTemplateListCell(field, rowIndex, itemField, eventValue($event))"
              >
                <option value="">{{ fieldPlaceholder(itemField) }}</option>
                <option v-for="option in fieldOptions(itemField)" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </template>
            <textarea
              v-else-if="componentTemplateFieldType(itemField) === 'textarea'"
              :value="row[componentTemplateFieldKey(itemField)]"
              class="bx--text-input component-template-textarea component-template-list-textarea"
              :aria-label="componentTemplateFieldLabel(itemField)"
              :placeholder="componentTemplateFieldDefaultValue(itemField) || componentTemplateFieldLabel(itemField)"
              rows="5"
              @input="updateComponentTemplateListCell(field, rowIndex, itemField, eventValue($event))"
            ></textarea>
            <input
              v-else
              :value="row[componentTemplateFieldKey(itemField)]"
              class="bx--text-input"
              :aria-label="componentTemplateFieldLabel(itemField)"
              :placeholder="componentTemplateFieldDefaultValue(itemField) || componentTemplateFieldLabel(itemField)"
              @input="updateComponentTemplateListCell(field, rowIndex, itemField, eventValue($event))"
            />
          </template>
          <span class="component-template-list-actions">
            <button type="button" title="添加一组" aria-label="添加一组" @click="addComponentTemplateListRow(field)">+</button>
            <button
              type="button"
              title="删除这一组"
              aria-label="删除这一组"
              :disabled="componentTemplateListRows(field).length <= 1"
              @click="removeComponentTemplateListRow(field, rowIndex)"
            >-</button>
          </span>
        </span>
      </span>

      <span v-else class="component-template-control">
        <label v-if="componentTemplateFieldType(field) === 'boolean'" class="component-template-checkbox">
          <input
            :checked="Boolean(fieldValues[componentTemplateFieldKey(field)])"
            type="checkbox"
            @change="updateComponentTemplateField(field, eventChecked($event))"
          />
          <span>{{ componentTemplateFieldLabel(field) }}</span>
        </label>

        <template v-else-if="fieldUsesTargetSelect(field)">
          <select
            :value="fieldValues[componentTemplateFieldKey(field)]"
            class="bx--select-input"
            @change="updateComponentTemplateField(field, eventValue($event))"
          >
            <option value="">{{ fieldPlaceholder(field) }}</option>
            <option v-for="option in fieldOptions(field)" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </template>

        <span v-else-if="componentTemplateFieldType(field) === 'password'" class="password-field-wrap">
          <input
            :value="fieldValues[componentTemplateFieldKey(field)]"
            class="bx--text-input password-field-input"
            :type="componentTemplatePasswordVisible(field) ? 'text' : 'password'"
            :placeholder="fieldPlaceholder(field)"
            @input="updateComponentTemplateField(field, eventValue($event))"
          />
          <button
            type="button"
            class="password-visible-toggle"
            :aria-label="componentTemplatePasswordVisible(field) ? '隐藏密码' : '显示密码'"
            :title="componentTemplatePasswordVisible(field) ? '隐藏密码' : '显示密码'"
            @click="toggleComponentTemplatePassword(field)"
          >
            <svg v-if="componentTemplatePasswordVisible(field)" focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
              <path d="M16 6C7 6 2 16 2 16s5 10 14 10 14-10 14-10S25 6 16 6zm0 18c-6.4 0-10.5-5.8-11.7-8C5.5 13.8 9.6 8 16 8s10.5 5.8 11.7 8c-1.2 2.2-5.3 8-11.7 8z"/>
              <path d="M16 10a6 6 0 1 0 0 12 6 6 0 0 0 0-12zm0 10a4 4 0 1 1 0-8 4 4 0 0 1 0 8z"/>
            </svg>
            <svg v-else focusable="false" width="16" height="16" viewBox="0 0 32 32" fill="currentColor">
              <path d="m3.3 2 26.7 26.7-1.4 1.4-5.1-5.1A15 15 0 0 1 16 26C7 26 2 16 2 16a25 25 0 0 1 6.2-7.5L1.9 3.4 3.3 2zm6.4 8A22.7 22.7 0 0 0 4.3 16C5.5 18.2 9.6 24 16 24c2.1 0 4-.6 5.7-1.5l-3-3A6 6 0 0 1 12.5 13l-2.8-3z"/>
              <path d="M16 6c9 0 14 10 14 10a24.9 24.9 0 0 1-4.6 6.1L24 20.7c1.7-1.6 3-3.5 3.7-4.7C26.5 13.8 22.4 8 16 8c-1.5 0-2.8.3-4.1.8L10.4 7.3A14 14 0 0 1 16 6z"/>
            </svg>
          </button>
        </span>

        <textarea
          v-else-if="componentTemplateFieldType(field) === 'textarea'"
          :value="fieldValues[componentTemplateFieldKey(field)]"
          class="bx--text-input component-template-textarea"
          :placeholder="fieldPlaceholder(field)"
          rows="6"
          @input="updateComponentTemplateField(field, eventValue($event))"
        ></textarea>

        <input
          v-else
          :value="fieldValues[componentTemplateFieldKey(field)]"
          class="bx--text-input"
          :type="componentTemplateFieldInputType(field)"
          :placeholder="fieldPlaceholder(field)"
          @input="updateComponentTemplateField(field, eventValue($event))"
        />
      </span>
    </label>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  componentTemplateFieldDefaultValue,
  componentTemplateFieldInputType,
  componentTemplateFieldKey,
  componentTemplateFieldLabel,
  componentTemplateFieldType,
  componentTemplateListItemFields,
  componentTemplateListRows as runtimeTemplateListRows,
  componentTemplateVisibleFields as runtimeTemplateVisibleFields,
  componentTemplateVisibleListItemFields,
  defaultComponentTemplateListRow,
} from '../views/componentConfigTemplateRuntime'

type TemplateField = Record<string, any>
type TemplateFieldOption = {
  value: string
  label: string
  target?: any
}

const props = defineProps<{
  fields: TemplateField[]
  fieldValues: Record<string, any>
  requiredForUser: (field: TemplateField) => boolean
  fieldHint: (field: TemplateField) => string
  fieldOptions: (field: TemplateField) => TemplateFieldOption[]
  fieldUsesTargetSelect: (field: TemplateField) => boolean
  fieldPlaceholder: (field: TemplateField) => string
}>()

const emit = defineEmits<{
  'update:fieldValues': [value: Record<string, any>]
}>()

const eventValue = (event: Event) => String((event.target as HTMLInputElement | HTMLSelectElement | null)?.value ?? '')
const eventChecked = (event: Event) => Boolean((event.target as HTMLInputElement | null)?.checked)
const visibleFields = computed(() => runtimeTemplateVisibleFields(props.fields))
const componentTemplateListRows = (field: TemplateField) => runtimeTemplateListRows(field, props.fieldValues)
const visiblePasswordKeys = ref<Set<string>>(new Set())

const updateComponentTemplateField = (field: TemplateField, value: any) => {
  emit('update:fieldValues', {
    ...props.fieldValues,
    [componentTemplateFieldKey(field)]: value,
  })
}
const componentTemplatePasswordVisible = (field: TemplateField) => visiblePasswordKeys.value.has(componentTemplateFieldKey(field))
const toggleComponentTemplatePassword = (field: TemplateField) => {
  const key = componentTemplateFieldKey(field)
  const next = new Set(visiblePasswordKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  visiblePasswordKeys.value = next
}
const updateComponentTemplateListRows = (field: TemplateField, rows: any[]) => {
  updateComponentTemplateField(field, rows)
}
const defaultComponentTemplateListRowWithOptions = (field: TemplateField) => {
  const row = defaultComponentTemplateListRow(field)
  for (const itemField of componentTemplateListItemFields(field)) {
    const options = props.fieldOptions(itemField)
    if (props.fieldUsesTargetSelect(itemField) && options.length > 0) {
      row[componentTemplateFieldKey(itemField)] = options[0].value
    }
  }
  return row
}

const addComponentTemplateListRow = (field: TemplateField) => {
  updateComponentTemplateListRows(field, [
    ...componentTemplateListRows(field),
    defaultComponentTemplateListRowWithOptions(field),
  ])
}

const removeComponentTemplateListRow = (field: TemplateField, index: number) => {
  updateComponentTemplateListRows(field, componentTemplateListRows(field).filter((_row:any, idx:number) => idx !== index))
}

const updateComponentTemplateListCell = (
  field: TemplateField,
  rowIndex: number,
  itemField: TemplateField,
  value: string,
) => {
  const itemKey = componentTemplateFieldKey(itemField)
  updateComponentTemplateListRows(field, componentTemplateListRows(field).map((row:any, idx:number) => (
    idx === rowIndex ? { ...row, [itemKey]: value } : row
  )))
}
</script>

<style scoped>
.component-template-field-list {
  display: grid;
  gap: 0;
  min-width: 0;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: var(--paap-panel);
}
.component-template-field {
  display: grid;
  grid-template-columns: minmax(128px, 0.32fr) minmax(0, 0.68fr);
  align-items: center;
  gap: 14px;
  min-width: 0;
  padding: 12px 0;
}
.component-template-field + .component-template-field {
  border-top: 1px solid var(--paap-border);
}
.component-template-field-label {
  display: grid;
  gap: 4px;
  min-width: 0;
  line-height: 1.35;
}
.component-template-field-label > span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.component-template-field em {
  padding: 1px 5px;
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-danger);
  font-size: var(--paap-fs-small);
  font-weight: 700;
  font-style: normal;
}
.component-template-field-label small {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 500;
  line-height: 1.35;
}
.component-template-control {
  display: block;
  min-width: 0;
}
.component-template-control .bx--text-input {
  width: 100%;
  min-width: 0;
  height: 40px;
  padding: 0 12px;
  border: 1px solid var(--paap-border-strong);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  font-size: var(--paap-fs-body);
  outline: none;
  transition: border-color 110ms, box-shadow 110ms;
}
.component-template-control .bx--select-input {
  width: 100%;
  min-width: 0;
  height: 40px;
  padding: 0 40px 0 12px;
  border: 1px solid var(--paap-border-strong);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  font-size: var(--paap-fs-body);
  outline: none;
  transition: border-color 110ms, box-shadow 110ms;
}
.component-template-control .bx--text-input:focus {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
}
.component-template-control .component-template-textarea {
  min-height: 120px;
  height: auto;
  padding-top: 10px;
  padding-bottom: 10px;
  resize: vertical;
  font-family: var(--paap-mono);
  line-height: 1.5;
}
.component-template-control .bx--select-input:focus {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
}
.password-field-wrap {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 40px;
  min-width: 0;
}
.password-field-wrap .password-field-input {
  border-right: 0;
}
.password-visible-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: 1px solid var(--paap-border-strong);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: pointer;
  transition: border-color 110ms, color 110ms, box-shadow 110ms;
}
.password-visible-toggle:hover {
  color: var(--paap-accent);
}
.password-field-wrap:focus-within .password-field-input,
.password-field-wrap:focus-within .password-visible-toggle {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
}
.component-template-list-control {
  display: grid;
  gap: 8px;
}
.component-template-list-row {
  display: grid;
  grid-template-columns: minmax(96px, 0.35fr) minmax(0, 0.65fr) auto;
  align-items: start;
  gap: 8px;
  min-width: 0;
  padding: 8px 0;
  border: 0;
  background: var(--paap-panel);
}
.component-template-list-textarea {
  min-height: 132px;
}
.component-template-list-actions {
  display: inline-grid;
  grid-template-columns: repeat(2, 28px);
  gap: 4px;
}
.component-template-list-actions button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--paap-border);
  border-radius: 0;
  background: var(--paap-panel);
  color: var(--paap-text);
  font: inherit;
  font-size: 15px;
  font-weight: 700;
  line-height: 1;
  cursor: pointer;
  transition: border-color 110ms, background 110ms, color 110ms;
}
.component-template-list-actions button:hover:not(:disabled) {
  border-color: var(--paap-accent);
  background: var(--paap-panel);
  color: var(--paap-accent);
}
.component-template-list-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.38;
}
.component-template-checkbox {
  display: inline-flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 32px;
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
}
.component-template-checkbox input {
  margin: 0;
}
@media (max-width: 760px) {
  .component-template-field {
    grid-template-columns: 1fr;
  }
  .component-template-list-row {
    grid-template-columns: 1fr auto;
  }
}
</style>
