<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import PCard from '@/components/PCard.vue'
import PInput from '@/components/PInput.vue'
import PBadge from '@/components/PBadge.vue'
import PButton from '@/components/PButton.vue'
import { registryApi } from '@/services/registryApi'

const router = useRouter()
const modules = ref([])
const loading = ref(true)
const searchQuery = ref('')
const pageToken = ref('')
const nextPageToken = ref('')

const loadModules = async () => {
  loading.value = true
  try {
    const response = await registryApi.listModules(50, pageToken.value)
    const modulesList = response.modules || []
    
    // Fetch full details for modules with empty tags
    const modulesWithDetails = await Promise.all(
      modulesList.map(async (module) => {
        // If tags and draftTags are both empty, fetch full module details
        if ((!module.tags || module.tags.length === 0) && 
            (!module.draftTags || module.draftTags.length === 0)) {
          try {
            const fullModule = await registryApi.getModule(module.name, true)
            return fullModule
          } catch (error) {
            console.error(`Failed to load details for module ${module.name}:`, error)
            return module // Return original module on error
          }
        }
        return module
      })
    )
    
    modules.value = modulesWithDetails
    nextPageToken.value = response.next_page_token || ''
  } catch (error) {
    console.error('Failed to load modules:', error)
  } finally {
    loading.value = false
  }
}

const goToModule = (moduleName) => {
  router.push(`/modules/${moduleName}`)
}

const nextPage = () => {
  if (nextPageToken.value) {
    pageToken.value = nextPageToken.value
    loadModules()
  }
}

onMounted(() => {
  loadModules()
})

// Computed property for filtered modules
const filteredModules = computed(() => {
  if (!searchQuery.value) {
    return modules.value
  }
  return modules.value.filter(m => 
    m.name.toLowerCase().includes(searchQuery.value.toLowerCase())
  )
})
</script>

<template>
  <div class="min-h-screen bg-surface-base">
    <div class="max-w-7xl mx-auto px-4 py-12">
      <div class="mb-8">
        <h1 class="text-4xl font-bold tracking-tight mb-4">Browse Modules</h1>
        <p class="text-zinc-400 text-lg">Explore all registered protobuf modules in the registry</p>
      </div>

      <!-- Search Bar -->
      <div class="mb-6">
        <PInput 
          v-model="searchQuery" 
          placeholder="Search modules..."
          type="text"
        />
      </div>

      <!-- Modules List -->
      <div v-if="loading" class="text-center py-12">
        <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-brand"></div>
        <p class="text-zinc-400 mt-4">Loading modules...</p>
      </div>

      <div v-else-if="filteredModules.length === 0" class="text-center py-12">
        <p class="text-zinc-400">No modules found{{ searchQuery ? ' matching your search' : ' in the registry' }}.</p>
      </div>

      <div v-else class="space-y-4">
        <PCard 
          v-for="module in filteredModules" 
          :key="module.id"
          hoverable
          @click="goToModule(module.name)"
        >
          <div class="flex items-start justify-between">
            <div class="flex-1">
              <h3 class="text-xl font-semibold text-zinc-50 mb-2 font-mono">{{ module.name }}</h3>
              
              <div class="flex flex-wrap gap-2 mb-3">
                <PBadge 
                  v-for="tag in module.tags" 
                  :key="tag"
                  variant="brand"
                >
                  {{ tag }}
                </PBadge>
                <PBadge 
                  v-for="draftTag in module.draftTags" 
                  :key="draftTag"
                  variant="draft"
                >
                  {{ draftTag }} (draft)
                </PBadge>
              </div>

              <div v-if="module.packages && module.packages.length > 0" class="text-sm text-zinc-400">
                <span class="font-medium">Packages:</span> {{ module.packages.join(', ') }}
              </div>
            </div>

            <div class="ml-4">
              <svg class="w-6 h-6 text-zinc-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
            </div>
          </div>
        </PCard>
      </div>

      <!-- Pagination -->
      <div v-if="nextPageToken && !loading" class="mt-8 flex justify-center">
        <PButton @click="nextPage">Load More</PButton>
      </div>
    </div>
  </div>
</template>
