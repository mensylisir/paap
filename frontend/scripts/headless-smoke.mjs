import { spawn } from 'node:child_process'
import { setTimeout as delay } from 'node:timers/promises'

const host = process.env.SMOKE_HOST || '127.0.0.1'
const port = Number(process.env.SMOKE_PORT || '5173')
const path = process.env.SMOKE_PATH || '/login'
const url = process.env.SMOKE_URL || `http://${host}:${port}${path}`
const expectedText = process.env.SMOKE_EXPECT || '应用管理平台'
const chromiumCandidates = [
  process.env.CHROMIUM_BIN,
  '/snap/bin/chromium',
  'chromium',
  'chromium-browser',
  'google-chrome',
  'google-chrome-stable',
].filter(Boolean)

let viteProcess = null

function run(command, args, options = {}) {
  return new Promise((resolve) => {
    const child = spawn(command, args, { stdio: ['ignore', 'pipe', 'pipe'], ...options })
    let stdout = ''
    let stderr = ''
    child.stdout?.on('data', (chunk) => { stdout += chunk.toString() })
    child.stderr?.on('data', (chunk) => { stderr += chunk.toString() })
    child.on('error', (error) => resolve({ code: 127, stdout, stderr: error.message }))
    child.on('close', (code) => resolve({ code, stdout, stderr }))
  })
}

async function httpOk(target) {
  try {
    const response = await fetch(target, { method: 'HEAD' })
    return response.ok
  } catch {
    return false
  }
}

async function waitForServer(target, timeoutMs = 15000) {
  const started = Date.now()
  while (Date.now() - started < timeoutMs) {
    if (await httpOk(target)) return true
    await delay(250)
  }
  return false
}

async function findChromium() {
  for (const candidate of chromiumCandidates) {
    const result = await run(candidate, ['--version'])
    if (result.code === 0) return candidate
  }
  return ''
}

async function ensureVite() {
  if (await httpOk(url)) return false

  viteProcess = spawn(
    'npm',
    ['run', 'dev', '--', '--host', host, '--port', String(port)],
    { stdio: ['ignore', 'pipe', 'pipe'], detached: true },
  )

  const ready = await waitForServer(url)
  if (!ready) {
    throw new Error(`Vite did not become ready at ${url}`)
  }
  return true
}

async function stopVite() {
  if (!viteProcess) return
  const pid = viteProcess.pid
  if (!pid) return

  const closed = new Promise((resolve) => {
    viteProcess.once('close', resolve)
  })
  try {
    process.kill(-pid, 'SIGTERM')
  } catch {
    try {
      viteProcess.kill('SIGTERM')
    } catch {
      return
    }
  }
  const timeout = delay(3000).then(() => 'timeout')
  if (await Promise.race([closed, timeout]) === 'timeout') {
    try {
      process.kill(-pid, 'SIGKILL')
    } catch {
      try {
        viteProcess.kill('SIGKILL')
      } catch {
        // already gone
      }
    }
    await Promise.race([closed, delay(1000)])
  }
}

async function main() {
  const chromium = await findChromium()
  if (!chromium) {
    throw new Error('No Chromium binary found. Set CHROMIUM_BIN or install chromium.')
  }

  const startedVite = await ensureVite()
  const result = await run(chromium, [
    '--headless=new',
    '--disable-gpu',
    '--no-sandbox',
    '--disable-dev-shm-usage',
    '--virtual-time-budget=1500',
    '--dump-dom',
    url,
  ])

  if (result.code !== 0) {
    throw new Error(`Chromium headless failed:\n${result.stderr || result.stdout}`)
  }
  if (!result.stdout.includes(expectedText)) {
    throw new Error(`Smoke page did not contain expected text "${expectedText}".`)
  }

  console.log(`Headless smoke passed: ${url}`)
  if (startedVite) console.log('Started and verified Vite without Xorg/headful browser.')
}

main()
  .catch((error) => {
    console.error(error.message)
    process.exitCode = 1
  })
  .finally(stopVite)
