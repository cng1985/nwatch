async function request(path, options = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options,
  })
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { error: text }
  }
  if (!res.ok) {
    throw new Error(data?.error || res.statusText || 'request failed')
  }
  return data
}

export const api = {
  products: () => request('/api/products'),
  createProduct: (body) => request('/api/products', { method: 'POST', body: JSON.stringify(body) }),
  inventory: () => request('/api/inventory'),
  movements: () => request('/api/inventory/movements'),
  orders: () => request('/api/orders'),
  order: (id) => request(`/api/orders/${id}`),
  createOrder: (body) => request('/api/orders', { method: 'POST', body: JSON.stringify(body) }),
  confirmOrder: (id) => request(`/api/orders/${id}/confirm`, { method: 'POST' }),
  fulfillOrder: (id) => request(`/api/orders/${id}/fulfill`, { method: 'POST' }),
}
