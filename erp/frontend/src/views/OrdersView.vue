<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const orders = ref([])
const products = ref([])
const error = ref('')
const msg = ref('')

const form = reactive({
  customer_name: '',
  notes: '',
  lines: [{ product_id: '', quantity: 1 }],
})

const productOptions = computed(() => products.value)

async function load() {
  error.value = ''
  try {
    ;[orders.value, products.value] = await Promise.all([api.orders(), api.products()])
    if (!form.lines[0].product_id && products.value.length) {
      form.lines[0].product_id = String(products.value[0].id)
    }
  } catch (e) {
    error.value = e.message
  }
}

function addLine() {
  const first = products.value[0]
  form.lines.push({ product_id: first ? String(first.id) : '', quantity: 1 })
}

function removeLine(i) {
  if (form.lines.length === 1) return
  form.lines.splice(i, 1)
}

async function createOrder() {
  error.value = ''
  msg.value = ''
  try {
    const body = {
      customer_name: form.customer_name,
      notes: form.notes,
      lines: form.lines.map((l) => ({
        product_id: Number(l.product_id),
        quantity: Number(l.quantity),
      })),
    }
    const order = await api.createOrder(body)
    msg.value = `Created ${order.order_no}`
    form.customer_name = ''
    form.notes = ''
    form.lines = [{ product_id: products.value[0] ? String(products.value[0].id) : '', quantity: 1 }]
    await load()
    router.push(`/orders/${order.id}`)
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="stack">
    <div v-if="error" class="alert">{{ error }}</div>
    <div v-if="msg" class="alert ok">{{ msg }}</div>

    <section class="panel">
      <div class="panel-head">
        <h2>Sales orders</h2>
        <button class="btn" type="button" @click="load">Refresh</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Order #</th>
              <th>Customer</th>
              <th>Status</th>
              <th>Total</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="o in orders" :key="o.id">
              <td>
                <RouterLink class="mono" :to="`/orders/${o.id}`">{{ o.order_no }}</RouterLink>
              </td>
              <td>{{ o.customer_name }}</td>
              <td><span class="badge" :class="o.status">{{ o.status }}</span></td>
              <td class="mono">${{ o.total_amount.toFixed(2) }}</td>
              <td class="muted">{{ new Date(o.created_at).toLocaleString() }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2>Create order</h2>
      </div>
      <form class="stack" @submit.prevent="createOrder">
        <div class="form-grid">
          <label>Customer <input v-model="form.customer_name" required placeholder="Northwind Retail" /></label>
          <label>Notes <input v-model="form.notes" placeholder="Optional" /></label>
        </div>

        <div v-for="(line, i) in form.lines" :key="i" class="form-grid">
          <label>
            Product
            <select v-model="line.product_id" required>
              <option v-for="p in productOptions" :key="p.id" :value="String(p.id)">
                {{ p.sku }} — {{ p.name }}
              </option>
            </select>
          </label>
          <label>
            Qty
            <input v-model.number="line.quantity" type="number" min="1" required />
          </label>
          <div class="actions" style="align-self: end">
            <button class="btn" type="button" @click="removeLine(i)" :disabled="form.lines.length === 1">Remove</button>
          </div>
        </div>

        <div class="actions">
          <button class="btn" type="button" @click="addLine">Add line</button>
          <button class="btn primary" type="submit">Create draft</button>
        </div>
      </form>
    </section>
  </div>
</template>
