<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'

const inventory = ref([])
const movements = ref([])
const error = ref('')

async function load() {
  error.value = ''
  try {
    ;[inventory.value, movements.value] = await Promise.all([api.inventory(), api.movements()])
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="stack">
    <div v-if="error" class="alert">{{ error }}</div>

    <section class="panel">
      <div class="panel-head">
        <h2>Stock levels</h2>
        <button class="btn" type="button" @click="load">Refresh</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>SKU</th>
              <th>Product</th>
              <th>On hand</th>
              <th>Updated</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in inventory" :key="row.id">
              <td class="mono">{{ row.product?.sku }}</td>
              <td>{{ row.product?.name }}</td>
              <td class="mono">{{ row.quantity }}</td>
              <td class="muted">{{ new Date(row.updated_at).toLocaleString() }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2>Recent stock movements</h2>
        <span class="muted">latest 100</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>SKU</th>
              <th>Type</th>
              <th>Qty Δ</th>
              <th>Order</th>
              <th>Note</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in movements" :key="m.id">
              <td class="muted">{{ new Date(m.created_at).toLocaleString() }}</td>
              <td class="mono">{{ m.product?.sku }}</td>
              <td><span class="badge">{{ m.type }}</span></td>
              <td class="mono">{{ m.quantity }}</td>
              <td class="mono">{{ m.order_id ?? '—' }}</td>
              <td class="muted">{{ m.note }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
