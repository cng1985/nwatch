<template>
  <div>
    <div class="page-head"><div><h1>告警事件</h1><p>只在状态变化时记录，同一次故障不会重复通知。</p></div></div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>时间</th><th>监控</th><th>事件</th><th>状态</th><th>说明</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id">
            <td>{{ clock(row.occurredAt) }}</td><td>{{ row.monitorName }}</td>
            <td>{{ eventText[row.eventType] || row.eventType }}</td>
            <td>{{ row.oldStatus }} → {{ row.newStatus }}</td><td>{{ row.message }}</td>
          </tr>
          <tr v-if="!items.length"><td colspan="5" class="empty">还没有告警事件</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { demoEvents } from '../demo'
import { clock, eventText, isDemo } from '../format'
const items = ref([])
onMounted(async () => { items.value = isDemo() ? demoEvents : (await api('/api/events')).items || [] })
</script>
