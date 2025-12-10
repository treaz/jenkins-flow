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
 * Triggers a workflow run.
 * @param {string} workflowPath - Path to the workflow file
 * @returns {Promise<{status: string}>}
 */
export async function runWorkflow(workflowPath) {
    const res = await fetch(`${API_BASE}/api/run`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workflow: workflowPath })
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to start workflow');
    }
    return res.json();
}
