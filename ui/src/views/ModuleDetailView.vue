<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PCard from '@/components/PCard.vue'
import PBadge from '@/components/PBadge.vue'
import PButton from '@/components/PButton.vue'
import PCodeBlock from '@/components/PCodeBlock.vue'
import { registryApi } from '@/services/registryApi'
import { driftApi } from '@/services/driftApi'

const route = useRoute()
const router = useRouter()
const module = ref(null)
const loading = ref(true)
const error = ref(null)
const tagDriftSummaryByTag = ref({})
const tagDriftError = ref(null)

const moduleName = computed(() => {
  const name = route.params.name
  return Array.isArray(name) ? name.join('/') : name
})

const loadTagDriftSummary = async (tags = []) => {
  tagDriftSummaryByTag.value = {}
  tagDriftError.value = null
  if (!tags.length) return

  const results = await Promise.allSettled(
    tags.map(async (tag) => {
      const response = await driftApi.getModuleDependencyDriftStatus(moduleName.value, tag)
      const statuses = response?.statuses || []
      let hasAlerts = false
      let hasRecommendations = false

      for (const status of statuses) {
        const severity = status?.severity
        const recommendation = status?.recommendation

        if (
          severity === 'DRIFT_SEVERITY_CRITICAL'
          || severity === 'DRIFT_SEVERITY_WARNING'
          || recommendation === 'DEPENDENCY_DRIFT_RECOMMENDATION_ALERT_REVIEW'
        ) {
          hasAlerts = true
        }

        if (
          recommendation === 'DEPENDENCY_DRIFT_RECOMMENDATION_ALERT_REVIEW'
          || recommendation === 'DEPENDENCY_DRIFT_RECOMMENDATION_SUGGEST_UPDATE'
        ) {
          hasRecommendations = true
        }
      }

      return { hasAlerts, hasRecommendations, unknown: false }
    })
  )

  const summary = {}
  let hasFailures = false
  results.forEach((result, index) => {
    const tag = tags[index]
    if (result.status === 'fulfilled') {
      summary[tag] = result.value
      return
    }

    hasFailures = true
    summary[tag] = { hasAlerts: false, hasRecommendations: false, unknown: true }
  })

  if (hasFailures) {
    tagDriftError.value = 'Some dependency drift summaries are unavailable.'
  }

  tagDriftSummaryByTag.value = summary
}

const getTagDriftSummary = (tag) => {
  return tagDriftSummaryByTag.value[tag] || {
    hasAlerts: false,
    hasRecommendations: false,
    unknown: true
  }
}

const tagAlertsBadgeVariant = (tag) => {
  return shouldShowTagAlertsBadge(tag) ? 'critical' : 'default'
}

const tagRecommendationsBadgeVariant = (tag) => {
  return shouldShowTagNewBadge(tag) ? 'info' : 'default'
}

const shouldShowTagAlertsBadge = (tag) => {
  const summary = getTagDriftSummary(tag)
  return !summary.unknown && summary.hasAlerts
}

const shouldShowTagNewBadge = (tag) => {
  const summary = getTagDriftSummary(tag)
  return !summary.unknown && !summary.hasAlerts && summary.hasRecommendations
}

const tagAlertsBadgeLabel = (tag) => {
  return shouldShowTagAlertsBadge(tag) ? 'ALERT' : ''
}

const tagRecommendationsBadgeLabel = (tag) => {
  return shouldShowTagNewBadge(tag) ? 'NEW' : ''
}

const loadModule = async () => {
  loading.value = true
  error.value = null
  try {
    const response = await registryApi.getModule(moduleName.value, true)
    module.value = response
    await loadTagDriftSummary(response.tags || [])
  } catch (err) {
    console.error('Failed to load module:', err)
    error.value = err.response?.data?.message || err.message || 'Failed to load module'
  } finally {
    loading.value = false
  }
}

const viewVersion = (tag) => {
  router.push(`/modules/${moduleName.value}/tags/${tag}`)
}

const vendorConfig = (tag) => {
  return `# Add to your pbuf.yaml:
modules:
  - name: ${moduleName.value}
    tag: ${tag}
    out: third_party

# Then run:
# pbuf-cli vendor`
}

onMounted(() => {
  loadModule()
})
</script>

