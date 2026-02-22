const API_BASE = '';

/**
 * Fetches the list of available workflows.
 * @returns {Promise<Array<{name: string, path: string}>>}
 */
export async function fetchWorkflows() {
    const res = await fetch(`${API_BASE}/api/workflows`);
    if (!res.ok) throw new Error('Failed to fetch workflows');
    return res.json();
}

/**
 * Fetches the current execution status.
 * @returns {Promise<{running: boolean, workflow: Object|null}>}
 */
export async function fetchStatus() {
    const res = await fetch(`${API_BASE}/api/status`);
    if (!res.ok) throw new Error('Failed to fetch status');
    return res.json();
}

/**
 * Fetches the static definition of a workflow for preview rendering.
 * @param {string} workflowPath - Absolute path returned by fetchWorkflows
 * @returns {Promise<Object>}
 */
export async function fetchWorkflowDefinition(workflowPath) {
    const encoded = encodeURIComponent(workflowPath);
    const res = await fetch(`${API_BASE}/api/workflows/${encoded}/definition`);
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to fetch workflow definition');
    }
    return res.json();
}

/**
 * Triggers a workflow run.
 * @param {string} workflowPath - Path to the workflow file
 * @param {Object} options
 * @param {Object} options.inputs - Workflow input values
 * @param {Array} options.disabledSteps - Steps to skip
 * @returns {Promise<{status: string}>}
 */
export async function runWorkflow(workflowPath, { inputs = {}, disabledSteps = [] } = {}) {
    const res = await fetch(`${API_BASE}/api/run`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workflow: workflowPath, inputs, disabledSteps })
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to start workflow');
    }
    return res.json();
}

/**
 * Fetches the current log level.
 * @returns {Promise<{level: string}>}
 */
export async function fetchLogLevel() {
    const res = await fetch(`${API_BASE}/api/settings/log-level`);
    if (!res.ok) throw new Error('Failed to fetch log level');
    return res.json();
}

/**
 * Sets the log level.
 * @param {string} level - "INFO", "DEBUG", etc.
 * @returns {Promise<{level: string}>}
 */
export async function setLogLevel(level) {
    const res = await fetch(`${API_BASE}/api/settings/log-level`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level })
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to set log level');
    }
    return res.json();
}

/**
 * Stops the currently running workflow.
 * @returns {Promise<{status: string}>}
 */
export async function stopWorkflow() {
    const res = await fetch(`${API_BASE}/api/stop`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to stop workflow');
    }
    return res.json();
}
