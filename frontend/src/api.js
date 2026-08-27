export async function GetConfig() {
  const res = await fetch('/api/config')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function SaveConfig(config) {
  const res = await fetch('/api/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config)
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function RunSync(dryRun = false) {
  const res = await fetch(`/api/sync?dry_run=${dryRun}`, {
    method: 'POST'
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function GetLogs() {
  const res = await fetch('/api/logs')
  if (!res.ok) throw new Error(await res.text())
  const data = await res.json()
  return data.logs
}

export async function ClearLogs() {
  const res = await fetch('/api/logs/clear', {
    method: 'POST'
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function GetStatus() {
  const res = await fetch('/api/status')
  if (!res.ok) throw new Error(await res.text())
  const data = await res.json()
  return data.syncing // true или false
}

export async function GetVersion(force = false) {
  const res = await fetch(`/api/version${force ? '?force=true' : ''}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function Browse(path = '') {
  const res = await fetch(`/api/browse?path=${encodeURIComponent(path)}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export function subscribeEvents({ onLog, onStatus, onReset, onConnected, onDisconnected }) {
  let eventSource = null
  let stopped = false
  let wasConnected = false
  let reconnectTimer = null

  function connect() {
    if (stopped) return
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }

    eventSource = new EventSource('/api/events')

    eventSource.onopen = () => {
      if (onConnected) {
        onConnected({ isReconnect: wasConnected })
      }
      wasConnected = true
    }

    eventSource.addEventListener('log', (event) => {
      if (event.data && onLog) {
        onLog(event.data)
      }
    })

    eventSource.addEventListener('status', (event) => {
      if (event.data && onStatus) {
        try {
          const parsed = JSON.parse(event.data)
          onStatus(parsed.syncing)
        } catch (e) {
          console.warn('Failed to parse status event:', e)
        }
      }
    })

    eventSource.addEventListener('reset', () => {
      if (onReset) {
        onReset()
      }
    })

    eventSource.onerror = () => {
      if (onDisconnected) {
        onDisconnected()
      }
      if (eventSource) {
        eventSource.close()
        eventSource = null
      }
      if (!stopped) {
        clearTimeout(reconnectTimer)
        reconnectTimer = setTimeout(connect, 3000)
      }
    }
  }

  connect()

  return () => {
    stopped = true
    clearTimeout(reconnectTimer)
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }
}
