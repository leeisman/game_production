<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'

const props = defineProps<{ projectId: string }>()

const services = ref<any[]>([])
const loading = ref(false)
const error = ref('')

async function fetchServices() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(`/api/services?project=${props.projectId}`)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error)
    services.value = data
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

watch(() => props.projectId, () => {
    fetchServices()
})

onMounted(() => {
  fetchServices()
})
</script>

<template>
  <div class="bg-white p-6 rounded-lg shadow-lg transition-all duration-500">
    <div class="flex justify-between items-center mb-4 border-b pb-2">
       <h2 class="text-xl font-bold text-gray-800 flex items-center gap-2">
         <span>🌐</span> Service Discovery Status
       </h2>
       <button @click="fetchServices" class="text-sm bg-gray-100 hover:bg-gray-200 px-3 py-1 rounded border transition-colors">Refresh</button>
    </div>
    
    <div v-if="loading" class="text-center py-8 text-gray-500 animate-pulse">Loading services...</div>
    <div v-if="error" class="text-red-600 bg-red-50 p-4 rounded border border-red-200">{{ error }}</div>

    <div v-if="!loading && !error">
      <table class="w-full text-left border-collapse">
        <thead>
          <tr class="bg-gray-50 text-gray-600 border-b uppercase text-xs tracking-wider">
            <th class="p-3">Service Name</th>
            <th class="p-3">Instances</th>
            <th class="p-3">Addresses</th>
            <th class="p-3">Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="svc in services" :key="svc.name" class="border-b hover:bg-blue-50 transition-colors">
            <td class="p-3 font-medium text-gray-900">{{ svc.name }}</td>
            <td class="p-3 text-gray-600">{{ svc.count }}</td>
            <td class="p-3 font-mono text-xs text-gray-500">
              <div v-for="addr in svc.instances" :key="addr">{{ addr }}</div>
            </td>
            <td class="p-3">
              <span v-if="svc.count > 0" class="bg-green-100 text-green-800 px-2 py-1 rounded text-xs font-bold border border-green-200">HEALTHY</span>
              <span v-else class="bg-red-100 text-red-800 px-2 py-1 rounded text-xs font-bold border border-red-200">DOWN</span>
            </td>
          </tr>
          <tr v-if="services.length === 0">
            <td colspan="4" class="p-8 text-center text-gray-500 italic">No services found in Nacos.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
