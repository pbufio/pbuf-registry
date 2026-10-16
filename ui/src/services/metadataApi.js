import { apiClient } from './apiClient'

export const metadataApi = {
  /**
   * Get metadata for a module
   * @param {string} name - Module name
   * @param {string} tag - Version tag
   * @returns {Promise<Object>} Metadata response with packages, proto files, messages, services
   */
  async getMetadata(name, tag) {
    const response = await apiClient.post('/v1/metadata', {
      name,
      tag
    })
    return response.data
  }
}
