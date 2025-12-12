<template>
  <div class="app-container">
    <AppHeader 
      :is-running="isRunning" 
      @open-settings="isSettingsModalOpen = true"
    />
    
    <main class="main-content">
      <AppSidebar 
        :workflows="workflows"
        :selected-workflow="selectedWorkflow"
        :current-status="currentStatus"
        @select="selectWorkflow"
      />
      
      <div class="content-area">
        <WorkflowView 
          v-if="displayWorkflow" 
          :workflow="displayWorkflow" 
          :is-running="isRunning"
          @run="openRunModal"
          @stop="triggerStop"
        />
        <div v-else-if="selectedWorkflow" class="workflow-preview">
          <h3>Loading Workflow</h3>
          <p>Fetching steps for {{ getWorkflowName(selectedWorkflow) }}...</p>
        </div>
        <div v-else class="empty-selection">
          <p>Select a workflow from the sidebar</p>
        </div>
      </div>
    </main>
    
    <RunWorkflowModal
      :is-open="isRunModalOpen"
      :inputs="workflowInputs"
      :is-loading="isStartingRun"
      @close="isRunModalOpen = false"
      @run="handleRunSubmit"
    />
    
    <SettingsModal
      :is-open="isSettingsModalOpen"
      :log-level="logLevel"
      @close="isSettingsModalOpen = false"
      @change-log-level="changeLogLevel"
    />
    
    <ToastNotification ref="toast" />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import WorkflowView from './components/WorkflowView.vue'
import ToastNotification from './components/ToastNotification.vue'
import AppHeader from './components/AppHeader.vue'
import AppSidebar from './components/AppSidebar.vue'
import RunWorkflowModal from './components/RunWorkflowModal.vue'
import SettingsModal from './components/SettingsModal.vue'
import { fetchWorkflows, fetchStatus, runWorkflow, stopWorkflow, fetchLogLevel, setLogLevel, fetchWorkflowDefinition } from './api/client'

const workflows = ref([])
const selectedWorkflow = ref('')
const currentStatus = ref(null)
const workflowDefinitions = ref({})
const isRunning = ref(false)
const logLevel = ref('INFO')
const pollTimer = ref(null)
const toast = ref(null)
const pendingDefinitions = new Set()

// Modal states
const isRunModalOpen = ref(false)
const isSettingsModalOpen = ref(false)
const isStartingRun = ref(false)

const displayWorkflow = computed(() => {
  const selected = selectedWorkflow.value
  if (!selected) return null

  const statusWorkflow = currentStatus.value?.workflow
  if (statusWorkflow && statusWorkflow.name === selected) {
    return statusWorkflow
  }

  return workflowDefinitions.value[selected] || null
})

const workflowInputs = computed(() => {
  if (!displayWorkflow.value) return {}
  return displayWorkflow.value.inputs || {}
})

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
    if (selectedWorkflow.value) {
      await loadWorkflowDefinition(selectedWorkflow.value)
    }
  } catch (err) {
    console.error('Failed to load workflows:', err)
  }
}

const loadWorkflowDefinition = async (path) => {
  if (!path) return
  if (workflowDefinitions.value[path] || pendingDefinitions.has(path)) return

  pendingDefinitions.add(path)
  try {
    const definition = await fetchWorkflowDefinition(path)
    workflowDefinitions.value = {
      ...workflowDefinitions.value,
      [path]: definition
    }
  } catch (err) {
    console.error('Failed to load workflow definition:', err)
  } finally {
    pendingDefinitions.delete(path)
  }
}

const updateStatus = async () => {
  try {
    const status = await fetchStatus()
    currentStatus.value = status
    isRunning.value = status.running
  } catch (err) {
    console.error('Failed to update status:', err)
  }
}

const selectWorkflow = (path) => {
  selectedWorkflow.value = path
  loadWorkflowDefinition(path)
}

const openRunModal = () => {
  isRunModalOpen.value = true
}

const handleRunSubmit = async (options) => {
  isStartingRun.value = true
  try {
    await runWorkflow(selectedWorkflow.value, options)
    isRunModalOpen.value = false
    toast.value.add({
      title: 'Workflow Started',
      message: `Successfully started ${getWorkflowName(selectedWorkflow.value)}`,
      type: 'success'
    })
    await updateStatus()
  } catch (err) {
    toast.value.add({
      title: 'Execution Failed',
      message: err.message,
      type: 'error',
      duration: 8000
    })
  } finally {
    isStartingRun.value = false
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
    await updateStatus()
  } catch (err) {
     toast.value.add({
      title: 'Stop Failed',
      message: err.message,
      type: 'error'
    })
  }
}

const changeLogLevel = async (newLevel) => {
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
    const current = await fetchLogLevel()
    logLevel.value = current.level
  }
}

watch(selectedWorkflow, (path) => {
  if (path) {
    loadWorkflowDefinition(path)
  }
})


onMounted(() => {
  loadWorkflows()
  updateStatus()
  
  fetchLogLevel().then(data => {
    logLevel.value = data.level
  }).catch(err => console.error('Failed to load log level:', err))

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

.main-content {
  flex: 1;
  display: flex;
  height: calc(100vh - 64px);
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
</style>
