import { apiClient } from './apiClient'

export const registryApi = {
  /**
   * List all registered modules
   * @param {number} pageSize - Maximum number of modules to return
   * @param {string} pageToken - Page token for pagination
   * @returns {Promise<Object>} Response with modules and next_page_token
   */
  async listModules(pageSize = 50, pageToken = '') {
    const params = { page_size: pageSize }
    if (pageToken) {
      params.page_token = pageToken
    }
    const response = await apiClient.get('/v1/modules', { params })
    return response.data
  },

  /**
   * Get a module by name
   * @param {string} name - Module name
   * @param {boolean} includeDraftTags - Include draft tags or not
   * @returns {Promise<Object>} Module object
   */
  async getModule(name, includeDraftTags = false) {
    const response = await apiClient.post('/v1/modules/get', {
      name,
      includeDraftTags
    })
    return response.data
  },

  /**
   * Pull a module tag
   * @param {string} name - Module name
   * @param {string} tag - Version tag
   * @returns {Promise<Object>} Module with proto files
   */
  async pullModule(name, tag) {
    const response = await apiClient.post('/v1/modules/pull', {
      name,
      tag
    })
    return response.data
  },

  /**
   * Get module dependencies
   * @param {string} name - Module name
   * @param {string} tag - Version tag
   * @returns {Promise<Object>} Dependencies response
   */
  async getModuleDependencies(name, tag) {
    const response = await apiClient.post('/v1/modules/dependencies', {
      name,
      tag
    })
    return response.data
  }
}
