<script setup>
const props = defineProps({
  workflows: {
    type: Array,
    required: true
  },
  selectedWorkflow: {
    type: String,
    default: ''
  },
  currentStatus: {
    type: Object,
    default: null
  }
})

defineEmits(['select'])

const dotState = (path) => {
  const cs = props.currentStatus
  if (!cs || !cs.workflow) return 'idle'
  const wf = cs.workflow
  if (wf.name !== path) return 'idle'
  const s = wf.status || (cs.running ? 'running' : 'idle')
  switch (s) {
    case 'pending':
      return 'pending'
    case 'running':
      return 'running'
    case 'failed':
      return 'failed'
    default:
      return 'idle'
  }
}

const dotClass = (path) => {
  const s = dotState(path)
  if (s === 'running') return ['running', 'animate-pulse']
  return s
}
</script>

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
        <span class="status-dot" :class="dotClass(wf.path)"></span>
        {{ wf.name }}
      </button>
    </div>
  </aside>
</template>

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

/* Status dot */
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 8px;
  vertical-align: middle;
  background: var(--text-muted);
  opacity: 0.9;
}
.status-dot.idle {
  background: var(--text-muted);
  opacity: 0.6;
}
.status-dot.pending {
  background: var(--status-pending);
}
.status-dot.running {
  background: var(--status-running);
}
.status-dot.failed {
  background: var(--status-failed);
}
</style>
