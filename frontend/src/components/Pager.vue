<script setup lang="ts">
// The page buttons, above the table and below it: Previous, which page of
// how many, and Next. label names each for screen readers, which list them.
defineProps<{ page: number; pages: number; label: string }>()
const emit = defineEmits<{ page: [page: number] }>()
</script>

<template>
  <nav class="pager" :aria-label="label">
    <button type="button" class="secondary" :disabled="page <= 1" @click="emit('page', page - 1)">Previous</button>
    <span>Page {{ page }} of {{ pages }}</span>
    <button type="button" class="secondary" :disabled="page >= pages" @click="emit('page', page + 1)">Next</button>
  </nav>
</template>
