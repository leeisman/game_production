<script setup lang="ts">
import { ref, onMounted } from 'vue'
import GenericPanel from './components/GenericPanel.vue'
import ServiceStatus from './components/ServiceStatus.vue'

const currentTab = ref('services')
const projects = ref<any[]>([])
const currentProject = ref('')

async function fetchProjects() {
  try {
    const res = await fetch('/api/projects')
    const data = await res.json()
    projects.value = data
    if (data.length > 0 && !currentProject.value) {
      currentProject.value = data[0].id
    }
  } catch (e) {
    console.error("Failed to fetch projects", e)
  }
}

onMounted(() => {
  fetchProjects()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50 font-sans text-gray-900">
    <!-- Navbar -->
    <nav class="bg-white border-b shadow-sm sticky top-0 z-50">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex justify-between h-16">
          <div class="flex items-center gap-8">
            <div class="flex-shrink-0 flex items-center gap-2">
              <div class="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center text-white font-bold">O</div>
              <h1 class="text-xl font-bold text-gray-900 tracking-tight">OPS Center</h1>
            </div>

            <!-- Project Selector -->
            <div class="flex items-center gap-2">
                <span class="text-xs text-gray-500 uppercase font-bold tracking-wider">Game:</span>
                <select 
                    v-model="currentProject"
                    class="block w-48 pl-3 pr-8 py-1.5 text-sm border-gray-300 focus:outline-none focus:ring-blue-500 focus:border-blue-500 rounded-md border bg-gray-50"
                >
                    <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
                </select>
            </div>

            <div class="hidden sm:flex sm:space-x-8">
              <button 
                @click="currentTab = 'services'"
                class="inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium transition-colors duration-200"
                :class="currentTab === 'services' ? 'border-blue-500 text-gray-900' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'"
              >
                Service Discovery
              </button>
              <button 
                 @click="currentTab = 'rpc'"
                 class="inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium transition-colors duration-200"
                 :class="currentTab === 'rpc' ? 'border-blue-500 text-gray-900' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'"
              >
                RPC Console
              </button>
            </div>
          </div>
          <div class="flex items-center">
             <span class="text-xs text-gray-400">v2.0.0</span>
          </div>
        </div>
      </div>
    </nav>

    <!-- Content -->
    <main class="py-8 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 space-y-6">
      <transition 
        enter-active-class="transition ease-out duration-200" 
        enter-from-class="transform opacity-0 scale-95" 
        enter-to-class="transform opacity-100 scale-100"
        leave-active-class="transition ease-in duration-75" 
        leave-from-class="transform opacity-100 scale-100" 
        leave-to-class="transform opacity-0 scale-95"
        mode="out-in"
      >
        <ServiceStatus v-if="currentTab === 'services' && currentProject" :projectId="currentProject" :key="currentProject" />
        <GenericPanel v-else-if="currentTab === 'rpc' && currentProject" :projectId="currentProject" :key="currentProject" />
        <div v-else class="text-center py-20 text-gray-400">
            <p v-if="projects.length === 0">Loading configuration...</p>
            <p v-else>Please select a project.</p>
        </div>
      </transition>
    </main>
  </div>
</template>
