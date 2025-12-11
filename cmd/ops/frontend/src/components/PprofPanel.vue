<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';

const props = defineProps<{ projectId: string }>()

interface ServiceInfo {
  name: string
  instances: string[]
  count: number
}

const services = ref<ServiceInfo[]>([])
const selectedService = ref('')
const selectedInstance = ref('')
const duration = ref(60)
const loading = ref(false)
const recording = ref(false)
const error = ref('')
const result = ref<any>(null)
const history = ref<any[]>([])
const selectedItems = ref(new Set<string>())
const showControls = ref(false)

const uniqueInstances = computed(() => {
    const svc = services.value.find(s => s.name === selectedService.value)
    return svc ? svc.instances : []
})

// Helper for URL Safe Base64
function getAnalysisUrl(path: string) {
    const base64 = btoa(path).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
    return `/api/performance/ui/${base64}/`
}

function toggleAll(e: Event) {
    const checked = (e.target as HTMLInputElement).checked
    if (checked) {
        history.value.forEach(item => selectedItems.value.add(item.folder))
    } else {
        selectedItems.value.clear()
    }
}

function toggleItem(folder: string) {
    if (selectedItems.value.has(folder)) {
        selectedItems.value.delete(folder)
    } else {
        selectedItems.value.add(folder)
    }
}

async function deleteRecords() {
    if (selectedItems.value.size === 0) return
    if (!confirm(`Are you sure you want to delete ${selectedItems.value.size} recordings?`)) return

    try {
        const folders = Array.from(selectedItems.value)
        const res = await fetch('/api/performance/history', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ folders })
        })
        if (res.ok) {
            selectedItems.value.clear()
            fetchHistory()
        } else {
            alert('Failed to delete')
        }
    } catch (e) {
        console.error(e)
        alert('Error deleting')
    }
}

async function fetchServices() {
  try {
    const res = await fetch(`/api/services?project=${props.projectId}`)
    const data = await res.json()
    services.value = data
    if (data.length > 0 && !selectedService.value) {
        selectedService.value = data[0].name
        if (data[0].instances.length > 0) {
            selectedInstance.value = data[0].instances[0]
        }
    }
  } catch (e) {
    console.error(e)
  }
}

async function fetchHistory() {
  try {
    const res = await fetch('/api/performance/history')
    const data = await res.json()
    // Sort by timestamp desc
    data.sort((a: any, b: any) => b.timestamp - a.timestamp)
    history.value = data
  } catch (e) {
    console.error(e)
  }
}

// Watch service change to update instance list default
watch(selectedService, (newVal) => {
    const svc = services.value.find(s => s.name === newVal)
    if (svc && svc.instances.length > 0) {
        selectedInstance.value = svc.instances[0]
    } else {
        selectedInstance.value = ''
    }
})

async function startRecording() {
  if (!selectedService.value) return
  
  loading.value = true
  recording.value = true
  error.value = ''
  result.value = null
  
  try {
    const res = await fetch('/api/performance/record', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({
        project: props.projectId,
        service: selectedService.value,
        instance: selectedInstance.value,
        duration: duration.value
      })
    })

    const data = await res.json()
    if (!res.ok) {
      throw new Error(data.error || 'Recording failed')
    }
    result.value = data
    fetchHistory() // Refresh history
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
    recording.value = false
  }
}

onMounted(() => {
  fetchServices()
  fetchHistory()
})
</script>

