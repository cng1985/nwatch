<template>
  <div>
    <div class="page-head">
      <div>
        <h1>用户与角色</h1>
        <p>按角色分配权限。一个用户可以同时拥有多个角色，实际权限是这些角色的并集。</p>
      </div>
    </div>

    <div class="page-head">
      <div><h2>用户</h2><p class="demo-note">停用后不能登录。至少保留一个可管理用户与角色的启用账号。</p></div>
      <button v-if="can('access.write')" class="btn primary" @click="openUser()">新增用户</button>
    </div>
    <div class="card table-wrap" style="margin-bottom:22px">
      <table>
        <thead><tr><th>用户名</th><th>显示名</th><th>角色</th><th>状态</th><th></th></tr></thead>
        <tbody>
          <tr v-for="row in users" :key="row.id">
            <td><b>{{ row.username }}</b></td>
            <td>{{ row.displayName || '-' }}</td>
            <td><span class="role-pills"><i v-for="role in row.roles" :key="role.id" class="pill">{{ role.name }}</i></span></td>
            <td><span class="status" :class="row.enabled ? 'UP' : 'PAUSED'">{{ row.enabled ? '启用' : '停用' }}</span></td>
            <td class="actions-inline">
              <template v-if="can('access.write')">
                <button @click="openUser(row)">编辑</button>
                <button class="danger" @click="removeUser(row)">删除</button>
              </template>
            </td>
          </tr>
          <tr v-if="!users.length"><td colspan="5" class="empty">还没有用户</td></tr>
        </tbody>
      </table>
    </div>

    <div class="page-head">
      <div><h2>角色</h2><p class="demo-note">管理员、运维、只读是内置角色。管理员始终拥有全部权限。</p></div>
      <button v-if="can('access.write')" class="btn primary" @click="openRole()">新增角色</button>
    </div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>角色</th><th>说明</th><th>权限</th><th>用户数</th><th></th></tr></thead>
        <tbody>
          <tr v-for="row in roles" :key="row.id">
            <td><b>{{ row.name }}</b><small v-if="row.builtin" style="display:block;color:#60707b">内置</small></td>
            <td>{{ row.description || '-' }}</td>
            <td>{{ (row.permissions || []).length }} 项</td>
            <td>{{ row.userCount ?? 0 }}</td>
            <td class="actions-inline">
              <template v-if="can('access.write')">
                <button @click="openRole(row)">编辑</button>
                <button v-if="!row.builtin" class="danger" @click="removeRole(row)">删除</button>
              </template>
            </td>
          </tr>
          <tr v-if="!roles.length"><td colspan="5" class="empty">还没有角色</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="userOpen" class="modal-mask" @click.self="userOpen = false">
      <form class="modal stack" @submit.prevent="saveUser">
        <h3>{{ userForm.id ? '编辑用户' : '新增用户' }}</h3>
        <label class="field"><span>用户名</span><input v-model="userForm.username" class="text" :disabled="!!userForm.id" placeholder="字母开头，2 到 32 位" autocomplete="off" /></label>
        <label class="field"><span>显示名</span><input v-model="userForm.displayName" class="text" placeholder="页面上显示的名字" /></label>
        <label class="field"><span>{{ userForm.id ? '重置密码' : '密码' }}</span><input v-model="userForm.password" class="text" type="password" autocomplete="new-password" :placeholder="userForm.id ? '留空表示不修改' : '至少 6 位'" /></label>
        <label class="check"><input v-model="userForm.enabled" type="checkbox" /> 启用</label>
        <div>
          <div class="demo-note" style="margin-bottom:8px">角色</div>
          <div class="perm-grid">
            <label v-for="role in roles" :key="role.id" class="check"><input v-model="userForm.roleIds" type="checkbox" :value="role.id" /> {{ role.name }}</label>
          </div>
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="head-actions"><button class="btn ghost" type="button" @click="userOpen = false">取消</button><button class="btn primary">保存</button></div>
      </form>
    </div>

    <div v-if="roleOpen" class="modal-mask" @click.self="roleOpen = false">
      <form class="modal wide stack" @submit.prevent="saveRole">
        <h3>{{ roleForm.id ? '编辑角色' : '新增角色' }}</h3>
        <label class="field"><span>名称</span><input v-model="roleForm.name" class="text" :disabled="roleForm.builtin" placeholder="例如 值班" /></label>
        <label class="field"><span>说明</span><input v-model="roleForm.description" class="text" placeholder="这个角色可以做什么" /></label>
        <div v-for="group in catalog" :key="group.key" class="perm-group">
          <h4>{{ group.label }}</h4>
          <div class="perm-grid">
            <label v-for="item in group.items" :key="item.key" class="check">
              <input type="checkbox" :checked="!!selected[item.key]" :disabled="roleForm.locked" @change="setPerm(item.key, $event.target.checked)" />
              {{ item.label }}
            </label>
          </div>
        </div>
        <p v-if="roleForm.locked" class="demo-note">管理员角色始终拥有全部权限。</p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="head-actions"><button class="btn ghost" type="button" @click="roleOpen = false">取消</button><button class="btn primary">保存</button></div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { isDemo } from '../format'
import { can, loadSession, session } from '../session'