<template>
  <div class="min-h-screen bg-surface-base">
    <div class="max-w-7xl mx-auto px-4 py-12">
      <!-- Loading State -->
      <div v-if="loading" class="text-center py-12">
        <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-brand"></div>
        <p class="text-zinc-400 mt-4">Loading module...</p>
      </div>

      <!-- Module Content -->
      <div v-else-if="module">
        <!-- Header -->
        <div class="mb-8">
          <PButton variant="secondary" @click="$router.push('/modules')" class="mb-4">
            ← Back
          </PButton>
          <h1 class="text-4xl font-bold tracking-tight mb-2 font-mono">{{ module.name }}</h1>
          <p class="text-zinc-400">Module ID: <span class="font-mono text-sm">{{ module.id }}</span></p>
        </div>

        <!-- Quick Install -->
        <div class="mb-8">
          <h2 class="text-2xl font-semibold mb-4">Vendor This Module</h2>
          <PCodeBlock :code="vendorConfig(module.tags[0] || 'v1.0.0')" />
          <p class="text-zinc-400 text-sm mt-2">
            Learn more: 
            <a href="https://github.com/pbufio/pbuf-cli" target="_blank" rel="noopener noreferrer" class="text-brand hover:underline">
              pbuf-cli Documentation
            </a>
          </p>
        </div>

        <!-- Tags Section -->
        <div class="mb-8">
          <h2 class="text-2xl font-semibold mb-4">Available Tags</h2>
          <PCard v-if="module.tags && module.tags.length > 0">
            <p v-if="tagDriftError" class="text-xs text-amber-300 mb-3">
              {{ tagDriftError }}
            </p>
            <div class="space-y-4">
              <div 
                v-for="tag in module.tags" 
                :key="tag"
                class="flex items-center justify-between p-4 bg-surface-base rounded-lg hover:bg-zinc-900/50 transition-colors cursor-pointer"
                @click="viewVersion(tag)"
              >
                <div class="min-w-0">
                  <div class="flex items-center gap-2 flex-wrap">
                    <PBadge variant="brand">{{ tag }}</PBadge>
                    <PBadge v-if="shouldShowTagAlertsBadge(tag)" :variant="tagAlertsBadgeVariant(tag)">
                      {{ tagAlertsBadgeLabel(tag) }}
                    </PBadge>
                    <PBadge v-if="shouldShowTagNewBadge(tag)" :variant="tagRecommendationsBadgeVariant(tag)">
                      {{ tagRecommendationsBadgeLabel(tag) }}
                    </PBadge>
                    <span class="text-zinc-300 font-mono text-sm">{{ module.name }}@{{ tag }}</span>
                  </div>
                </div>
                <PButton variant="secondary" @click.stop="viewVersion(tag)">
                  View Details →
                </PButton>
              </div>
            </div>
          </PCard>
          <p v-else class="text-zinc-400">No published tags available.</p>
        </div>

        <!-- Draft Tags Section -->
        <div v-if="module.draftTags && module.draftTags.length > 0" class="mb-8">
          <h2 class="text-2xl font-semibold mb-4">Draft Tags</h2>
          <PCard>
            <div class="space-y-4">
              <div 
                v-for="tag in module.draftTags" 
                :key="tag"
                class="flex items-center justify-between p-4 bg-surface-base rounded-lg"
              >
                <div class="flex items-center gap-3">
                  <PBadge variant="draft">{{ tag }}</PBadge>
                  <span class="text-zinc-400 font-mono text-sm">{{ module.name }}@{{ tag }} (draft)</span>
                </div>
              </div>
            </div>
          </PCard>
        </div>

        <!-- Packages Section -->
        <div v-if="module.packages && module.packages.length > 0" class="mb-8">
          <h2 class="text-2xl font-semibold mb-4">Packages</h2>
          <PCard>
            <div class="flex flex-wrap gap-2">
              <PBadge v-for="pkg in module.packages" :key="pkg">
                {{ pkg }}
              </PBadge>
            </div>
          </PCard>
        </div>

        <!-- Usage Examples -->
        <div class="mb-8">
          <h2 class="text-2xl font-semibold mb-4">Usage Examples</h2>
          <div class="space-y-4">
            <div>
              <h3 class="text-lg font-medium mb-2 text-zinc-200">Vendor a specific version</h3>
              <PCodeBlock :code="vendorConfig(module.tags[0] || 'v1.0.0')" />
            </div>
            <div>
              <h3 class="text-lg font-medium mb-2 text-zinc-200">Get module information</h3>
              <PCodeBlock :code="`pbuf-cli modules get ${module.name}`" />
            </div>
            <div>
              <h3 class="text-lg font-medium mb-2 text-zinc-200">Advanced: Vendor with code generation path</h3>
              <PCodeBlock :code="`# Add to your pbuf.yaml:
modules:
  - name: ${module.name}
    tag: ${module.tags[0] || 'v1.0.0'}
    out: third_party
    gen_out: gen  # patches go_package option

# Then run:
# pbuf-cli vendor`" />
            </div>
          </div>
        </div>
      </div>

      <!-- Error State -->
      <div v-else-if="error" class="text-center py-12">
        <div class="mb-4">
          <svg class="w-16 h-16 text-red-500 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="text-zinc-300 font-semibold mb-2">Failed to Load Module</p>
          <p class="text-zinc-400">{{ error }}</p>
        </div>
        <div class="flex gap-4 justify-center">
          <PButton variant="primary" @click="loadModule">
            Retry
          </PButton>
          <PButton variant="secondary" @click="$router.push('/modules')">
            Go Back
          </PButton>
        </div>
      </div>

      <!-- Not Found State -->
      <div v-else class="text-center py-12">
        <p class="text-zinc-400">Module not found.</p>
        <PButton variant="secondary" @click="$router.push('/modules')" class="mt-4">
          Go Back
        </PButton>
      </div>
    </div>
  </div>
</template>
