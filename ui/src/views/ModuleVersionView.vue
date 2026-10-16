<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import PCard from '@/components/PCard.vue'
import PBadge from '@/components/PBadge.vue'
import PButton from '@/components/PButton.vue'
import PCodeBlock from '@/components/PCodeBlock.vue'
import { registryApi } from '@/services/registryApi'
import { metadataApi } from '@/services/metadataApi'
import { driftApi } from '@/services/driftApi'

const route = useRoute()
const protoFiles = ref([])
const dependencies = ref([])
const metadata = ref(null)
const loading = ref(true)
const loadingProtoFiles = ref(false)
const protoFilesLoaded = ref(false)
const activeTab = ref('files')
const error = ref(null)
const protoFilesError = ref(null)

const driftEvents = ref([])
const driftLoading = ref(false)
const driftLoaded = ref(false)
const driftError = ref(null)
const dependencyDriftStatuses = ref([])
const dependencyDriftError = ref(null)

const moduleName = computed(() => {
  const name = route.params.name
  return Array.isArray(name) ? name.join('/') : name
})
const tag = computed(() => route.params.tag)

watch([moduleName, tag], () => {
  // Reset state when navigating between tags without recreating the component
  protoFiles.value = []
  dependencies.value = []
  metadata.value = null
  error.value = null
  protoFilesError.value = null

  protoFilesLoaded.value = false
  driftLoaded.value = false
  driftEvents.value = []
  driftError.value = null
  dependencyDriftStatuses.value = []
  dependencyDriftError.value = null

  activeTab.value = 'files'

  loadVersionDetails()
  loadProtoFiles()
  loadDriftEvents()
})

const severityRank = {
  DRIFT_SEVERITY_UNSPECIFIED: 0,
  DRIFT_SEVERITY_INFO: 1,
  DRIFT_SEVERITY_WARNING: 2,
  DRIFT_SEVERITY_CRITICAL: 3
}

const driftSummary = computed(() => {
  const events = driftEvents.value || []
  const total = events.length
  if (total === 0) return null

  const byType = {
    added: 0,
    modified: 0,
    deleted: 0,
    other: 0
  }

  const filenames = new Set()
  let unacknowledged = 0

  for (const e of events) {
    if (e?.filename) filenames.add(e.filename)
    if (!e?.acknowledged) unacknowledged += 1

    switch (e?.eventType) {
      case 'DRIFT_EVENT_TYPE_ADDED':
        byType.added += 1
        break
      case 'DRIFT_EVENT_TYPE_MODIFIED':
        byType.modified += 1
        break
      case 'DRIFT_EVENT_TYPE_DELETED':
        byType.deleted += 1
        break
      default:
        byType.other += 1
    }
  }

  return {
    total,
    filesChanged: filenames.size,
    unacknowledged,
    byType
  }
})

const overallSeverity = computed(() => {
  let max = 'DRIFT_SEVERITY_UNSPECIFIED'
  for (const e of driftEvents.value || []) {
    const s = e?.severity || 'DRIFT_SEVERITY_UNSPECIFIED'
    if ((severityRank[s] ?? 0) > (severityRank[max] ?? 0)) {
      max = s
    }
  }
  return max
})

const overallSeverityLabel = computed(() => {
  return formatEnumLabel(overallSeverity.value, 'DRIFT_SEVERITY_')
})

const severityToVariant = (severity) => {
  switch (severity) {
    case 'DRIFT_SEVERITY_INFO':
      return 'info'
    case 'DRIFT_SEVERITY_WARNING':
      return 'warning'
    case 'DRIFT_SEVERITY_CRITICAL':
      return 'critical'
    default:
      return 'default'
  }
}

const recommendationToVariant = (recommendation) => {
  switch (recommendation) {
    case 'DEPENDENCY_DRIFT_RECOMMENDATION_ALERT_REVIEW':
      return 'warning'
    case 'DEPENDENCY_DRIFT_RECOMMENDATION_SUGGEST_UPDATE':
      return 'info'
    default:
      return 'default'
  }
}

const overallSeverityVariant = computed(() => {
  return severityToVariant(overallSeverity.value)
})

const formatEnumLabel = (value, prefix) => {
  if (!value) return 'UNSPECIFIED'
  return value.replace(prefix, '').replaceAll('_', ' ')
}

const formatEventType = (eventType) => {
  if (!eventType) return 'UNSPECIFIED'
  return eventType.replace('DRIFT_EVENT_TYPE_', '')
}

