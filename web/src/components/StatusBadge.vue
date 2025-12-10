<template>
  <span class="status-badge" :class="statusClass">
    <span v-if="status === 'running'" class="spinner"></span>
    <span class="icon" v-else>{{ statusIcon }}</span>
    <span class="label">{{ label || status }}</span>
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: {
    type: String,
    required: true,
    validator: (v) => ['pending', 'running', 'success', 'failed', 'skipped'].includes(v)
  },
  label: String
})

const statusClass = computed(() => `status-${props.status}`)

const statusIcon = computed(() => {
  switch (props.status) {
    case 'success': return '✓'
    case 'failed': return '✗'
    case 'skipped': return '⊘'
    case 'pending': return '○'
    default: return ''
  }
})
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.status-pending {
  background: var(--status-pending-bg);
  color: var(--status-pending);
}

.status-running {
  background: var(--status-running-bg);
  color: var(--status-running);
}

.status-success {
  background: var(--status-success-bg);
  color: var(--status-success);
}

.status-failed {
  background: var(--status-failed-bg);
  color: var(--status-failed);
}

.status-skipped {
  background: var(--status-pending-bg);
  color: var(--text-muted);
}

.icon {
  font-size: 14px;
}

.spinner {
  width: 12px;
  height: 12px;
  border: 2px solid transparent;
  border-top-color: currentColor;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