<template>
  <div class="h-full flex flex-col p-6 space-y-6 bg-gray-50 dark:bg-gray-900">
    <div class="flex flex-col space-y-4">
      
      <!-- Top Bar: Toggle & Status -->
      <div class="flex items-center justify-between">
         <h2 class="text-2xl font-bold text-gray-900 dark:text-white">Performance Monitor (pprof)</h2>
         <button @click="showControls = !showControls" 
                 class="px-4 py-2 text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 transition flex items-center gap-2">
                 <span v-if="!showControls">➕ New Recording</span>
                 <span v-else>➖ Hide Controls</span>
         </button>
      </div>

      <div class="flex gap-6 items-start">
         
         <!-- Left Control Panel (Collapsible) -->
         <div v-show="showControls" class="w-full md:w-1/3 space-y-6 bg-white dark:bg-gray-800 p-6 rounded-lg shadow transition-all duration-300">
            <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Service</label>
                <select v-model="selectedService" class="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2">
                    <option v-for="s in services" :key="s.name" :value="s.name">
                        {{ s.name }} ({{ s.count }})
                    </option>
                </select>
            </div>

            <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Instance (Target)</label>
                <select v-model="selectedInstance" class="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2">
                    <option value="">Any (Load Balanced)</option>
                    <option v-for="inst in uniqueInstances" :key="inst" :value="inst">
                        {{ inst }}
                    </option>
                </select>
            </div>

            <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Duration (seconds)</label>
                <input v-model.number="duration" type="number" min="1" max="600" class="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2">
            </div>

            <button @click="startRecording" :disabled="loading"
                class="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 disabled:opacity-50 transition">
                <span v-if="loading">Recording...</span>
                <span v-else>Start Profiling</span>
            </button>

            <div v-if="result" class="mt-4 p-4 rounded bg-green-50 dark:bg-green-900 text-green-700 dark:text-green-200 text-sm">
                <p>✅ Profiling Complete!</p>
                <p class="text-xs mt-1">Files saved successfully.</p>
            </div>
            
            <div v-if="error" class="mt-4 p-4 rounded bg-red-50 dark:bg-red-900 text-red-700 dark:text-red-200 text-sm">
                ❌ {{ error }}
            </div>
         </div>

         <!-- History List (Full Width when collapsed) -->
         <div :class="showControls ? 'w-full md:w-2/3' : 'w-full'" class="transition-all duration-300">
            <div class="mb-4 flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <h3 class="text-lg font-medium leading-6 text-gray-900 dark:text-gray-100 flex items-center gap-2">
                    📦 History
                    <span class="text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 py-0.5 px-2 rounded-full">{{ history.length }}</span>
                    </h3>
                    
                    <button v-if="selectedItems.size > 0" 
                            @click="deleteRecords"
                            class="px-3 py-1 text-sm font-semibold rounded bg-red-600 text-white hover:bg-red-700 transition shadow-sm flex items-center gap-1">
                            🗑 Delete ({{ selectedItems.size }})
                    </button>
                    
                     <div class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1 bg-blue-50 dark:bg-blue-900/30 px-2 py-1 rounded">
                        <span class="text-lg">ℹ️</span>
                        <span><strong>Full Analysis</strong> opens an interactive view (FlameGraph, CallGraph, Source, Peek) in a new tab.</span>
                    </div>
                </div>

                <button @click="fetchHistory" class="text-sm text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300">
                    Refresh
                </button>
            </div>
            
            <div class="overflow-x-auto rounded-lg border dark:border-gray-700 bg-white dark:bg-gray-800 shadow">
                <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                    <thead class="bg-gray-50 dark:bg-gray-800">
                        <tr>
                            <th class="px-6 py-3 text-left w-10">
                                <input type="checkbox" 
                                       :checked="history.length > 0 && selectedItems.size === history.length"
                                       @change="toggleAll"
                                       class="rounded border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50 dark:bg-gray-700 dark:border-gray-600">
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Time</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Service</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Instance</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Downloads</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Analysis</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                        <tr v-if="history.length === 0">
                            <td colspan="6" class="px-6 py-4 text-center text-gray-500">No recordings found.</td>
                        </tr>
                        <tr v-for="item in history" :key="String(item.timestamp)" class="hover:bg-gray-50 dark:hover:bg-gray-800 transition">
                            <td class="px-6 py-4 whitespace-nowrap">
                                <input type="checkbox" 
                                       :checked="selectedItems.has(item.folder)"
                                       @change="toggleItem(item.folder)"
                                       class="rounded border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50 dark:bg-gray-700 dark:border-gray-600">
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-gray-500 dark:text-gray-400">
                                {{ new Date(item.timestamp * 1000).toLocaleString() }}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap font-medium text-gray-900 dark:text-gray-100">
                                {{ item.service_name }}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-gray-500 text-xs font-mono">
                                {{ item.instance || 'unknown' }}
                            </td>
                            <!-- Downloads Column -->
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="flex flex-wrap gap-2">
                                    <a v-for="(path, type) in item.files" :key="type" :href="`/api/performance/download/${path}`" 
                                       target="_blank"
                                       class="px-2 py-1 text-xs font-semibold rounded bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 uppercase border dark:border-gray-600"
                                       :title="'Download ' + String(type)">
                                       ⬇ {{ type }}
                                    </a>
                                </div>
                            </td>
                            <!-- Analysis Column -->
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="flex flex-wrap gap-2">
                                     <template v-for="(path, type) in item.files" :key="type">
                                        <a v-if="['cpu', 'heap', 'trace', 'block', 'mutex'].includes(String(type))" 
                                           :href="getAnalysisUrl(path)"
                                           target="_blank"
                                           class="px-2 py-1 text-xs font-semibold rounded bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200 hover:bg-indigo-200 dark:hover:bg-indigo-800 uppercase flex items-center gap-1 border border-indigo-200 dark:border-indigo-800 bg-opacity-50"
                                           title="Opens FlameGraph, Top, Graph, Peek views in new tab">
                                           📊 {{ String(type).toUpperCase() }} Analysis
                                        </a>
                                    </template>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
         </div>

      </div>

    </div>
  </div>
</template>
