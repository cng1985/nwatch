<template>
  <div class="shell">
    <header class="topbar">
      <div class="brand">
        <div class="brand-mark">Demo ERP</div>
        <h1>Sales & Inventory</h1>
        <p>Closed-loop demo: products → stock → order → fulfill</p>
      </div>
      <nav class="nav">
        <RouterLink to="/">Products</RouterLink>
        <RouterLink to="/inventory">Inventory</RouterLink>
        <RouterLink to="/orders">Sales Orders</RouterLink>
      </nav>
    </header>
    <RouterView />
  </div>
</template>
