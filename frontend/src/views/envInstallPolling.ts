export const TEMPLATE_INSTALL_POLL_INTERVAL_MS = 2000
export const TEMPLATE_INSTALL_POLL_MAX_ATTEMPTS = 12

export function shouldPollTemplateInstallations(env: any, services: any[]) {
  const status = String(env?.status || '').toLowerCase()
  const templateId = Number(env?.templateId || 0)
  if (templateId <= 0) return false
  if (services.length > 0) return false
  return status === 'creating' || status === 'pending' || status === 'running'
}
