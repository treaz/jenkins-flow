<template>
  <div class="workflow-view" v-if="workflow">
    <div class="workflow-header">
      <div class="workflow-info">
        <h2 class="workflow-name">{{ workflow.name }}</h2>
        <div class="workflow-meta">
          <StatusBadge :status="workflow.status || 'pending'" />
          <span v-if="workflow.startedAt" class="started-time">
            Started {{ formatTime(workflow.startedAt) }}
          </span>
          <span v-if="totalDuration" class="total-duration">
            {{ totalDuration }}
          </span>
        </div>
      </div>
      
      <div class="workflow-controls">
        <button
          v-if="!isRunning"
          class="btn btn-primary"
          :disabled="hasError || isStartingRun"
          @click="emitRun"
        >
          {{ isStartingRun ? 'Starting...' : 'Run Workflow' }}
        </button>
        <button
          v-else
          class="btn btn-danger"
          @click="$emit('stop')"
        >
           Stop
        </button>
      </div>
    </div>
    
    <div v-if="workflow.error" class="workflow-error">
      {{ workflow.error }}
    </div>

    <div v-if="!isRunning && hasInputs" class="workflow-inputs">
      <div v-for="(_, key) in localInputs" :key="key" class="input-group">
        <label :for="`input-${key}`">{{ key }}</label>
        <input :id="`input-${key}`" v-model="localInputs[key]" type="text" class="input-field" />
      </div>
    </div>

    <div class="workflow-items">
      <div v-for="(item, index) in workflow.items" :key="index" class="workflow-item">
        <div class="item-connector" v-if="index > 0">
          <div class="connector-line"></div>
        </div>
        
        <StepCard
          v-if="item.isParallel"
          :name="item.parallel?.name || `Parallel Group ${index + 1}`"
          :status="item.parallel?.status || 'pending'"
          :is-parallel="true"
          :steps="item.parallel?.steps"
          :show-toggle="!isRunning"
          :disabled-sub-steps="getDisabledForItem(index)"
          @toggle-sub-step="(stepIndex) => toggleStep(index, stepIndex)"
        />
        <PRWaitCard
          v-else-if="item.isPRWait"
          :name="item.prWait?.name || 'Wait for Pull Request'"
          :owner="item.prWait?.owner"
          :repo="item.prWait?.repo"
          :head-branch="item.prWait?.headBranch"
          :pr-number="item.prWait?.prNumber"
          :wait-for="item.prWait?.waitFor"
          :status="item.prWait?.status || 'pending'"
          :html-url="item.prWait?.htmlUrl"
          :pr-title="item.prWait?.title"
          :error="item.prWait?.error"
          :started-at="item.prWait?.startedAt"
          :ended-at="item.prWait?.endedAt"
          :show-toggle="!isRunning"
          :enabled="!isDisabled(index, 0)"
          @toggle="toggleStep(index, 0)"
        />
        <StepCard
          v-else
          :name="item.step?.name || 'Unknown'"
          :instance="item.step?.instance"
          :job="item.step?.job"
          :status="item.step?.status || 'pending'"
          :build-url="item.step?.buildUrl"
          :error="item.step?.error"
          :started-at="item.step?.startedAt"
          :ended-at="item.step?.endedAt"
          :show-toggle="!isRunning"
          :enabled="!isDisabled(index, 0)"
          @toggle="toggleStep(index, 0)"
        />
      </div>
    </div>
  </div>
  
  <div class="empty-state" v-else>
    <div class="empty-icon">⚙️</div>
    <h3>No Workflow Running</h3>
    <p>Select a workflow and click Run to start execution.</p>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import StepCard from './StepCard.vue'
import StatusBadge from './StatusBadge.vue'
import PRWaitCard from './PRWaitCard.vue'

const props = defineProps({
  workflow: Object,
  isRunning: Boolean,
  isStartingRun: Boolean
})

const emit = defineEmits(['run', 'stop'])

// Set of disabled step keys like "itemIndex:stepIndex"
const disabledSteps = ref(new Set())
const localInputs = ref({})

// Reset state when a different workflow is selected
watch(() => props.workflow?.name, () => {
  disabledSteps.value = new Set()
  localInputs.value = { ...(props.workflow?.inputs || {}) }
}, { immediate: true })

const isDisabled = (itemIndex, stepIndex) => {
  return disabledSteps.value.has(`${itemIndex}:${stepIndex}`)
}

const toggleStep = (itemIndex, stepIndex) => {
  const key = `${itemIndex}:${stepIndex}`
  const next = new Set(disabledSteps.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  disabledSteps.value = next
}

const getDisabledForItem = (itemIndex) => {
  const result = new Set()
  for (const key of disabledSteps.value) {
    const [iStr, sStr] = key.split(':')
    if (parseInt(iStr) === itemIndex) {
      result.add(parseInt(sStr))
    }
  }
  return result
}

const emitRun = () => {
  const disabledList = []
  for (const key of disabledSteps.value) {
    const [iStr, sStr] = key.split(':')
    disabledList.push({ itemIndex: parseInt(iStr), stepIndex: parseInt(sStr) })
  }
  emit('run', { disabledSteps: disabledList, inputs: { ...localInputs.value } })
}

const formatTime = (isoString) => {
  if (!isoString) return ''
  const date = new Date(isoString)
  return date.toLocaleTimeString()
}

const hasError = computed(() => !!props.workflow?.error)

const hasInputs = computed(() => Object.keys(localInputs.value).length > 0)

const totalDuration = computed(() => {
  if (!props.workflow?.startedAt) return null

  const start = new Date(props.workflow.startedAt)
  const end = props.workflow.endedAt ? new Date(props.workflow.endedAt) : new Date()
  const diff = Math.floor((end - start) / 1000)

  if (diff < 60) return `${diff}s`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ${diff % 60}s`
  return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`
})
</script>

<style scoped>
.workflow-view {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 24px;
}

.workflow-header {
  margin-bottom: 24px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.workflow-name {
  font-size: 20px;
  font-weight: 700;
  margin-bottom: 8px;
}

.workflow-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 13px;
  color: var(--text-secondary);
}

.workflow-error {
  margin-bottom: 24px;
  padding: 16px;
  background: var(--status-failed-bg);
  border-radius: var(--radius-md);
  color: var(--status-failed);
  font-family: monospace;
  font-size: 13px;
}

.workflow-inputs {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 24px;
  padding: 16px;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 180px;
  flex: 1;
}

.input-group label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.input-field {
  padding: 7px 10px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  background: var(--bg-secondary);
  color: var(--text-primary);
  font-size: 13px;
}

.input-field:focus {
  outline: none;
  border-color: var(--accent);
}

.workflow-items {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.workflow-item {
  position: relative;
}

.item-connector {
  display: flex;
  justify-content: center;
  padding: 8px 0;
}

.connector-line {
  width: 2px;
  height: 24px;
  background: var(--border-color);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 24px;
  text-align: center;
  background: var(--bg-secondary);
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-lg);
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-state h3 {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text-primary);
}

.empty-state p {
  color: var(--text-secondary);
  font-size: 14px;
}

.btn {
  padding: 8px 16px;
  border-radius: var(--radius-md);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.2s;
}

.btn-primary {
  background: var(--accent);
  color: white;
}

.btn-primary:hover {
  background: var(--accent-hover);
}

.btn-danger {
  background: #ef4444;
  color: white;
}

.btn-danger:hover {
  opacity: 0.9;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary:disabled {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}
</style>
