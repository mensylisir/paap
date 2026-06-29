export type CreateEnvironmentForm = {
  name: string
  identifier: string
  mode: string
  templateId: string
  additionalNamespacesInput: string
  sharedResourceIds: string[]
}

export type SharedCapabilityResource = {
  id: number | string
  capability?: string
  provider?: string
  serviceType?: string
  serviceName?: string
  status?: string
}

export function emptyEnvironmentForm(defaultTemplateId: number | string = 1): CreateEnvironmentForm {
  return {
    name: '',
    identifier: '',
    mode: 'empty',
    templateId: String(defaultTemplateId || 1),
    additionalNamespacesInput: '',
    sharedResourceIds: [],
  }
}

export function buildSharedCapabilityPayload(selectedIds: string[], resources: SharedCapabilityResource[]) {
  const selected = new Set((selectedIds || []).map(String))
  return (resources || [])
    .filter((resource) => selected.has(String(resource.id)))
    .map((resource) => ({
      source: 'shared',
      capability: resource.capability,
      capabilityKey: `shared-${resource.capability}-${resource.id}`,
      provider: resource.provider,
      serviceType: resource.serviceType,
      refServiceId: Number(resource.id),
    }))
}
