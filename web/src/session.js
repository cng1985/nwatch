import { reactive } from 'vue'
import { api } from './api'
import { isDemo } from './format'

export const session = reactive({
  username: '',
  displayName: '',
  roles: [],
  permissions: [],
  loaded: false,
})

export function can(perm) {
  if (isDemo()) return true
  if (session.permissions.includes(perm)) return true
  if (perm.endsWith('.read')) return session.permissions.includes(perm.replace(/\.read$/, '.write'))
  return false
}

export function applySession(data = {}) {
  session.username = data.username || ''
  session.displayName = data.displayName || data.username || ''
  session.roles = data.roles || []
  session.permissions = data.permissions || []
  session.loaded = true
  if (session.username) localStorage.setItem('nmonitor_user', session.username)
}

export async function loadSession() {
  if (isDemo()) {
    applySession({ username: 'admin', displayName: '管理员', roles: [{ name: '管理员' }], permissions: [] })
    return session
  }
  applySession(await api('/api/auth/me'))
  return session
}

export function clearSession() {
  session.username = ''
  session.displayName = ''
  session.roles = []
  session.permissions = []
  session.loaded = false
}
