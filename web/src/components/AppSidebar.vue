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
    
    <div class="actions">
      <div class="run-options" v-if="!isRunning">
        <!-- Dynamic Inputs -->
        <div v-for="(value, key) in inputs" :key="key" class="input-group">
          <label :for="key">{{ key }}</label>
          <input 
            :id="key"
            v-model="inputs[key]"
            type="text"
            class="input-field"
          >
        </div>

        <label class="checkbox-label">
          <input type="checkbox" v-model="skipPRCheck">
          Skip PR Wait
        </label>
      </div>
      <button 
        v-if="!isRunning"
        class="run-btn" 
        :disabled="!selectedWorkflow"
        @click="handleRun"
      >
        Run Workflow
      </button>
      <button 
        v-else
        class="stop-btn" 
        @click="$emit('stop')"
      >
        Stop Workflow
      </button>
    </div>

    <div class="settings-section">
      <div class="section-title">Settings</div>
      <div class="setting-item">
         <label for="log-level">Log Level</label>
         <select 
          id="log-level" 
          class="select-input" 
          :value="logLevel" 
          @change="$emit('change-log-level', $event.target.value)"
        >
           <option value="ERROR">Error</option>
           <option value="INFO">Info</option>
           <option value="DEBUG">Debug</option>
           <option value="TRACE">Trace</option>
         </select>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { ref, watch } from 'vue'

const skipPRCheck = ref(false)
const inputs = ref({})

const props = defineProps({
  workflows: {
    type: Array,
    required: true
  },
  selectedWorkflow: {
    type: String,
    default: ''
  },
  selectedDefinition: {
    type: Object,
    default: null
  },
  isRunning: {
    type: Boolean,
    default: false
  },
  logLevel: {
    type: String,
    default: 'INFO'
  }
})

const emit = defineEmits(['select', 'run', 'stop', 'change-log-level'])

watch(() => props.selectedDefinition, (newDef) => {
  if (newDef && newDef.inputs) {
    // Clone config inputs to local state
    inputs.value = { ...newDef.inputs }
  } else {
    inputs.value = {}
  }
}, { immediate: true })

const handleRun = () => {
  emit('run', { 
    skipPRCheck: skipPRCheck.value,
    inputs: inputs.value
  })
}
</script>

<style scoped>
.sidebar {
  width: 280px;
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
  margin-bottom: 24px;
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

.actions {
  padding-top: 24px;
  border-top: 1px solid var(--border-color);
}

.run-btn {
  width: 100%;
  padding: 10px;
  background: var(--status-success);
  color: white;
  border: none;
  border-radius: var(--radius-md);
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.2s;
}

.run-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.run-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  background: var(--text-muted);
}

.stop-btn {
  width: 100%;
  padding: 10px;
  background: #ef4444;
  color: white;
  border: none;
  border-radius: var(--radius-md);
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.2s;
}

.stop-btn:hover {
  opacity: 0.9;
}

.settings-section {
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--border-color);
}

.setting-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.setting-item label {
  font-size: 13px;
  color: var(--text-secondary);
}

.input-field {
  width: 100%;
  padding: 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
}

.select-input {
  width: 100%;
  padding: 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
}

.run-options {
  margin-bottom: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
}

.checkbox-label input {
  cursor: pointer;
}

.input-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.input-group label {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
}

</style>
