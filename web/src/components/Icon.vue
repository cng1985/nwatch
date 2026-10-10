<template>
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path v-if="name === 'wave'" d="M3 12h3l2.2-6 3.2 12 2.4-7 2 4H21" />
    <path v-else-if="name === 'home'" d="M4 10.5 12 4l8 6.5V20a1 1 0 0 1-1 1h-5v-6H10v6H5a1 1 0 0 1-1-1z" />
    <path v-else-if="name === 'list'" d="M8 6h12M8 12h12M8 18h12M4 6h.01M4 12h.01M4 18h.01" />
    <path v-else-if="name === 'cert'" d="M7 3h10a1 1 0 0 1 1 1v16l-6-3-6 3V4a1 1 0 0 1 1-1z" />
    <path v-else-if="name === 'bell'" d="M6 9a6 6 0 1 1 12 0c0 7 2 7 2 7H4s2 0 2-7M10 19a2 2 0 0 0 4 0" />
    <path v-else-if="name === 'chat'" d="M5 6h14v9H8l-3 3z" />
    <path v-else-if="name === 'ticket'" d="M5 7h14v4a2 2 0 0 0 0 4v4H5v-4a2 2 0 0 0 0-4z" />
    <path v-else-if="name === 'folder'" d="M3 7h6l2 2h10v9a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1z" />
    <path v-else-if="name === 'settings'" d="M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M4 12h2M18 12h2M12 4v2M12 18v2" />
    <path v-else-if="name === 'user'" d="M12 12a4 4 0 1 0-4-4 4 4 0 0 0 4 4M5 20a7 7 0 0 1 14 0" />
    <path v-else-if="name === 'logout'" d="M10 7V5a1 1 0 0 1 1-1h8v16h-8a1 1 0 0 1-1-1v-2M4 12h10M11 9l3 3-3 3" />
    <path v-else-if="name === 'plus'" d="M12 5v14M5 12h14" />
    <path v-else-if="name === 'globe'" d="M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18M3 12h18M12 3c2.5 2.8 2.5 15.2 0 18M12 3c-2.5 2.8-2.5 15.2 0 18" />
    <path v-else-if="name === 'lock'" d="M8 10V8a4 4 0 0 1 8 0v2M7 10h10v9H7z" />
    <path v-else-if="name === 'db'" d="M12 3c5 0 8 1.3 8 3s-3 3-8 3-8-1.3-8-3 3-3 8-3M4 6v6c0 1.7 3 3 8 3s8-1.3 8-3V6M4 12v6c0 1.7 3 3 8 3s8-1.3 8-3v-6" />
    <path v-else-if="name === 'file'" d="M7 3h7l5 5v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1zM14 3v5h5" />
    <path v-else-if="name === 'eye'" d="M2 12s4-6 10-6 10 6 10 6-4 6-10 6S2 12 2 12M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6" />
    <path v-else-if="name === 'check'" d="m5 12 5 5L20 7" />
    <path v-else-if="name === 'alert'" d="M12 9v4M12 17h.01M10.3 4.8 2.8 18a2 2 0 0 0 1.7 3h15a2 2 0 0 0 1.7-3L13.7 4.8a2 2 0 0 0-3.4 0z" />
    <path v-else-if="name === 'search'" d="m21 21-4.3-4.3M11 18a7 7 0 1 1 0-14 7 7 0 0 1 0 14" />
    <path v-else-if="name === 'bulb'" d="M9 18h6M10 21h4M8 14a6 6 0 1 1 8 0c-.8.8-2 2-2 3H10c0-1-1.2-2.2-2-3z" />
    <path v-else-if="name === 'cpu'" d="M8 8h8v8H8zM9 4v4M12 4v4M15 4v4M9 16v4M12 16v4M15 16v4M4 9h4M4 12h4M4 15h4M16 9h4M16 12h4M16 15h4" />
    <path v-else-if="name === 'mem'" d="M5 7h14v10H5zM8 7v10M12 7v10M16 7v10" />
    <path v-else-if="name === 'disk'" d="M4 7h16v10H4zM4 12h16M8 15h2" />
    <path v-else-if="name === 'script'" d="M5 4h14v16H5zM8 8l3 3-3 3M13 15h3" />
    <path v-else-if="name === 'terminal'" d="M4 6h16v12H4zM7 10l3 2-3 2M12 14h5" />
  </svg>
</template>

<script setup>
defineProps({ name: String })
</script>
