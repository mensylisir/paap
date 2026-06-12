const STYLE_ID = 'paap-grafana-embed-compact-style'

const COMPACT_STYLE = `
body.app-grafana #pageContent {
  padding-left: 0 !important;
  width: 100% !important;
}

body.app-grafana .main-view > div {
  padding-top: 0 !important;
}

body.app-grafana .page-toolbar {
  margin-top: 0 !important;
}
`

const installCompactStyle = (doc: Document) => {
  const head = doc.head || doc.querySelector('head')
  if (!head || doc.getElementById(STYLE_ID)) return
  const style = doc.createElement('style')
  style.id = STYLE_ID
  style.textContent = COMPACT_STYLE
  head.appendChild(style)
}

const compactExploreOutline = (doc: Document) => {
  const view = doc.defaultView
  if (!view) return
  const outlineItem = doc.querySelector('button[aria-label="Queries"], button[aria-label="Logs"]')
  if (!(outlineItem instanceof view.HTMLElement)) return

  let wrapper: HTMLElement | null = outlineItem.parentElement
  let candidate: HTMLElement | null = null
  while (wrapper && wrapper !== doc.body) {
    const rect = wrapper.getBoundingClientRect()
    if (rect.width >= 120 && rect.width <= 220 && rect.height >= 200) {
      candidate = wrapper
    }
    wrapper = wrapper.parentElement
  }
  if (!candidate) return

  candidate.style.display = 'none'
  const next = candidate.nextElementSibling
  if (next instanceof view.HTMLElement) {
    next.style.flex = '1 1 auto'
    next.style.width = '100%'
    next.style.maxWidth = 'none'
  }
}

const applyCompactGrafanaEmbed = (frame: HTMLIFrameElement) => {
  let doc: Document | null = null
  try {
    doc = frame.contentDocument
  } catch {
    return
  }
  if (!doc) return
  installCompactStyle(doc)
  compactExploreOutline(doc)
}

export const compactGrafanaEmbed = (event: Event) => {
  const frame = event.currentTarget
  if (!(frame instanceof HTMLIFrameElement)) return
  applyCompactGrafanaEmbed(frame)
  frame.contentWindow?.requestAnimationFrame(() => applyCompactGrafanaEmbed(frame))
  window.setTimeout(() => applyCompactGrafanaEmbed(frame), 200)
  window.setTimeout(() => applyCompactGrafanaEmbed(frame), 800)
}
