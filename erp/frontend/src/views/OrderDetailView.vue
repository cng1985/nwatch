<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'

const props = defineProps({
  id: { type: [String, Number], required: true },
})

const order = ref(null)
const error = ref('')
const msg = ref('')
const busy = ref(false)

const canConfirm = computed(() => order.value?.status === 'draft')
const canFulfill = computed(() => order.value?.status === 'confirmed')

async function load() {
  error.value = ''
  try {
    order.value = await api.order(props.id)
  } catch (e) {
    error.value = e.message
  }
}

async function confirm() {
  busy.value = true
  error.value = ''
  msg.value = ''
  try {
    order.value = await api.confirmOrder(props.id)
    msg.value = 'Order confirmed — ready to fulfill'
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function fulfill() {
  busy.value = true
  error.value = ''
  msg.value = ''
  try {
    order.value = await api.fulfillOrder(props.id)
    msg.value = 'Order fulfilled — inventory deducted'
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="stack">
    <div v-if="error" class="alert">{{ error }}</div>
    <div v-if="msg" class="alert ok">{{ msg }}</div>

    <section class="panel" v-if="order">
      <div class="panel-head">
        <div>
          <h2>{{ order.order_no }}</h2>
          <p class="muted" style="margin: 4px 0 0">Order detail</p>
        </div>
        <div class="actions">
          <RouterLink class="btn" to="/orders">Back</RouterLink>
          <button class="btn primary" type="button" :disabled="!canConfirm || busy" @click="confirm">
            Confirm
          </button>
          <button class="btn danger" type="button" :disabled="!canFulfill || busy" @click="fulfill">
            Ship / Fulfill
          </button>
        </div>
      </div>

      <dl class="kv">
        <dt>Customer</dt>
        <dd>{{ order.customer_name }}</dd>
        <dt>Status</dt>
        <dd><span class="badge" :class="order.status">{{ order.status }}</span></dd>
        <dt>Total</dt>
        <dd class="mono">${{ order.total_amount.toFixed(2) }}</dd>
        <dt>Notes</dt>
        <dd>{{ order.notes || '—' }}</dd>
        <dt>Created</dt>
        <dd class="muted">{{ new Date(order.created_at).toLocaleString() }}</dd>
        <dt>Confirmed</dt>
        <dd class="muted">{{ order.confirmed_at ? new Date(order.confirmed_at).toLocaleString() : '—' }}</dd>
        <dt>Fulfilled</dt>
        <dd class="muted">{{ order.fulfilled_at ? new Date(order.fulfilled_at).toLocaleString() : '—' }}</dd>
      </dl>
    </section>

    <section class="panel" v-if="order">
      <div class="panel-head">
        <h2>Lines</h2>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>SKU</th>
              <th>Product</th>
              <th>Qty</th>
              <th>Unit</th>
              <th>Line total</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="line in order.lines" :key="line.id">
              <td class="mono">{{ line.product?.sku }}</td>
              <td>{{ line.product?.name }}</td>
              <td class="mono">{{ line.quantity }}</td>
              <td class="mono">${{ line.unit_price.toFixed(2) }}</td>
              <td class="mono">${{ line.line_total.toFixed(2) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="muted" style="margin-top: 12px">
        Demo tip: Confirm → Fulfill. Fulfill checks stock and writes <code>sale_out</code> movements.
      </p>
    </section>
  </div>
</template>