const dependencyDriftByKey = computed(() => {
  const map = new Map()
  for (const status of dependencyDriftStatuses.value || []) {
    if (!status?.dependencyName) continue
    const key = `${status.dependencyName}@${status.currentTag || ''}`
    if (!map.has(key)) map.set(key, [])
    map.get(key).push(status)
  }
  return map
})

const dependenciesWithDrift = computed(() => {
  return (dependencies.value || []).map((dep) => {
    const exactKey = `${dep.name}@${dep.tag || ''}`
    const fallbackKey = `${dep.name}@`
    const statuses = dependencyDriftByKey.value.get(exactKey)
      || dependencyDriftByKey.value.get(fallbackKey)
      || []

    let highestSeverity = 'DRIFT_SEVERITY_UNSPECIFIED'
    for (const status of statuses) {
      const severity = status?.severity || 'DRIFT_SEVERITY_UNSPECIFIED'
      if ((severityRank[severity] ?? 0) > (severityRank[highestSeverity] ?? 0)) {
        highestSeverity = severity
      }
    }

    return {
      ...dep,
      driftStatuses: statuses,
      driftHighestSeverity: highestSeverity
    }
  })
})

const dependenciesTabDriftBadge = computed(() => {
  if (dependencyDriftError.value) {
    return null
  }

  let hasAlerts = false
  let hasRecommendations = false
  for (const status of dependencyDriftStatuses.value || []) {
    const severity = status?.severity || 'DRIFT_SEVERITY_UNSPECIFIED'
    const recommendation = status?.recommendation || 'DEPENDENCY_DRIFT_RECOMMENDATION_UNSPECIFIED'

    if (
      severity === 'DRIFT_SEVERITY_WARNING'
      || severity === 'DRIFT_SEVERITY_CRITICAL'
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

  if (hasAlerts) return { label: 'ALERT', variant: 'critical' }
  if (hasRecommendations) return { label: 'NEW', variant: 'info' }
  return null
})

const formatDateTime = (value) => {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

const loadVersionDetails = async () => {
  loading.value = true
  error.value = null
  dependencyDriftStatuses.value = []
  dependencyDriftError.value = null
  try {
    // Get metadata (parsed proto structure) - Level 3
    const metadataResponse = await metadataApi.getMetadata(moduleName.value, tag.value)
    metadata.value = metadataResponse

    // Get dependencies
    const depsResponse = await registryApi.getModuleDependencies(moduleName.value, tag.value)
    dependencies.value = depsResponse.dependencies || []

    try {
      const driftStatusResponse = await driftApi.getModuleDependencyDriftStatus(moduleName.value, tag.value)
      dependencyDriftStatuses.value = driftStatusResponse.statuses || []
    } catch (err) {
      console.error('Failed to load dependency drift status:', err)
      dependencyDriftStatuses.value = []
      dependencyDriftError.value = err.response?.data?.message || err.message || 'Failed to load dependency drift status'
    }
  } catch (err) {
    console.error('Failed to load version details:', err)
    error.value = err.response?.data?.message || err.message || 'Failed to load version details'
  } finally {
    loading.value = false
  }
}

const loadProtoFiles = async () => {
  if (protoFilesLoaded.value) return
  
  loadingProtoFiles.value = true
  protoFilesError.value = null
  try {
    // Pull module to get proto files - only when needed
    const pullResponse = await registryApi.pullModule(moduleName.value, tag.value)
    protoFiles.value = pullResponse.protofiles || []
    protoFilesLoaded.value = true
  } catch (err) {
    console.error('Failed to load proto files:', err)
    protoFilesError.value = err.response?.data?.message || err.message || 'Failed to load proto files'
  } finally {
    loadingProtoFiles.value = false
  }
}

const loadDriftEvents = async () => {
  if (driftLoaded.value) return

  driftLoading.value = true
  driftError.value = null
  try {
    const response = await driftApi.getModuleDriftEvents(moduleName.value, tag.value)
    driftEvents.value = response.events || []
    driftLoaded.value = true
  } catch (err) {
    console.error('Failed to load drift events:', err)
    driftError.value = err.response?.data?.message || err.message || 'Failed to load drift events'
  } finally {
    driftLoading.value = false
  }
}

const retryLoadDriftEvents = async () => {
  driftLoaded.value = false
  await loadDriftEvents()
}

// Watch activeTab and load proto files when Files tab is clicked
watch(activeTab, (newTab) => {
  if (newTab === 'files' && !protoFilesLoaded.value) {
    loadProtoFiles()
  }
  if (newTab === 'drift' && !driftLoaded.value) {
    loadDriftEvents()
  }
})

onMounted(() => {
  loadVersionDetails()
  loadProtoFiles()
  loadDriftEvents()
})
</script>

<template>
  <div class="min-h-screen bg-surface-base">
    <div class="max-w-7xl mx-auto px-4 py-12">
      <!-- Loading State -->
      <div v-if="loading" class="text-center py-12">
        <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-brand"></div>
        <p class="text-zinc-400 mt-4">Loading version details...</p>
      </div>

      <!-- Error State -->
      <div v-else-if="error" class="text-center py-12">
        <div class="mb-4">
          <svg class="w-16 h-16 text-red-500 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="text-zinc-300 font-semibold mb-2">Failed to Load Version Details</p>
          <p class="text-zinc-400">{{ error }}</p>
        </div>
        <div class="flex gap-4 justify-center">
          <PButton variant="primary" @click="loadVersionDetails">
            Retry
          </PButton>
          <PButton variant="secondary" @click="$router.push(`/modules/${moduleName}`)">
            Back to Module
          </PButton>
        </div>
      </div>

      <!-- Content -->
      <div v-else>
        <!-- Header -->
        <div class="mb-8">
          <PButton variant="secondary" @click="$router.push(`/modules/${moduleName}`)" class="mb-4">
            ← Back to Module
          </PButton>
          <div class="flex items-center gap-3 mb-2">
            <h1 class="text-4xl font-bold tracking-tight font-mono">{{ moduleName }}</h1>
            <PBadge variant="brand">{{ tag }}</PBadge>
          </div>
        </div>

        <!-- Quick Vendor Command -->
        <div class="mb-8">
          <h2 class="text-2xl font-semibold mb-4">Vendor This Version</h2>
          <PCodeBlock :code="`# Add to your pbuf.yaml:
modules:
  - name: ${moduleName}
    tag: ${tag}
    out: third_party

# Then run:
# pbuf-cli vendor`" />
          <p class="text-zinc-400 text-sm mt-2">
            Learn more: 
            <a href="https://github.com/pbufio/pbuf-cli" target="_blank" rel="noopener noreferrer" class="text-brand hover:underline">
              pbuf-cli Documentation
            </a>
          </p>
        </div>

        <!-- Tabs -->
        <div class="mb-6 border-b border-surface-border">
          <div class="flex gap-6">
            <button
              @click="activeTab = 'files'"
              :class="[
                'pb-3 px-1 font-medium transition-colors cursor-pointer',
                activeTab === 'files' 
                  ? 'text-brand border-b-2 border-brand' 
                  : 'text-zinc-400 hover:text-zinc-200'
              ]"
            >
              Proto Files ({{ protoFiles.length }})
            </button>
            <button
              @click="activeTab = 'dependencies'"
              :class="[
                'pb-3 px-1 font-medium transition-colors cursor-pointer',
                activeTab === 'dependencies' 
                  ? 'text-brand border-b-2 border-brand' 
                  : 'text-zinc-400 hover:text-zinc-200'
              ]"
            >
              <span class="inline-flex items-center gap-2">
                <span>Dependencies ({{ dependencies.length }})</span>
                <PBadge v-if="dependenciesTabDriftBadge" :variant="dependenciesTabDriftBadge.variant">{{ dependenciesTabDriftBadge.label }}</PBadge>
              </span>
            </button>
            <button
              @click="activeTab = 'metadata'"
              :class="[
                'pb-3 px-1 font-medium transition-colors cursor-pointer',
                activeTab === 'metadata' 
                  ? 'text-brand border-b-2 border-brand' 
                  : 'text-zinc-400 hover:text-zinc-200'
              ]"
            >
              Metadata
            </button>
            <button
              @click="activeTab = 'drift'"
              :class="[
                'pb-3 px-1 font-medium transition-colors cursor-pointer',
                activeTab === 'drift'
                  ? 'text-brand border-b-2 border-brand'
                  : 'text-zinc-400 hover:text-zinc-200'
              ]"
            >
              Drift ({{ driftEvents.length }})
            </button>
          </div>
        </div>

        <!-- Proto Files Tab -->
        <div v-if="activeTab === 'files'" class="space-y-4">
          <div v-if="loadingProtoFiles" class="text-center py-12">
            <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-brand"></div>
            <p class="text-zinc-400 mt-4">Loading proto files...</p>
          </div>
          <div v-else-if="protoFilesError" class="text-center py-12">
            <PCard>
              <div class="mb-4">
                <svg class="w-12 h-12 text-red-500 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <p class="text-zinc-300 font-semibold mb-2">Failed to Load Proto Files</p>
                <p class="text-zinc-400 text-sm">{{ protoFilesError }}</p>
              </div>
              <PButton variant="primary" @click="loadProtoFiles">
                Retry
              </PButton>
            </PCard>
          </div>
          <template v-else>
            <PCard v-for="file in protoFiles" :key="file.filename">
              <h3 class="text-lg font-semibold font-mono mb-3 text-zinc-100">{{ file.filename }}</h3>
              <div class="bg-surface-base rounded-lg p-4 overflow-x-auto">
                <pre class="font-mono text-sm text-zinc-300 whitespace-pre">{{ file.content }}</pre>
              </div>
            </PCard>
            <p v-if="protoFiles.length === 0 && !protoFilesError" class="text-zinc-400 text-center py-8">No proto files available.</p>
          </template>
        </div>

        <!-- Dependencies Tab -->
        <div v-if="activeTab === 'dependencies'" class="space-y-4">
          <PCard v-if="dependencies.length > 0">
            <p v-if="dependencyDriftError" class="text-sm text-amber-300 mb-4">
              Drift status unavailable: {{ dependencyDriftError }}
            </p>
            <div class="space-y-3">
              <div
                v-for="dep in dependenciesWithDrift"
                :key="`${dep.name}@${dep.tag}`"
                class="flex items-start justify-between gap-4 p-4 bg-surface-base rounded-lg"
              >
                <div class="min-w-0">
                  <div class="flex items-center gap-3 flex-wrap">
                    <span class="font-mono text-zinc-200">{{ dep.name }}</span>
                    <PBadge variant="brand">{{ dep.tag }}</PBadge>
                    <PBadge :variant="severityToVariant(dep.driftHighestSeverity)">
                      {{ formatEnumLabel(dep.driftHighestSeverity, 'DRIFT_SEVERITY_') }}
                    </PBadge>
                  </div>
                  <div v-if="dep.driftStatuses.length > 0" class="mt-2 space-y-1">
                    <p class="text-xs text-zinc-400">
                      {{ dep.driftStatuses.length }} drift event candidate(s) on newer dependency tags
                    </p>
                    <div
                      v-for="status in dep.driftStatuses"
                      :key="`${status.dependencyName}:${status.currentTag}:${status.targetTag}`"
                      class="flex items-center gap-2 flex-wrap text-xs"
                    >
                      <span class="text-zinc-500">→ {{ status.targetTag }}</span>
                      <PBadge :variant="severityToVariant(status.severity)">
                        {{ formatEnumLabel(status.severity, 'DRIFT_SEVERITY_') }}
                      </PBadge>
                      <PBadge :variant="recommendationToVariant(status.recommendation)">
                        {{ formatEnumLabel(status.recommendation, 'DEPENDENCY_DRIFT_RECOMMENDATION_') }}
                      </PBadge>
                    </div>
                  </div>
                  <p v-else class="text-xs text-zinc-500 mt-2">
                    No dependency drift events on newer dependency tags.
                  </p>
                </div>
                <PButton variant="secondary" @click="$router.push(`/modules/${dep.name}`)">
                  View Module →
                </PButton>
              </div>
            </div>
          </PCard>
          <p v-else class="text-zinc-400 text-center py-8">No dependencies for this version.</p>
        </div>

        <!-- Metadata Tab -->
        <div v-if="activeTab === 'metadata'" class="space-y-4">
          <div v-if="metadata && metadata.packages">
            <PCard v-for="pkg in metadata.packages" :key="pkg.name" class="mb-4">
              <h3 class="text-xl font-bold mb-4 text-brand">Package: {{ pkg.name }}</h3>

              <!-- Proto Files in Package -->
              <div v-for="protoFile in pkg.protoFiles" :key="protoFile.filename" class="mb-6">
                <h4 class="text-lg font-semibold mb-3 font-mono text-zinc-200">{{ protoFile.filename }}</h4>

                <!-- Messages -->
                <div v-if="protoFile.messages && protoFile.messages.length > 0" class="mb-4">
                  <h5 class="text-md font-medium mb-2 text-zinc-300">Messages</h5>
                  <div class="space-y-3">
                    <div v-for="message in protoFile.messages" :key="message.name" class="bg-surface-base rounded-lg p-4">
                      <p class="font-mono text-brand mb-3">{{ message.name }}</p>
                      <div v-if="message.fields && message.fields.length > 0" class="pl-6 space-y-1.5">
                        <div v-for="field in message.fields" :key="field.name" class="text-sm font-mono">
                          <span class="text-zinc-500">{{ field.tag }}:</span>
                          <span class="text-zinc-300 ml-1">{{ field.messageType }}</span>
                          <span class="text-brand ml-1">{{ field.name }}</span>
                          <span v-if="field.repeated" class="text-zinc-500 ml-1">(repeated)</span>
                          <span v-if="field.optional" class="text-zinc-500 ml-1">(optional)</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Services -->
                <div v-if="protoFile.services && protoFile.services.length > 0" class="mb-4">
                  <h5 class="text-md font-medium mb-2 text-zinc-300">Services</h5>
                  <div class="space-y-3">
                    <div v-for="service in protoFile.services" :key="service.name" class="bg-surface-base rounded-lg p-4">
                      <p class="font-mono text-brand mb-2">{{ service.name }}</p>
                      <div v-if="service.methods && service.methods.length > 0" class="ml-4 space-y-1">
                        <div v-for="method in service.methods" :key="method.name" class="text-sm font-mono text-zinc-400">
                          <span class="text-zinc-300">{{ method.name }}</span>
                          <span class="text-zinc-500">({{ method.inputType }}) → {{ method.outputType }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </PCard>
          </div>
          <p v-else class="text-zinc-400 text-center py-8">No metadata available.</p>
        </div>

        <!-- Drift Tab -->
        <div v-if="activeTab === 'drift'" class="space-y-4">
          <div v-if="driftLoading" class="text-center py-12">
            <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-brand"></div>
            <p class="text-zinc-400 mt-4">Loading drift events...</p>
          </div>
          <div v-else-if="driftError" class="text-center py-12">
            <PCard>
              <div class="mb-4">
                <svg class="w-12 h-12 text-red-500 mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <p class="text-zinc-300 font-semibold mb-2">Failed to Load Drift Events</p>
                <p class="text-zinc-400 text-sm">{{ driftError }}</p>
              </div>
              <PButton variant="primary" @click="retryLoadDriftEvents">
                Retry
              </PButton>
            </PCard>
          </div>
          <template v-else>
            <PCard>
              <div class="flex items-start justify-between gap-4">
                <div>
                  <h3 class="text-lg font-semibold mb-1">What changed in {{ tag }}</h3>
                  <p v-if="driftSummary" class="text-zinc-400 text-sm">
                    {{ driftSummary.filesChanged }} file(s) affected •
                    {{ driftSummary.byType.added }} added, {{ driftSummary.byType.modified }} modified, {{ driftSummary.byType.deleted }} deleted
                    <span v-if="driftSummary.unacknowledged"> • {{ driftSummary.unacknowledged }} unacknowledged</span>
                  </p>
                  <p v-else class="text-zinc-400 text-sm">No drift events detected for this tag.</p>
                </div>
                <div class="text-right">
                  <p class="text-xs text-zinc-500 mb-1">Severity</p>
                  <PBadge :variant="overallSeverityVariant">{{ overallSeverityLabel }}</PBadge>
                </div>
              </div>
            </PCard>

            <div v-if="driftEvents.length > 0" class="space-y-3">
              <PCard v-for="event in driftEvents" :key="event.id">
                <div class="flex items-start justify-between gap-4">
                  <div class="min-w-0">
                    <p class="font-mono text-zinc-100 truncate">{{ event.filename }}</p>
                    <p class="text-sm text-zinc-400 mt-1">
                      {{ formatEventType(event.eventType) }} • Detected {{ formatDateTime(event.detectedAt) }}
                    </p>
                    <p class="text-xs text-zinc-500 mt-1" v-if="event.previousHash || event.currentHash">
                      <span v-if="event.previousHash">prev: {{ event.previousHash }}</span>
                      <span v-if="event.previousHash && event.currentHash"> • </span>
                      <span v-if="event.currentHash">curr: {{ event.currentHash }}</span>
                    </p>
                  </div>
                  <div class="flex flex-col items-end gap-2">
                    <PBadge :variant="severityToVariant(event.severity)">
                      {{ formatEnumLabel(event.severity, 'DRIFT_SEVERITY_') }}
                    </PBadge>
                    <PBadge :variant="event.acknowledged ? 'default' : 'brand'">
                      {{ event.acknowledged ? 'ACKNOWLEDGED' : 'UNACKNOWLEDGED' }}
                    </PBadge>
                  </div>
                </div>
                <div v-if="event.acknowledged" class="mt-3 text-xs text-zinc-500">
                  Acknowledged {{ formatDateTime(event.acknowledgedAt) }}<span v-if="event.acknowledgedBy"> by {{ event.acknowledgedBy }}</span>
                </div>
              </PCard>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
