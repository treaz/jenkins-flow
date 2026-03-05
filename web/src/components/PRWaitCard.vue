<template>
  <div class="pr-card" :class="{ 'pr-card--disabled': !enabled }">
    <div class="pr-header">
      <div class="pr-info">
        <label v-if="showToggle" class="step-toggle">
          <input type="checkbox" :checked="enabled" @change="$emit('toggle')" />
          <h3 class="pr-name">{{ name }}</h3>
        </label>
        <h3 v-else class="pr-name">{{ name }}</h3>

        <!-- Editable mode: input fields for PR properties -->
        <div v-if="editable" class="pr-inputs">
          <div class="pr-input-row">
            <div class="pr-input-group">
              <label>Owner</label>
              <input type="text" :value="localOwner" @input="updateField('owner', $event.target.value)" class="pr-input-field" />
            </div>
            <div class="pr-input-group">
              <label>Repo</label>
              <input type="text" :value="localRepo" @input="updateField('repo', $event.target.value)" class="pr-input-field" />
            </div>
          </div>
          <div class="pr-input-row">
            <div class="pr-input-group">
              <label>Head Branch</label>
              <input type="text" :value="localHeadBranch" @input="updateField('headBranch', $event.target.value)" class="pr-input-field" placeholder="branch name" />
            </div>
            <div class="pr-input-group pr-input-group--small">
              <label>PR #</label>
              <input type="number" :value="localPrNumber || ''" @input="updateField('prNumber', parseInt($event.target.value) || 0)" class="pr-input-field" placeholder="#" />
            </div>
          </div>
          <div class="pr-input-row">
            <div class="pr-input-group">
              <label>Wait For</label>
              <select :value="localWaitFor" @change="updateField('waitFor', $event.target.value)" class="pr-input-field">
                <option value="merged">Merged</option>
                <option value="closed">Closed</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Read-only mode: display metadata -->
        <div v-else class="pr-meta">
          <span class="repo" v-if="repoPath">{{ repoPath }}</span>
          <span class="identifier" v-if="identifier">
            {{ identifier }}
          </span>
          <span class="target" v-if="waitForLabel">
            Waiting for {{ waitForLabel }}
          </span>
        </div>
      </div>
      <StatusBadge :status="status" />
    </div>

    <div v-if="htmlUrl" class="pr-link">
      <a :href="htmlUrl" target="_blank" rel="noopener">
        {{ linkLabel }} →
      </a>
    </div>

    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <div v-if="duration" class="duration">
      {{ duration }}
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  name: { type: String, required: true },
  owner: { type: String, default: '' },
  repo: { type: String, default: '' },
  headBranch: { type: String, default: '' },
  prNumber: { type: Number, default: 0 },
  waitFor: { type: String, default: '' },
  status: { type: String, required: true },
  htmlUrl: { type: String, default: '' },
  prTitle: { type: String, default: '' },
  error: { type: String, default: '' },
  startedAt: String,
  endedAt: String,
  showToggle: { type: Boolean, default: false },
  enabled: { type: Boolean, default: true },
  editable: { type: Boolean, default: false },
  itemIndex: { type: Number, default: -1 }
})

const emit = defineEmits(['toggle', 'update:prWait'])

// Local editable state
const localOwner = ref(props.owner)
const localRepo = ref(props.repo)
const localHeadBranch = ref(props.headBranch)
const localPrNumber = ref(props.prNumber)
const localWaitFor = ref(props.waitFor)

// Reset local state when props change (e.g. workflow selection changes)
watch(() => [props.owner, props.repo, props.headBranch, props.prNumber, props.waitFor], () => {
  localOwner.value = props.owner
  localRepo.value = props.repo
  localHeadBranch.value = props.headBranch
  localPrNumber.value = props.prNumber
  localWaitFor.value = props.waitFor
})

const updateField = (field, value) => {
  switch (field) {
    case 'owner': localOwner.value = value; break
    case 'repo': localRepo.value = value; break
    case 'headBranch': localHeadBranch.value = value; break
    case 'prNumber': localPrNumber.value = value; break
    case 'waitFor': localWaitFor.value = value; break
  }
  emit('update:prWait', {
    itemIndex: props.itemIndex,
    owner: localOwner.value,
    repo: localRepo.value,
    headBranch: localHeadBranch.value,
    prNumber: localPrNumber.value,
    waitFor: localWaitFor.value
  })
}

const repoPath = computed(() => {
  if (!props.owner && !props.repo) return ''
  if (!props.owner) return props.repo
  if (!props.repo) return props.owner
  return `${props.owner}/${props.repo}`
})

const identifier = computed(() => {
  if (props.prNumber > 0) return `PR #${props.prNumber}`
  if (props.headBranch) return `Branch ${props.headBranch}`
  return ''
})

const waitForLabel = computed(() => {
  if (!props.waitFor) return ''
  return props.waitFor.charAt(0).toUpperCase() + props.waitFor.slice(1)
})

const linkLabel = computed(() => {
  if (props.prTitle) return props.prTitle
  if (props.prNumber > 0) return `Open PR #${props.prNumber}`
  return 'Open pull request'
})

const duration = computed(() => {
  if (!props.startedAt) return null

  const start = new Date(props.startedAt)
  const end = props.endedAt ? new Date(props.endedAt) : new Date()
  const diff = Math.floor((end - start) / 1000)

  if (diff < 60) return `${diff}s`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ${diff % 60}s`
  return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`
})
</script>

<style scoped>
.pr-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 16px;
}

.pr-card--disabled {
  opacity: 0.5;
}

.step-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  margin-bottom: 4px;
}

.step-toggle input[type="checkbox"] {
  width: 15px;
  height: 15px;
  cursor: pointer;
  flex-shrink: 0;
}

.step-toggle .pr-name {
  margin-bottom: 0;
}

.pr-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.pr-info {
  flex: 1;
  min-width: 0;
}

.pr-name {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--text-primary);
}

.pr-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 13px;
  color: var(--text-secondary);
}

.pr-meta .repo::before {
  content: '\2387 ';
  opacity: 0.6;
}

.pr-meta .identifier::before {
  content: '# ';
  opacity: 0.6;
}

.pr-meta .target::before {
  content: '\23F1 ';
  opacity: 0.6;
}

.pr-inputs {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pr-input-row {
  display: flex;
  gap: 12px;
}

.pr-input-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 0;
}

.pr-input-group--small {
  flex: 0 0 80px;
}

.pr-input-group label {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.pr-input-field {
  padding: 5px 8px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 13px;
}

.pr-input-field:focus {
  outline: none;
  border-color: var(--accent);
}

select.pr-input-field {
  cursor: pointer;
}

.pr-link {
  margin-top: 12px;
}

.pr-link a {
  color: var(--accent);
  text-decoration: none;
  font-size: 13px;
  font-weight: 500;
}

.pr-link a:hover {
  text-decoration: underline;
}

.error-message {
  margin-top: 12px;
  padding: 10px 12px;
  background: var(--status-failed-bg);
  border-radius: var(--radius-sm);
  color: var(--status-failed);
  font-size: 13px;
  font-family: monospace;
}

.duration {
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-muted);
}
</style>
