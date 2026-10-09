<template>
  <div>
    <div class="page-head"><div><h1>通知日志</h1><p>每次发送都会留下结果，方便核对钉钉和企业微信的返回。</p></div></div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>时间</th><th>渠道</th><th>事件</th><th>结果</th><th>错误</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id"><td>{{ clock(row.sentAt) }}</td><td>{{ row.notifierName }}</td><td>{{ eventText[row.eventType] || row.eventType }}</td><td>{{ row.success ? '成功' : '失败' }}</td><td>{{ row.errorMessage }}</td></tr>
          <tr v-if="!items.length"><td colspan="5" class="empty">还没有通知日志</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { clock, eventText, isDemo } from '../format'
const items = ref([])
onMounted(async () => { if (!isDemo()) items.value = (await api('/api/notification-logs')).items || [] })
</script>
