export const TEMPLATE_INSTALL_POLL_INTERVAL_MS = 2000
export const TEMPLATE_INSTALL_POLL_MAX_ATTEMPTS = 60

const environmentPollingStatuses = new Set(['creating', 'pending'])
const resourcePollingStatuses = new Set(['creating', 'installing', 'deploying', 'building', 'pending'])

function hasPendingResource(items: any[]) {
  return items.some((item) => resourcePollingStatuses.has(String(item?.status || '').toLowerCase()))
}

export function shouldPollTemplateInstallations(env: any, services: any[], components: any[] = []) {
  const status = String(env?.status || '').toLowerCase()
  const templateId = Number(env?.templateId || 0)
  if (environmentPollingStatuses.has(status)) return true
  if (hasPendingResource(services) || hasPendingResource(components)) return true
  if (templateId <= 0) return false
  if (services.length > 0) return false
  return status === 'running'
}
