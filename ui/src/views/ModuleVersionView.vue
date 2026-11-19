<script setup>
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PCard from '@/components/PCard.vue'
import PBadge from '@/components/PBadge.vue'
import PButton from '@/components/PButton.vue'
import PCodeBlock from '@/components/PCodeBlock.vue'
import PDependencyGraph from '@/components/PDependencyGraph.vue'
import { registryApi } from '@/services/registryApi'
import { metadataApi } from '@/services/metadataApi'
import { driftApi } from '@/services/driftApi'
import { dedupeDriftStatuses, dependencyNodeId } from '@/utils/dependencyGraph'
import { authToken } from '@/auth/authToken'
import {
  canAcknowledgeDriftEvent,
  getAcknowledgeDriftErrorMessage,
  replaceDriftEvent
} from '@/utils/driftAcknowledgeUtils'
import {
  buildVendorSnippet,
  copyTextToClipboard,
  diffProtoFileSets,
  formatProtoViewer,
  pickDefaultBaseTag
} from '@/utils/protoDiff'
import {
  flattenMetadataMessages
} from '@/utils/metadataUtils'

const MAX_GRAPH_EDGE_PARENTS = 20

const route = useRoute()
const router = useRouter()
const protoFiles = ref([])
const dependencies = ref([])
const metadata = ref(null)
const metadataPending = ref(false)
const isDraftTag = ref(false)
const loading = ref(true)
const loadingProtoFiles = ref(false)
const protoFilesLoaded = ref(false)
const activeTab = ref('files')
const error = ref(null)
const protoFilesError = ref(null)
const copiedFilename = ref('')
let copyResetTimer = null

const clearCopyFeedback = () => {
  if (copyResetTimer) {
    clearTimeout(copyResetTimer)
    copyResetTimer = null
  }
  copiedFilename.value = ''
}

const copyProtoFile = async (file) => {
  if (!file?.filename) return
  if (!(await copyTextToClipboard(file.content))) return
  if (copyResetTimer) clearTimeout(copyResetTimer)
  copiedFilename.value = file.filename
  copyResetTimer = setTimeout(() => {
    copiedFilename.value = ''
    copyResetTimer = null
  }, 2000)
}

const displayProtoFiles = computed(() =>
  (protoFiles.value || []).map((file) => ({
    ...file,
    ...formatProtoViewer(file?.content)
  }))
)

const compareTags = ref([])
const compareTagsLoaded = ref(false)
const compareBaseTag = ref('')
const compareBaseFiles = ref([])
const compareLoading = ref(false)
const compareError = ref(null)
let compareProtoCache = new Map()
let protoFilesPromise = null

const driftEvents = ref([])
const driftLoading = ref(false)
const driftLoaded = ref(false)
const driftError = ref(null)
const dependencyDriftStatuses = ref([])
const dependencyDriftError = ref(null)
const isLoggedIn = authToken.isLoggedIn
const acknowledgingDriftEventId = ref(null)
const driftAcknowledgeError = ref(null)

const graphEdgesByParent = ref({})
const graphTransitiveDriftStatuses = ref([])
const graphEdgesLoaded = ref(false)
const graphEdgesError = ref(null)
const resolvingGraphEdges = ref(false)

const moduleName = computed(() => {
  const name = route.params.name
  return Array.isArray(name) ? name.join('/') : name
})
const tag = computed(() => route.params.tag)

