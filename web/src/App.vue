<template>
  <div class="app-container">
    <header class="app-header">
      <div class="logo">
        <span class="icon">⚙️</span>
        <h1>Jenkins Flow</h1>
      </div>
      
      <div class="status-indicator" :class="{ running: isRunning }">
        <span class="dot"></span>
        {{ isRunning ? 'Running' : 'Ready' }}
      </div>
    </header>
    
    <main class="main-content">
      <aside class="sidebar">
        <div class="section-title">Workflows</div>
        <div class="workflow-list">
          <button
            v-for="wf in workflows"
            :key="wf.path"
            class="workflow-btn"
            :class="{ active: selectedWorkflow === wf.path }"
            @click="selectWorkflow(wf.path)"
          >
            {{ wf.name }}
          </button>
        </div>
        
        <div class="actions">
          <button 
            v-if="!isRunning"
            class="run-btn" 
            :disabled="!selectedWorkflow"
            @click="triggerRun"
          >
            Run Workflow
          </button>
          <button 
            v-else
            class="stop-btn" 
            @click="triggerStop"
          >
            Stop Workflow
          </button>
        </div>

        <div class="settings-section">
          <div class="section-title">Settings</div>
          <div class="setting-item">
             <label for="log-level">Log Level</label>
             <select id="log-level" class="select-input" :value="logLevel" @change="changeLogLevel">
               <option value="INFO">Info</option>
               <option value="DEBUG">Debug</option>
               <option value="TRACE">Trace</option>
             </select>
          </div>
        </div>
      </aside>
      
      <div class="content-area">
        <WorkflowView 
          v-if="currentStatus?.workflow" 
          :workflow="currentStatus.workflow" 
        />
        <div v-else-if="selectedWorkflow" class="workflow-preview">
          <h3>Ready to Run</h3>
          <p>Click "Run Workflow" to start {{ getWorkflowName(selectedWorkflow) }}</p>
        </div>
        <div v-else class="empty-selection">
          <p>Select a workflow from the sidebar</p>
        </div>
      </div>
    </main>
  </div>
  <ToastNotification ref="toast" />
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import WorkflowView from './components/WorkflowView.vue'
import ToastNotification from './components/ToastNotification.vue'
import { fetchWorkflows, fetchStatus, runWorkflow, stopWorkflow, fetchLogLevel, setLogLevel } from './api/client'

const workflows = ref([])
const selectedWorkflow = ref('')
const currentStatus = ref(null)
const isRunning = ref(false)
const logLevel = ref('INFO')
const pollTimer = ref(null)
const toast = ref(null)

const getWorkflowName = (path) => {
  const wf = workflows.value.find(w => w.path === path)
  return wf ? wf.name : path
}

const loadWorkflows = async () => {
  try {
    workflows.value = await fetchWorkflows()
    if (workflows.value.length > 0 && !selectedWorkflow.value) {
      selectedWorkflow.value = workflows.value[0].path
    }
  } catch (err) {
    console.error('Failed to load workflows:', err)
  }
}

const updateStatus = async () => {
  try {
    const status = await fetchStatus()
    currentStatus.value = status
    isRunning.value = status.running
    
    // If running, ensure we select the running workflow
    // Note: The backend state currently tracks the running workflow name/path
    // If we wanted to force selection:
    // if (status.running && status.workflow?.name) { ... }
  } catch (err) {
    console.error('Failed to update status:', err)
  }
}

const selectWorkflow = (path) => {
  selectedWorkflow.value = path
}

const triggerRun = async () => {
  if (!selectedWorkflow.value || isRunning.value) return
  
  try {
    await runWorkflow(selectedWorkflow.value)
    toast.value.add({
      title: 'Workflow Started',
      message: `Successfully started ${getWorkflowName(selectedWorkflow.value)}`,
      type: 'success'
    })
    // Immediate update
    await updateStatus()
  } catch (err) {
    toast.value.add({
      title: 'Execution Failed',
      message: err.message,
      type: 'error',
      duration: 8000
    })
  }
}

const triggerStop = async () => {
  try {
    await stopWorkflow()
     toast.value.add({
      title: 'Workflow Stopped',
      message: 'Stop signal sent to workflow',
      type: 'success'
    })
    // Immediate update
    await updateStatus()
  } catch (err) {
     toast.value.add({
      title: 'Stop Failed',
      message: err.message,
      type: 'error'
    })
  }
}

const changeLogLevel = async (e) => {
  const newLevel = e.target.value
  try {
    await setLogLevel(newLevel)
    logLevel.value = newLevel
    toast.value.add({
      title: 'Settings Updated',
      message: `Log level set to ${newLevel}`,
      type: 'success'
    })
  } catch (err) {
     toast.value.add({
      title: 'Update Failed',
      message: err.message,
      type: 'error'
    })
    // Revert on failure
    const current = await fetchLogLevel()
    logLevel.value = current.level
  }
}


onMounted(() => {
  loadWorkflows()
  updateStatus()
  
  // Load initial log level
  fetchLogLevel().then(data => {
    logLevel.value = data.level
  }).catch(err => console.error('Failed to load log level:', err))

  // Poll every 5 seconds
  pollTimer.value = setInterval(updateStatus, 5000)
})

onUnmounted(() => {
  if (pollTimer.value) clearInterval(pollTimer.value)
})
</script>

<style scoped>
.app-container {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.app-header {
  height: 64px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  background: var(--bg-secondary);
}

.logo {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo h1 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.logo .icon {
  font-size: 24px;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  padding: 6px 12px;
  background: var(--bg-primary);
  border-radius: 20px;
  border: 1px solid var(--border-color);
}

.status-indicator.running {
  color: var(--status-running);
  border-color: var(--status-running);
  background: var(--status-running-bg);
}

.status-indicator .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: currentColor;
}

.main-content {
  flex: 1;
  display: flex;
  height: calc(100vh - 64px);
}

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

.content-area {
  flex: 1;
  padding: 32px;
  overflow-y: auto;
  background: var(--bg-primary);
}

.workflow-preview, .empty-selection {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-secondary);
  text-align: center;
}

.workflow-preview h3 {
  color: var(--text-primary);
  margin-bottom: 8px;
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

.select-input {
  width: 100%;
  padding: 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
}
</style>
