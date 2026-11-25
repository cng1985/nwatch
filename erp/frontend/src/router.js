import { createRouter, createWebHistory } from 'vue-router'
import ProductsView from './views/ProductsView.vue'
import InventoryView from './views/InventoryView.vue'
import OrdersView from './views/OrdersView.vue'
import OrderDetailView from './views/OrderDetailView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'products', component: ProductsView },
    { path: '/inventory', name: 'inventory', component: InventoryView },
    { path: '/orders', name: 'orders', component: OrdersView },
    { path: '/orders/:id', name: 'order-detail', component: OrderDetailView, props: true },
  ],
})

export default router
