<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'

const products = ref([])
const error = ref('')
const msg = ref('')
const form = reactive({
  sku: '',
  name: '',
  description: '',
  unit_price: 10,
  initial_qty: 0,
})

async function load() {
  error.value = ''
  try {
    products.value = await api.products()
  } catch (e) {
    error.value = e.message
  }
}

async function createProduct() {
  error.value = ''
  msg.value = ''
  try {
    await api.createProduct({
      sku: form.sku,
      name: form.name,
      description: form.description,
      unit_price: Number(form.unit_price),
      initial_qty: Number(form.initial_qty) || 0,
    })
    msg.value = 'Product created'
    form.sku = ''
    form.name = ''
    form.description = ''
    form.unit_price = 10
    form.initial_qty = 0
    await load()
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
        <h2>Products / SKUs</h2>
        <span class="muted">{{ products.length }} items</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>SKU</th>
              <th>Name</th>
              <th>Description</th>
              <th>Unit price</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in products" :key="p.id">
              <td class="mono">{{ p.sku }}</td>
              <td>{{ p.name }}</td>
              <td class="muted">{{ p.description }}</td>
              <td class="mono">${{ p.unit_price.toFixed(2) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2>Add product</h2>
      </div>
      <form class="stack" @submit.prevent="createProduct">
        <div class="form-grid">
          <label>SKU <input v-model="form.sku" required placeholder="SKU-WIDGET-01" /></label>
          <label>Name <input v-model="form.name" required placeholder="Widget" /></label>
          <label>Unit price <input v-model.number="form.unit_price" type="number" min="0" step="0.01" required /></label>
          <label>Initial qty <input v-model.number="form.initial_qty" type="number" min="0" step="1" /></label>
        </div>
        <label>Description <input v-model="form.description" placeholder="Optional" /></label>
        <div class="actions">
          <button class="btn primary" type="submit">Create</button>
        </div>
      </form>
    </section>
  </div>
</template>
