export type ComponentConfigFileRow = {
  name: string
  configMapName: string
  key: string
  mountPath: string
  readOnly?: boolean
}

export type ComponentConfigTemplateFileRow = {
  name: string
  configMapName: string
  key: string
  recommendedMountPath: string
  readOnly?: boolean
}

const text = (value:any) => String(value ?? '').trim()

export const normalizeComponentTemplateFiles = (items:any[]): ComponentConfigTemplateFileRow[] => Array.isArray(items)
  ? items.map((item:any) => ({
    name: text(item?.name),
    configMapName: text(item?.configMapName),
    key: text(item?.key),
    recommendedMountPath: text(item?.recommendedMountPath || item?.mountPath),
    readOnly: item?.readOnly !== false,
  })).filter((item:ComponentConfigTemplateFileRow) => item.key)
  : []

const sameConfigFileIdentity = (left:Partial<ComponentConfigFileRow>, right:Partial<ComponentConfigFileRow>) => {
  const leftConfig = text(left.configMapName)
  const rightConfig = text(right.configMapName)
  const leftKey = text(left.key)
  const rightKey = text(right.key)
  if (leftConfig && rightConfig && leftKey && rightKey && leftConfig === rightConfig && leftKey === rightKey) return true
  const leftName = text(left.name)
  const rightName = text(right.name)
  if (leftName && rightName && leftName === rightName && leftKey && rightKey && leftKey === rightKey) return true
  const leftMount = text(left.mountPath)
  const rightMount = text(right.mountPath)
  return Boolean(leftMount && rightMount && leftMount === rightMount)
}

export const mergeComponentConfigFile = (
  files: ComponentConfigFileRow[],
  file: ComponentConfigFileRow,
): ComponentConfigFileRow[] => {
  const idx = files.findIndex((item) => sameConfigFileIdentity(item, file))
  if (idx < 0) return [...files, file]
  const next = [...files]
  next.splice(idx, 1, { ...files[idx], ...file })
  return next
}

export const componentTemplateFileMountPath = ({
  templateFile,
  configMapName,
  key,
  existingFiles,
  render,
}: {
  templateFile: ComponentConfigTemplateFileRow | Record<string, any>
  configMapName: string
  key: string
  existingFiles: ComponentConfigFileRow[]
  render: (value:string) => string
}) => {
  const existing = existingFiles.find((item) => sameConfigFileIdentity(item, {
    name: text(templateFile.name),
    configMapName,
    key,
    mountPath: '',
  }))
  if (existing?.mountPath) return existing.mountPath
  return render(text(templateFile.recommendedMountPath || (templateFile as Record<string, any>).mountPath))
}
