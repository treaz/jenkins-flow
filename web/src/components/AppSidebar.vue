<template>
  <aside class="sidebar">
    <div class="section-title">Workflows</div>
    <div class="workflow-list">
      <button
        v-for="wf in workflows"
        :key="wf.path"
        class="workflow-btn"
        :class="{ active: selectedWorkflow === wf.path }"
        @click="$emit('select', wf.path)"
      >
        {{ wf.name }}
      </button>
    </div>
  </aside>
</template>

<script setup>
defineProps({
  workflows: {
    type: Array,
    required: true
  },
  selectedWorkflow: {
    type: String,
    default: ''
  }
})

defineEmits(['select'])
</script>

<style scoped>
.sidebar {
  width: 250px;
  border-right: 1px solid var(--border-color);
  background: var(--bg-secondary);
  display: flex;
  flex-direction: column;
  padding: 24px;
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 16px;
  letter-spacing: 0.5px;
}

.workflow-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  overflow-y: auto;
}

.workflow-btn {
  text-align: left;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s;
}

.workflow-btn:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.workflow-btn.active {
  background: var(--accent);
  color: white;
}
</style>
