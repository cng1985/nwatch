<template>
  <div>
    <div class="page-head">
      <div>
        <h1>脚本执行</h1>
        <p>在服务器上立即执行命令，并查看输出。定时检查请到监控里选择脚本类型。</p>
      </div>
    </div>
    <div class="editor">
      <section class="card section">
        <h3><em>01</em> 执行</h3>
        <div class="form-grid">
          <label class="field"><span>名称</span><input v-model="form.name" class="text" placeholder="手动检查" /></label>
          <label class="field"><span>超时</span>
            <select v-model.number="form.timeout" class="select" style="width:100%">
              <option :value="5">5 秒</option><option :value="10">10 秒</option><option :value="30">30 秒</option><option :value="60">60 秒</option><option :value="120">120 秒</option>
            </select>
          </label>
          <label class="field full"><span class="req">脚本内容</span><textarea v-model="form.command" class="text" rows="6" placeholder="df -h /"></textarea></label>
          <label class="field full"><span>工作目录</span><input v-model="form.workDir" class="text" placeholder="/tmp" /></label>
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="head-actions" style="margin:4px 0 16px">
          <button class="btn primary" :disabled="running" @click="run">{{ running ? '执行中' : '立即执行' }}</button>
        </div>
      </section>
      <aside class="card preview">
        <h2>说明</h2>
        <p class="demo-note" style="line-height:1.7">命令通过 sh -c 执行，身份与 NMonitor 进程相同。超时后会结束脚本及其子进程。输出最多保留 16KB。执行记录保留 30 天。</p>
      </aside>
    </div>
    <div class="card table-wrap">
      <div class="card-title" style="padding:16px 16px 0">
        <h2>执行记录</h2>
        <button class="btn ghost" @click="load">刷新</button>
      </div>
      <table>
        <thead><tr><th>时间</th><th>名称</th><th>结果</th><th>退出码</th><th>耗时</th><th>命令</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id" class="clickable" :class="{ 'row-on': selected && selected.id === row.id }" @click="selected = row">
            <td>{{ clock(row.startedAt) }}</td>
            <td>{{ row.name || '手动执行' }}</td>
            <td><span class="status" :class="row.success ? 'UP' : 'DOWN'">{{ row.success ? '成功' : '失败' }}</span></td>
            <td>{{ row.exitCode }}</td>
            <td>{{ row.durationMs }} ms</td>
            <td>{{ firstLine(row.command) }}</td>
          </tr>
          <tr v-if="!items.length"><td colspan="6" class="empty">还没有执行记录</td></tr>
        </tbody>
      </table>
    </div>
    <div v-if="selected" class="card card-pad" style="margin-top:14px">
      <div class="card-title">
        <div><h2>{{ selected.name || '执行输出' }}</h2><p>{{ selected.errorMessage || (selected.success ? '执行成功' : '执行失败') }}</p></div>
      </div>
      <div class="log-box">{{ selected.stdout || '（没有标准输出）' }}</div>
      <div v-if="selected.stderr" class="log-box">{{ selected.stderr }}</div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { clock, isDemo } from '../format'

const form = reactive({ name: '', command: '', workDir: '', timeout: 10 })
const items = ref([])
const selected = ref(null)
const running = ref(false)
const error = ref('')

function firstLine(value) {
  return (value || '').split('\n')[0] || '-'
}

async function load() {
  if (isDemo()) {
    items.value = [{
      id: 1, name: '磁盘检查', command: 'df -h /', success: true, exitCode: 0, durationMs: 18,
      startedAt: new Date().toISOString(), stdout: '/dev/vda1  80G  48G  32G  61% /', stderr: '', errorMessage: '',
    }]
    return
  }
  const data = await api('/api/scripts/runs?pageSize=50')
  items.value = data.items || []
}

async function run() {
  error.value = ''
  if (isDemo()) { error.value = '演示模式不会执行脚本'; return }
  if (!form.command.trim()) { error.value = '请填写脚本内容'; return }
  running.value = true
  try {
    const row = await api('/api/scripts/run', { method: 'POST', body: { ...form } })
    selected.value = row
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    running.value = false
  }
}

onMounted(load)
</script>
