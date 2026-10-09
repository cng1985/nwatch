<template>
  <div>
    <div class="page-head"><div><h1>HTTPS 证书</h1><p>按剩余天数升序，最先到期的证书排在最上面。</p></div></div>
    <div class="card table-wrap">
      <table>
        <thead><tr><th>域名</th><th>到期时间</th><th>剩余天数</th><th>颁发机构</th><th>状态</th></tr></thead>
        <tbody>
          <tr v-for="row in items" :key="row.id">
            <td><b>{{ row.name }}</b><small style="display:block;color:#7e8c98">{{ row.url || row.host }}</small></td>
            <td>{{ clock(row.certNotAfter) }}</td>
            <td><b>{{ row.certDaysRemaining ?? '-' }}</b></td>
            <td>{{ row.certIssuer || '-' }}</td>
            <td><span class="status" :class="row.tlsStatus === 'NORMAL' ? 'UP' : 'DOWN'">{{ statusText[row.tlsStatus] || '等待检测' }}</span></td>
          </tr>
          <tr v-if="!items.length"><td colspan="5" class="empty">还没有证书监控</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { demoCerts } from '../demo'
import { clock, isDemo, statusText } from '../format'
const items = ref([])
onMounted(async () => { items.value = isDemo() ? demoCerts : await api('/api/certificates') })
</script>
