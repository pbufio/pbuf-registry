import { apiClient } from './apiClient'

export const driftApi = {
  /**
   * Get drift events for a specific module, optionally filtered by tag.
   * Swagger: POST /v1/drift/modules/events
   * Body: { moduleName, tagName? }
   *
   * @param {string} moduleName - Module name (may include slashes)
   * @param {string} tagName - Optional tag name to filter by
   * @returns {Promise<Object>} Response with `events`
   */
  async getModuleDriftEvents(moduleName, tagName = '') {
    const body = {
      moduleName
    }
    if (tagName) body.tagName = tagName

    const response = await apiClient.post('/v1/drift/modules/events', body)
    return response.data
  },

  /**
   * Get dependency drift status for a specific module, optionally filtered by tag.
   * Swagger: POST /v1/drift/modules/dependencies/status
   * Body: { moduleName, tagName? }
   *
   * @param {string} moduleName - Module name (may include slashes)
   * @param {string} tagName - Optional tag name to evaluate
   * @returns {Promise<Object>} Response with `statuses`
   */
  async getModuleDependencyDriftStatus(moduleName, tagName = '') {
    const body = {
      moduleName
    }
    if (tagName) body.tagName = tagName

    const response = await apiClient.post('/v1/drift/modules/dependencies/status', body)
    return response.data
  }
}