watch([moduleName, tag], (newVals, oldVals) => {
  // Reset state when navigating between tags without recreating the component
  if (!oldVals || newVals[0] !== oldVals[0]) {
    compareProtoCache = new Map()
    compareTags.value = []
    compareTagsLoaded.value = false
  }
  protoFiles.value = []
  dependencies.value = []
  metadata.value = null
  metadataPending.value = false
  isDraftTag.value = false
  error.value = null
  protoFilesError.value = null
  clearCopyFeedback()
  loadingProtoFiles.value = false
  protoFilesPromise = null

  protoFilesLoaded.value = false
  compareBaseTag.value = ''
  compareBaseFiles.value = []
  compareLoading.value = false
  compareError.value = null
  driftLoaded.value = false
  driftEvents.value = []
  driftError.value = null
  acknowledgingDriftEventId.value = null
  driftAcknowledgeError.value = null
  dependencyDriftStatuses.value = []
  dependencyDriftError.value = null
  graphEdgesByParent.value = {}
  graphTransitiveDriftStatuses.value = []
  graphEdgesLoaded.value = false
  graphEdgesError.value = null
  resolvingGraphEdges.value = false

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

const allDependencyDriftStatuses = computed(() =>
  dedupeDriftStatuses([
    ...(dependencyDriftStatuses.value || []),
    ...(graphTransitiveDriftStatuses.value || [])
  ])
)

const dependencyDriftByKey = computed(() => {
  const map = new Map()
  for (const status of allDependencyDriftStatuses.value) {
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
  for (const status of allDependencyDriftStatuses.value) {
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

const loadTransitiveGraphEdges = async () => {
  if (graphEdgesLoaded.value || resolvingGraphEdges.value) return
  const hasTransitive = (dependencies.value || []).some((d) => d?.dependencyType === 'transitive')
  if (!hasTransitive) {
    graphEdgesError.value = null
    graphEdgesLoaded.value = true
    return
  }

  const currentModule = moduleName.value
  const currentTag = tag.value
  resolvingGraphEdges.value = true
  graphEdgesError.value = null
  try {
    const parents = (dependencies.value || [])
      .filter((d) => d?.name && d?.tag)
      .slice(0, MAX_GRAPH_EDGE_PARENTS)

    const results = await Promise.allSettled(
      parents.map(async (dep) => {
        const [subDeps, subDrift] = await Promise.allSettled([
          registryApi.getModuleDependencies(dep.name, dep.tag, false),
          driftApi.getModuleDependencyDriftStatus(dep.name, dep.tag)
        ])
        return {
          key: dependencyNodeId(dep.name, dep.tag),
          failed: subDeps.status !== 'fulfilled' || subDrift.status !== 'fulfilled',
          deps: subDeps.status === 'fulfilled' ? (subDeps.value?.dependencies || []) : [],
          statuses: subDrift.status === 'fulfilled' ? (subDrift.value?.statuses || []) : []
        }
      })
    )

    if (moduleName.value !== currentModule || tag.value !== currentTag) return

    const nextEdges = {}
    const extraStatuses = []
    let hadFailure = false
    for (const r of results) {
      if (r.status !== 'fulfilled') {
        hadFailure = true
        continue
      }
      if (r.value.failed) {
        hadFailure = true
      }
      if (r.value.deps.length > 0) {
        nextEdges[r.value.key] = r.value.deps
      }
      extraStatuses.push(...r.value.statuses)
    }
    graphEdgesByParent.value = nextEdges
    graphTransitiveDriftStatuses.value = dedupeDriftStatuses(extraStatuses)
    if (hadFailure) {
      graphEdgesError.value = 'Some transitive dependency details or drift statuses could not be loaded.'
      graphEdgesLoaded.value = false
    } else {
      graphEdgesLoaded.value = true
    }
  } finally {
    if (moduleName.value === currentModule && tag.value === currentTag) {
      resolvingGraphEdges.value = false
    }
  }
}

const onSelectGraphNode = (node) => {
  if (node?.route) {
    router.push(node.route)
  }
}

const loadVersionDetails = async () => {
  const currentModule = moduleName.value
  const currentTag = tag.value
  const isSameTag = () => moduleName.value === currentModule && tag.value === currentTag

  loading.value = true
  error.value = null
  metadataPending.value = false
  isDraftTag.value = false
  dependencyDriftStatuses.value = []
  dependencyDriftError.value = null
  graphEdgesByParent.value = {}
  graphTransitiveDriftStatuses.value = []
  graphEdgesLoaded.value = false
  graphEdgesError.value = null
  resolvingGraphEdges.value = false
  try {
    // Get metadata (parsed proto structure) - Level 3. Newly pushed tags return 404 (TAG_META_NOT_FOUND)
    // until background proto-parsing runs; draft tags return 404 (MODULE_TAG_NOT_FOUND) while pullModule succeeds.
    try {
      const metadataResponse = await metadataApi.getMetadata(currentModule, currentTag)
      if (!isSameTag()) return
      metadata.value = metadataResponse
    } catch (metaErr) {
      if (!isSameTag()) return
      const status = metaErr?.response?.status
      const reason = metaErr?.response?.data?.reason
      if (status === 404 && reason === 'TAG_META_NOT_FOUND') {
        metadata.value = null
        metadataPending.value = true
      } else if (status === 404 && reason === 'MODULE_TAG_NOT_FOUND') {
        await loadProtoFiles()
        if (!isSameTag()) return
        if (protoFilesLoaded.value) {
          isDraftTag.value = true
          metadata.value = null
          dependencies.value = []
          return
        }
        if (protoFilesError.value) {
          throw new Error(protoFilesError.value)
        }
        throw metaErr
      } else {
        throw metaErr
      }
    }

    // Get direct + transitive dependencies
    const depsResponse = await registryApi.getModuleDependencies(currentModule, currentTag, true)
    if (!isSameTag()) return
    dependencies.value = depsResponse.dependencies || []

    try {
      const driftStatusResponse = await driftApi.getModuleDependencyDriftStatus(currentModule, currentTag)
      if (!isSameTag()) return
      dependencyDriftStatuses.value = driftStatusResponse.statuses || []
    } catch (err) {
      if (!isSameTag()) return
      console.error('Failed to load dependency drift status:', err)
      dependencyDriftStatuses.value = []
      dependencyDriftError.value = err.response?.data?.message || err.message || 'Failed to load dependency drift status'
    }
  } catch (err) {
    if (!isSameTag()) return
    console.error('Failed to load version details:', err)
    error.value = err.response?.data?.message || err.message || 'Failed to load version details'
  } finally {
    if (isSameTag()) {
      loading.value = false
    }
  }
}

const loadProtoFiles = () => {
  if (protoFilesLoaded.value) return Promise.resolve()
  if (protoFilesPromise) return protoFilesPromise

  const currentModule = moduleName.value
  const currentTag = tag.value
  loadingProtoFiles.value = true
  protoFilesError.value = null

  const p = (async () => {
    try {
      // Pull module to get proto files - only when needed
      const pullResponse = await registryApi.pullModule(currentModule, currentTag)
      const files = pullResponse.protofiles || []
      if (moduleName.value === currentModule && tag.value === currentTag) {
        protoFiles.value = files
        protoFilesLoaded.value = true
        compareProtoCache.set(currentTag, files)
      }
    } catch (err) {
      if (moduleName.value === currentModule && tag.value === currentTag) {
        console.error('Failed to load proto files:', err)
        protoFilesError.value = err.response?.data?.message || err.message || 'Failed to load proto files'
      }
    } finally {
      if (protoFilesPromise === p) {
        protoFilesPromise = null
      }
      if (moduleName.value === currentModule && tag.value === currentTag) {
        loadingProtoFiles.value = false
      }
    }
  })()

  protoFilesPromise = p
  return p
}

const loadDriftEvents = async () => {
  if (driftLoaded.value) return

  const currentModule = moduleName.value
  const currentTag = tag.value
  const isSameTag = () => moduleName.value === currentModule && tag.value === currentTag

  driftLoading.value = true
  driftError.value = null
  try {
    const response = await driftApi.getModuleDriftEvents(currentModule, currentTag)
    if (!isSameTag()) return
    driftEvents.value = response.events || []
    driftLoaded.value = true
  } catch (err) {
    if (!isSameTag()) return
    console.error('Failed to load drift events:', err)
    driftError.value = err.response?.data?.message || err.message || 'Failed to load drift events'
  } finally {
    if (isSameTag()) {
      driftLoading.value = false
    }
  }
}

const retryVersionView = () => {
  loadVersionDetails()
  loadProtoFiles()
  retryLoadDriftEvents()
}

const acknowledgeDriftEvent = async (event) => {
  if (!canAcknowledgeDriftEvent({ isLoggedIn: isLoggedIn.value, event, pendingEventId: acknowledgingDriftEventId.value })) return

  const currentModule = moduleName.value
  const currentTag = tag.value
  const isSameTag = () => moduleName.value === currentModule && tag.value === currentTag

  acknowledgingDriftEventId.value = event.id
  driftAcknowledgeError.value = null
  try {
    const response = await driftApi.acknowledgeDriftEvent(event.id)
    // The user navigated to another tag while the request was in flight: drop the late response.
    if (!isSameTag()) return
    driftEvents.value = replaceDriftEvent(driftEvents.value, event.id, response)
  } catch (err) {
    if (!isSameTag()) return
    console.error('Failed to acknowledge drift event:', err)
    driftAcknowledgeError.value = getAcknowledgeDriftErrorMessage(err)
  } finally {
    if (isSameTag()) acknowledgingDriftEventId.value = null
  }
}

const compareCandidateTags = computed(() =>
  (compareTags.value || []).filter((t) => t && t !== tag.value)
)

const compareDiff = computed(() => {
  if (!compareBaseTag.value || compareLoading.value || compareError.value || !protoFilesLoaded.value) {
    return null
  }
  return diffProtoFileSets(compareBaseFiles.value, protoFiles.value)
})

const diffStatusVariant = (status) => {
  if (status === 'added') return 'info'
  if (status === 'deleted') return 'critical'
  return 'warning'
}

const loadCompareBaseFiles = async (baseTag) => {
  if (!baseTag) {
    compareBaseFiles.value = []
    compareError.value = null
    compareLoading.value = false
    return
  }
  const currentModule = moduleName.value
  const currentTag = tag.value
  compareLoading.value = true
  compareError.value = null
  try {
    if (!protoFilesLoaded.value) {
      await loadProtoFiles()
    }
    if (moduleName.value !== currentModule || tag.value !== currentTag) return
    if (!protoFilesLoaded.value) {
      throw new Error(protoFilesError.value || 'Failed to load proto files')
    }
    let files = compareProtoCache.get(baseTag)
    if (!files) {
      const pullResp = await registryApi.pullModule(currentModule, baseTag)
      files = pullResp.protofiles || []
      if (moduleName.value === currentModule) {
        compareProtoCache.set(baseTag, files)
      }
    }
    if (moduleName.value === currentModule && tag.value === currentTag && compareBaseTag.value === baseTag) {
      compareBaseFiles.value = files
    }
  } catch (err) {
    if (moduleName.value === currentModule && tag.value === currentTag && compareBaseTag.value === baseTag) {
      console.error('Failed to load baseline tag for comparison:', err)
      compareError.value = err.response?.data?.message || err.message || 'Failed to load baseline tag'
    }
  } finally {
    if (moduleName.value === currentModule && tag.value === currentTag && compareBaseTag.value === baseTag) {
      compareLoading.value = false
    }
  }
}

const loadCompareView = async () => {
  const currentModule = moduleName.value
  const currentTag = tag.value
  compareError.value = null
  try {
    if (!compareTagsLoaded.value) {
      compareLoading.value = true
      const mod = await registryApi.getModule(currentModule, true)
      if (moduleName.value !== currentModule || tag.value !== currentTag) return
      const merged = Array.from(new Set([...(mod.tags || []), ...(mod.draftTags || [])]))
      compareTags.value = merged
      compareTagsLoaded.value = true
    }
    if (!compareBaseTag.value || compareBaseTag.value === currentTag) {
      compareBaseTag.value = pickDefaultBaseTag(compareTags.value, currentTag)
    }
    await loadCompareBaseFiles(compareBaseTag.value)
  } catch (err) {
    if (moduleName.value === currentModule && tag.value === currentTag) {
      console.error('Failed to load module tags for comparison:', err)
      compareError.value = err.response?.data?.message || err.message || 'Failed to load module tags'
      compareLoading.value = false
    }
  }
}

const retryLoadDriftEvents = async () => {
  driftLoaded.value = false
  await loadDriftEvents()
}

// Watch activeTab and load lazy tab data on demand
watch(activeTab, (newTab) => {
  if (newTab === 'files' && !protoFilesLoaded.value) {
    loadProtoFiles()
  }
  if (newTab === 'compare') {
    loadCompareView()
  }
  if (newTab === 'graph' && !graphEdgesLoaded.value) {
    loadTransitiveGraphEdges()
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

onBeforeUnmount(() => {
  clearCopyFeedback()
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
          <PButton variant="primary" @click="retryVersionView">
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
            <PBadge :variant="isDraftTag ? 'draft' : 'brand'">{{ tag }}</PBadge>
            <PBadge v-if="isDraftTag" variant="draft">DRAFT</PBadge>
          </div>
        </div>

        <!-- Quick Vendor Command -->
        <div class="mb-8">
          <h2 class="text-2xl font-semibold mb-4">Vendor This Version</h2>
          <PCodeBlock :code="buildVendorSnippet({ moduleName, tag })" />
          <p class="text-zinc-400 text-sm mt-2">
            Learn more: 
            <a href="https://github.com/pbufio/pbuf-registry#cli-reference" target="_blank" rel="noopener noreferrer" class="text-brand hover:underline">
              pbuf CLI Documentation
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
              @click="activeTab = 'compare'"
              :class="[
                'pb-3 px-1 font-medium transition-colors cursor-pointer',
                activeTab === 'compare'
                  ? 'text-brand border-b-2 border-brand'
                  : 'text-zinc-400 hover:text-zinc-200'
              ]"
            >
              Compare
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
              v-if="dependencies.length > 0"
              @click="activeTab = 'graph'"
              :class="[
                'pb-3 px-1 font-medium transition-colors cursor-pointer',
                activeTab === 'graph'
                  ? 'text-brand border-b-2 border-brand'
                  : 'text-zinc-400 hover:text-zinc-200'
              ]"
            >
              Dependency Graph
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
            <PCard v-for="file in displayProtoFiles" :key="file.filename">
              <div class="flex items-center justify-between gap-3 mb-3">
                <h3 class="text-lg font-semibold font-mono text-zinc-100 truncate">{{ file.filename }}</h3>
                <button
                  type="button"
                  :aria-label="`Copy ${file.filename}`"
                  class="text-xs font-mono text-zinc-400 hover:text-zinc-200 cursor-pointer transition-colors px-2.5 py-1 rounded border border-surface-border hover:bg-zinc-800 shrink-0"
                  @click="copyProtoFile(file)"
                >
                  {{ copiedFilename === file.filename ? 'Copied!' : 'Copy' }}
                </button>
              </div>
              <div class="bg-surface-base rounded-lg p-4 flex font-mono text-sm leading-6">
                <pre
                  aria-hidden="true"
                  data-testid="proto-line-numbers"
                  class="select-none text-right pr-4 mr-4 border-r border-surface-border text-zinc-600 shrink-0"
                >{{ file.lineNumbers }}</pre>
                <pre
                  data-testid="proto-file-content"
                  class="text-zinc-300 whitespace-pre overflow-x-auto min-w-0 flex-1"
                >{{ file.code }}</pre>
              </div>
            </PCard>
            <p v-if="displayProtoFiles.length === 0 && !protoFilesError" class="text-zinc-400 text-center py-8">No proto files available.</p>
          </template>
        </div>

        <!-- Compare Tab -->
        <div v-if="activeTab === 'compare'" class="space-y-4" data-testid="compare-tab-panel">
          <PCard>
            <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
              <div>
                <h3 class="text-lg font-semibold text-zinc-100">Compare Schema Against Baseline Tag</h3>
                <p v-if="compareDiff && compareBaseTag" class="text-sm text-zinc-400 mt-1">
                  Comparing <span class="font-mono text-zinc-200">{{ compareBaseTag }}</span> → <span class="font-mono text-zinc-200">{{ tag }}</span>:
                  {{ compareDiff.added }} added, {{ compareDiff.modified }} modified, {{ compareDiff.deleted }} deleted, {{ compareDiff.unchanged }} unchanged
                </p>
                <p v-else-if="compareTagsLoaded && compareCandidateTags.length === 0" class="text-sm text-zinc-400 mt-1">
                  No other tags exist on this module to compare against yet.
                </p>
              </div>
              <div v-if="compareCandidateTags.length > 0" class="flex items-center gap-2">
                <label for="compare-base-tag" class="text-sm text-zinc-400 whitespace-nowrap">Compare against tag</label>
                <select
                  id="compare-base-tag"
                  v-model="compareBaseTag"
                  aria-label="Compare against tag"
                  class="bg-surface-base border border-surface-border rounded-lg px-3 py-1.5 text-sm font-mono text-zinc-100 focus:outline-none focus:border-brand"
                  @change="loadCompareBaseFiles(compareBaseTag)"
                >
                  <option v-for="candidate in compareCandidateTags" :key="candidate" :value="candidate">
                    {{ candidate }}
                  </option>
                </select>
              </div>
            </div>
          </PCard>

          <div v-if="compareLoading" class="text-center py-12">
            <div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-brand"></div>
            <p class="text-zinc-400 mt-4">Loading tag diff...</p>
          </div>
          <PCard v-else-if="compareError" class="text-center py-8">
            <p class="text-zinc-300 font-semibold mb-2">Failed to Load Tag Diff</p>
            <p class="text-zinc-400 text-sm mb-4">{{ compareError }}</p>
            <PButton variant="primary" @click="loadCompareView">Retry</PButton>
          </PCard>
          <template v-else-if="compareDiff && compareBaseTag">
            <p v-if="compareDiff.files.length === 0" class="text-zinc-400 text-center py-8">
              No .proto file differences between {{ compareBaseTag }} and {{ tag }}.
            </p>
            <PCard v-for="fileDiff in compareDiff.files" :key="fileDiff.filename">
              <div class="flex items-center justify-between gap-3 flex-wrap mb-3">
                <div class="flex items-center gap-2">
                  <PBadge :variant="diffStatusVariant(fileDiff.status)">{{ fileDiff.status.toUpperCase() }}</PBadge>
                  <h4 class="font-mono text-sm sm:text-base font-semibold text-zinc-100">{{ fileDiff.filename }}</h4>
                </div>
                <div class="font-mono text-xs flex items-center gap-2">
                  <span class="text-emerald-400">+{{ fileDiff.additions }}</span>
                  <span class="text-red-400">-{{ fileDiff.deletions }}</span>
                </div>
              </div>
              <div class="bg-surface-base rounded-lg overflow-x-auto border border-surface-border">
                <div
                  v-for="(line, idx) in fileDiff.lines"
                  :key="idx"
                  :class="[
                    'grid grid-cols-[3rem_3rem_1.5rem_1fr] font-mono text-xs leading-5 px-2',
                    line.type === 'add'
                      ? 'bg-emerald-950/40 text-emerald-200'
                      : line.type === 'del'
                        ? 'bg-red-950/40 text-red-200'
                        : 'text-zinc-400'
                  ]"
                >
                  <span class="select-none text-right pr-2 text-zinc-600">{{ line.oldLine ?? '' }}</span>
                  <span class="select-none text-right pr-2 text-zinc-600">{{ line.newLine ?? '' }}</span>
                  <span class="select-none text-center">{{ line.type === 'add' ? '+' : line.type === 'del' ? '-' : ' ' }}</span>
                  <span class="whitespace-pre">{{ line.text }}</span>
                </div>
              </div>
            </PCard>
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
                    <PBadge v-if="dep.dependencyType === 'transitive'" variant="draft">TRANSITIVE</PBadge>
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

        <!-- Dependency Graph Tab -->
        <div v-if="activeTab === 'graph'" class="space-y-4">
          <PCard v-if="dependencies.length > 0">
            <p v-if="dependencyDriftError" class="text-sm text-amber-300 mb-4">
              Drift status unavailable: {{ dependencyDriftError }}
            </p>
            <div v-if="graphEdgesError" class="flex items-center justify-between gap-4 text-sm text-amber-300 mb-4">
              <span>{{ graphEdgesError }}</span>
              <button
                type="button"
                class="text-brand hover:underline cursor-pointer shrink-0"
                @click="loadTransitiveGraphEdges"
              >
                Retry
              </button>
            </div>
            <PDependencyGraph
              :root-module="moduleName"
              :root-tag="tag"
              :dependencies="dependencies"
              :drift-statuses="allDependencyDriftStatuses"
              :edges-by-parent="graphEdgesByParent"
              :resolving-edges="resolvingGraphEdges"
              :direct-drift-unavailable="Boolean(dependencyDriftError)"
              :transitive-drift-unavailable="Boolean(graphEdgesError)"
              @select="onSelectGraphNode"
            />
          </PCard>
        </div>

        <!-- Metadata Tab -->
        <div v-if="activeTab === 'metadata'" class="space-y-4">
          <div v-if="metadata && metadata.packages">
            <PCard v-for="(pkg, pkgIdx) in metadata.packages" :key="`${pkg.name}-${pkgIdx}`" class="mb-4">
              <h3 class="text-xl font-bold mb-4 text-brand">Package: {{ pkg.name || '(no package)' }}</h3>

              <!-- Proto Files in Package -->
              <div v-for="protoFile in pkg.protoFiles" :key="protoFile.filename" class="mb-6">
                <h4 class="text-lg font-semibold mb-3 font-mono text-zinc-200">{{ protoFile.filename }}</h4>

                <!-- Messages (including nested messages, enums, maps, and oneofs) -->
                <div v-if="protoFile.messages && protoFile.messages.length > 0" class="mb-4">
                  <h5 class="text-md font-medium mb-2 text-zinc-300">Messages</h5>
                  <div class="space-y-3">
                    <div
                      v-for="message in flattenMetadataMessages(protoFile.messages)"
                      :key="message.key"
                      class="bg-surface-base rounded-lg p-4"
                      :style="message.depth > 0 ? { marginLeft: `${Math.min(message.depth, 4) * 1.25}rem` } : undefined"
                    >
                      <p class="font-mono text-brand mb-3">{{ message.fullName }}</p>
                      <div v-if="message.fields.length > 0" class="pl-6 space-y-1.5">
                        <template v-for="(field, fieldIdx) in message.fields" :key="`${field.name}-${fieldIdx}`">
                          <div v-if="field.kind === 'oneof'" class="text-sm font-mono space-y-1">
                            <div>
                              <span class="text-zinc-500">oneof</span>
                              <span class="text-brand ml-1">{{ field.name }}</span>
                            </div>
                            <div
                              v-for="branch in field.branches"
                              :key="branch.name"
                              class="pl-4 text-zinc-400"
                            >
                              <span class="text-zinc-300">{{ branch.type }}</span>
                              <span class="text-brand ml-1">{{ branch.name }}</span>
                            </div>
                          </div>
                          <div v-else class="text-sm font-mono">
                            <span class="text-zinc-500">{{ field.tag }}:</span>
                            <span class="text-zinc-300 ml-1">{{ field.typeLabel }}</span>
                            <span class="text-brand ml-1">{{ field.name }}</span>
                            <span
                              v-for="mod in field.modifiers"
                              :key="mod"
                              class="text-zinc-500 ml-1"
                            >({{ mod }})</span>
                          </div>
                        </template>
                      </div>
                      <div v-if="message.nestedEnums.length > 0" class="pl-6 mt-3 space-y-2">
                        <div v-for="(enumDef, enumIdx) in message.nestedEnums" :key="`${enumDef.name}-${enumIdx}`" class="text-sm font-mono">
                          <p class="text-zinc-400">enum <span class="text-brand">{{ enumDef.name }}</span></p>
                          <div class="pl-4 space-y-0.5 text-zinc-400">
                            <div v-for="(val, valIdx) in (enumDef.values || [])" :key="`${val.name}-${valIdx}`">
                              <span class="text-zinc-300">{{ val.name }}</span>
                              <span class="text-zinc-500"> = {{ val.tag ?? 0 }}</span>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Top-Level Enums -->
                <div v-if="protoFile.enums && protoFile.enums.length > 0" class="mb-4">
                  <h5 class="text-md font-medium mb-2 text-zinc-300">Enums</h5>
                  <div class="space-y-3">
                    <div
                      v-for="(enumDef, enumIdx) in protoFile.enums"
                      :key="`${enumDef.name}-${enumIdx}`"
                      class="bg-surface-base rounded-lg p-4 font-mono text-sm"
                    >
                      <p class="text-brand mb-2">{{ enumDef.name }}</p>
                      <div v-if="enumDef.values && enumDef.values.length > 0" class="pl-4 space-y-1 text-zinc-400">
                        <div v-for="(val, valIdx) in enumDef.values" :key="`${val.name}-${valIdx}`">
                          <span class="text-zinc-300">{{ val.name }}</span>
                          <span class="text-zinc-500"> = {{ val.tag ?? 0 }}</span>
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
          <p v-else-if="isDraftTag" class="text-zinc-400 text-center py-8">
            Metadata indexing is only available for published tags.
          </p>
          <p v-else-if="metadataPending" class="text-zinc-400 text-center py-8">
            Metadata is pending background indexing — refresh shortly.
          </p>
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

            <p v-if="driftAcknowledgeError" class="text-sm text-amber-300 mb-3">
              Failed to acknowledge drift event: {{ driftAcknowledgeError }}
            </p>

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
                    <PButton
                      v-if="isLoggedIn && !event.acknowledged"
                      variant="secondary"
                      :disabled="acknowledgingDriftEventId !== null"
                      @click="acknowledgeDriftEvent(event)"
                    >
                      {{ acknowledgingDriftEventId === event.id ? 'Acknowledging...' : 'Acknowledge' }}
                    </PButton>
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
