<script setup>
import { computed, ref } from 'vue'
import { buildDependencyGraph } from '@/utils/dependencyGraph'

const props = defineProps({
  rootModule: { type: String, required: true },
  rootTag: { type: String, default: '' },
  dependencies: { type: Array, default: () => [] },
  driftStatuses: { type: Array, default: () => [] },
  edgesByParent: { type: Object, default: () => ({}) },
  resolvingEdges: { type: Boolean, default: false }
})

const emit = defineEmits(['select'])
const hoveredNodeId = ref(null)

const SEVERITY_STYLES = {
  ROOT: {
    stroke: '#3b82f6',
    badgeBg: 'rgba(59, 130, 246, 0.16)',
    badgeText: '#93c5fd',
    label: 'CURRENT'
  },
  HEALTHY: {
    stroke: '#10b981',
    badgeBg: 'rgba(16, 185, 129, 0.14)',
    badgeText: '#6ee7b7',
    label: 'HEALTHY'
  },
  INFO: {
    stroke: '#0ea5e9',
    badgeBg: 'rgba(14, 165, 233, 0.14)',
    badgeText: '#7dd3fc',
    label: 'INFO'
  },
  WARNING: {
    stroke: '#f59e0b',
    badgeBg: 'rgba(245, 158, 11, 0.14)',
    badgeText: '#fcd34d',
    label: 'WARNING'
  },
  CRITICAL: {
    stroke: '#ef4444',
    badgeBg: 'rgba(239, 68, 68, 0.16)',
    badgeText: '#fca5a5',
    label: 'CRITICAL'
  }
}

const graph = computed(() =>
  buildDependencyGraph({
    rootModule: props.rootModule,
    rootTag: props.rootTag,
    dependencies: props.dependencies,
    driftStatuses: props.driftStatuses,
    edgesByParent: props.edgesByParent
  })
)

const styleFor = (severity) => SEVERITY_STYLES[severity] || SEVERITY_STYLES.HEALTHY

const isEdgeHighlighted = (edge) => {
  if (!hoveredNodeId.value) return true
  return edge.from === hoveredNodeId.value || edge.to === hoveredNodeId.value
}

const handleSelect = (node) => {
  if (node.isRoot || !node.route) return
  emit('select', node)
}

const handleKeydown = (event, node) => {
  if (node.isRoot) return
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    handleSelect(node)
  }
}
</script>

<template>
  <div class="space-y-4" data-testid="dependency-graph">
    <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-zinc-400">
      <div class="flex items-center gap-3 flex-wrap">
        <span>
          {{ graph.counts.direct }} direct • {{ graph.counts.transitive }} transitive
        </span>
        <span v-if="resolvingEdges" class="text-zinc-500">• Resolving transitive edges…</span>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <span class="inline-flex items-center gap-1.5">
          <span class="w-2.5 h-2.5 rounded-full bg-emerald-500"></span>
          <span>Healthy</span>
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="w-2.5 h-2.5 rounded-full bg-sky-500"></span>
          <span>Info</span>
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="w-2.5 h-2.5 rounded-full bg-amber-500"></span>
          <span>Warning</span>
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="w-2.5 h-2.5 rounded-full bg-red-500"></span>
          <span>Critical</span>
        </span>
        <span class="inline-flex items-center gap-1.5 text-zinc-500">
          <span class="w-4 border-t border-dashed border-zinc-400"></span>
          <span>Transitive</span>
        </span>
      </div>
    </div>

    <div class="bg-surface-base border border-surface-border rounded-xl overflow-x-auto p-2">
      <svg
        :viewBox="`0 0 ${graph.width} ${graph.height}`"
        :style="{ minWidth: `${graph.width}px`, height: `${graph.height}px` }"
        class="block w-full"
        role="img"
        :aria-label="`Dependency graph for ${rootModule}@${rootTag}`"
      >
        <defs>
          <marker
            v-for="key in ['HEALTHY', 'INFO', 'WARNING', 'CRITICAL']"
            :id="`dep-arrow-${key}`"
            :key="key"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M 0 1 L 10 5 L 0 9 z" :fill="styleFor(key).stroke" />
          </marker>
        </defs>

        <!-- Directed Edges -->
        <g>
          <path
            v-for="edge in graph.edges"
            :key="edge.id"
            :d="edge.path"
            fill="none"
            :stroke="styleFor(edge.severity).stroke"
            :stroke-width="hoveredNodeId && isEdgeHighlighted(edge) ? 2.5 : 1.75"
            :stroke-dasharray="edge.dashed ? '6 4' : undefined"
            :stroke-opacity="isEdgeHighlighted(edge) ? 0.85 : 0.2"
            :marker-end="`url(#dep-arrow-${edge.severity})`"
            :data-edge-severity="edge.severity"
            class="transition-opacity duration-150"
          />
        </g>

        <!-- Module Nodes -->
        <g
          v-for="node in graph.nodes"
          :key="node.id"
          :transform="`translate(${node.x}, ${node.y})`"
          :role="node.isRoot ? undefined : 'link'"
          :tabindex="node.isRoot ? undefined : 0"
          :aria-label="node.isRoot ? `${node.name}@${node.tag} (current)` : `Open ${node.name}@${node.tag} (${styleFor(node.severity).label})`"
          :data-node-id="node.id"
          :data-severity="node.severity"
          :data-dependency-type="node.dependencyType"
          :class="!node.isRoot ? 'cursor-pointer focus:outline-none' : ''"
          @mouseenter="hoveredNodeId = node.id"
          @mouseleave="hoveredNodeId = null"
          @focus="hoveredNodeId = node.id"
          @blur="hoveredNodeId = null"
          @click="handleSelect(node)"
          @keydown="handleKeydown($event, node)"
        >
          <title>{{ node.name }}@{{ node.tag }} ({{ styleFor(node.severity).label }})</title>

          <rect
            :width="node.width"
            :height="node.height"
            rx="10"
            fill="#18181b"
            :stroke="styleFor(node.severity).stroke"
            :stroke-width="hoveredNodeId === node.id ? 2.25 : 1.5"
          />

          <!-- Module Name -->
          <text
            x="14"
            y="27"
            fill="#f4f4f5"
            font-size="13"
            font-weight="600"
            font-family="JetBrains Mono, Fira Code, monospace"
          >
            {{ node.displayName }}
          </text>

          <!-- Version Tag -->
          <text
            x="14"
            y="53"
            fill="#a1a1aa"
            font-size="11.5"
            font-family="JetBrains Mono, Fira Code, monospace"
          >
            {{ node.displayTag }}
            <tspan v-if="node.dependencyType === 'transitive'" fill="#71717a"> • transitive</tspan>
          </text>

          <!-- Severity / Root Pill -->
          <g :transform="`translate(${node.width - 82}, 40)`">
            <rect
              width="70"
              height="20"
              rx="5"
              :fill="styleFor(node.severity).badgeBg"
            />
            <text
              x="35"
              y="13.5"
              text-anchor="middle"
              :fill="styleFor(node.severity).badgeText"
              font-size="9.5"
              font-weight="600"
              font-family="Inter, system-ui, sans-serif"
            >
              {{ styleFor(node.severity).label }}
            </text>
          </g>
        </g>
      </svg>
    </div>

    <p class="text-xs text-zinc-500">
      Click any dependency node to inspect its version details.
    </p>
  </div>
</template>
