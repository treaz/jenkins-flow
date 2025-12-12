<template>
  <aside class="sidebar">
    <div class="section-title">Workflows</div>
    <div class="workflow-list">
      <button
        v-for="wf in workflows"
        :key="wf.path"
        class="workflow-btn"
        :class="{ active: selectedWorkflow === wf.path, invalid: !wf.valid }"
        @click="$emit('select', wf.path)"
      >
        <span class="workflow-icon" v-if="!wf.valid">⚠️</span>
        <span class="workflow-name">{{ wf.name }}</span>
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
  display: flex;
  align-items: center;
  gap: 8px;
}

.workflow-btn:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.workflow-btn.active {
  background: var(--accent);
  color: white;
}

.workflow-btn.invalid {
  color: #f59e0b;
}

.workflow-btn.invalid:hover {
  background: rgba(245, 158, 11, 0.1);
}

.workflow-icon {
  flex-shrink: 0;
  font-size: 16px;
}

.workflow-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