const demoCatalog = [
  { key: 'overview', label: '总览', items: [{ key: 'dashboard.read', label: '查看总览' }] },
  { key: 'monitor', label: '监控', items: [
    { key: 'monitor.read', label: '查看监控' }, { key: 'monitor.write', label: '管理监控' },
    { key: 'cert.read', label: '查看证书' }, { key: 'group.read', label: '查看分组' }, { key: 'group.write', label: '管理分组' },
  ] },
  { key: 'host', label: '主机', items: [
    { key: 'host.read', label: '查看主机资源' }, { key: 'service.read', label: '查看系统服务' }, { key: 'service.write', label: '操作系统服务' },
    { key: 'script.read', label: '查看脚本记录' }, { key: 'script.write', label: '执行脚本' },
    { key: 'cron.read', label: '查看定时任务' }, { key: 'cron.write', label: '管理定时任务' }, { key: 'shell.use', label: '使用在线终端' },
  ] },
  { key: 'alert', label: '告警', items: [
    { key: 'event.read', label: '查看告警' }, { key: 'notifier.read', label: '查看通知渠道' }, { key: 'notifier.write', label: '管理通知渠道' },
  ] },
  { key: 'system', label: '系统', items: [
    { key: 'log.read', label: '查看运行日志' }, { key: 'settings.read', label: '查看系统配置' }, { key: 'settings.write', label: '修改系统配置' },
    { key: 'access.read', label: '查看用户与角色' }, { key: 'access.write', label: '管理用户与角色' },
  ] },
]
const allPerms = demoCatalog.flatMap((group) => group.items.map((item) => item.key))
const demoRoles = [
  { id: 1, key: 'admin', name: '管理员', builtin: true, description: '管理系统配置、用户、角色，以及全部监控和主机操作。', permissions: allPerms, userCount: 1 },
  { id: 2, key: 'operator', name: '运维', builtin: true, description: '管理监控、通知、脚本、定时任务和系统服务。', permissions: allPerms.filter((key) => !['shell.use', 'settings.write', 'access.read', 'access.write'].includes(key)), userCount: 0 },
  { id: 3, key: 'viewer', name: '只读', builtin: true, description: '查看监控、主机、告警和运行日志。', permissions: allPerms.filter((key) => key.endsWith('.read') && !key.startsWith('settings') && !key.startsWith('access')), userCount: 0 },
]
const demoUsers = [
  { id: 1, username: 'admin', displayName: '管理员', enabled: true, roles: [{ id: 1, name: '管理员' }] },
]

const users = ref([])
const roles = ref([])
const catalog = ref(demoCatalog)
const error = ref('')
const userOpen = ref(false)
const roleOpen = ref(false)
const userForm = reactive({ id: 0, username: '', displayName: '', password: '', enabled: true, roleIds: [] })
const roleForm = reactive({ id: 0, name: '', description: '', builtin: false, locked: false })
const selected = reactive({})

async function load() {
  if (isDemo()) {
    users.value = demoUsers
    roles.value = demoRoles
    catalog.value = demoCatalog
    return
  }
  const [userRows, roleRows, groups] = await Promise.all([
    api('/api/access/users'),
    api('/api/access/roles'),
    api('/api/access/permissions'),
  ])
  users.value = userRows || []
  roles.value = roleRows || []
  catalog.value = groups || []
}

function openUser(row) {
  error.value = ''
  Object.assign(userForm, {
    id: row?.id || 0,
    username: row?.username || '',
    displayName: row?.displayName || '',
    password: '',
    enabled: row ? row.enabled : true,
    roleIds: (row?.roles || []).map((item) => item.id),
  })
  userOpen.value = true
}

async function saveUser() {
  if (isDemo()) { error.value = '演示模式不会写入服务器'; return }
  const body = { username: userForm.username, displayName: userForm.displayName, enabled: userForm.enabled, roleIds: userForm.roleIds }
  if (userForm.password) body.password = userForm.password
  try {
    if (userForm.id) await api('/api/access/users/' + userForm.id, { method: 'PUT', body })
    else await api('/api/access/users', { method: 'POST', body })
    userOpen.value = false
    await load()
    if (userForm.username === session.username) await loadSession()
  } catch (err) { error.value = err.message }
}

async function removeUser(row) {
  if (isDemo() || !confirm('删除用户「' + row.username + '」？')) return
  try {
    await api('/api/access/users/' + row.id, { method: 'DELETE' })
    await load()
  } catch (err) { alert(err.message) }
}

function clearSelected() {
  for (const key of Object.keys(selected)) delete selected[key]
}

function openRole(row) {
  error.value = ''
  clearSelected()
  const locked = row?.key === 'admin'
  for (const key of locked ? allKeys() : (row?.permissions || [])) selected[key] = true
  Object.assign(roleForm, {
    id: row?.id || 0,
    name: row?.name || '',
    description: row?.description || '',
    builtin: !!row?.builtin,
    locked,
  })
  roleOpen.value = true
}

function allKeys() {
  return catalog.value.flatMap((group) => group.items.map((item) => item.key))
}

function setPerm(key, on) {
  if (roleForm.locked) return
  selected[key] = on
  if (on && key.endsWith('.write')) {
    const read = key.replace(/\.write$/, '.read')
    if (allKeys().includes(read)) selected[read] = true
  }
  if (!on && key.endsWith('.read')) selected[key.replace(/\.read$/, '.write')] = false
}

async function saveRole() {
  if (isDemo()) { error.value = '演示模式不会写入服务器'; return }
  const permissions = allKeys().filter((key) => selected[key])
  const body = { name: roleForm.name, description: roleForm.description, permissions }
  try {
    if (roleForm.id) await api('/api/access/roles/' + roleForm.id, { method: 'PUT', body })
    else await api('/api/access/roles', { method: 'POST', body })
    roleOpen.value = false
    await load()
    await loadSession()
  } catch (err) { error.value = err.message }
}

async function removeRole(row) {
  if (isDemo() || !confirm('删除角色「' + row.name + '」？')) return
  try {
    await api('/api/access/roles/' + row.id, { method: 'DELETE' })
    await load()
  } catch (err) { alert(err.message) }
}

onMounted(load)
</script>
