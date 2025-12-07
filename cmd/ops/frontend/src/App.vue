<script setup lang="ts">
import { onMounted, ref } from 'vue'
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

const isDark = ref(localStorage.getItem('theme') === 'dark')

function toggleDark() {
  isDark.value = !isDark.value
  if (isDark.value) {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme', 'dark')
  } else {
    document.documentElement.classList.remove('dark')
    localStorage.setItem('theme', 'light')
  }
}

onMounted(() => {
  fetchProjects()
  if (isDark.value) {
    document.documentElement.classList.add('dark')
  }
})
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900 font-sans text-gray-900 dark:text-gray-100 transition-colors duration-300">
    <!-- Navbar -->
    <nav class="bg-white dark:bg-gray-800 border-b dark:border-gray-700 shadow-sm sticky top-0 z-50 transition-colors duration-300">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex justify-between h-16">
          <div class="flex items-center gap-8">
            <div class="flex-shrink-0 flex items-center gap-2">
              <div class="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center text-white font-bold">O</div>
              <h1 class="text-xl font-bold text-gray-900 dark:text-white tracking-tight">OPS Center</h1>
            </div>

            <!-- Project Selector -->
            <div class="flex items-center gap-2">
                <span class="text-xs text-gray-500 uppercase font-bold tracking-wider">Game:</span>
                <select 
                    v-model="currentProject"
                    class="block w-48 pl-3 pr-8 py-1.5 text-sm border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-blue-500 focus:border-blue-500 rounded-md border bg-gray-50 dark:bg-gray-700 dark:text-white transition-colors duration-300"
                >
                    <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
                </select>
            </div>

            <div class="hidden sm:flex sm:space-x-8">
              <button 
                 @click="currentTab = 'services'"
                class="inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium transition-colors duration-200"
                :class="currentTab === 'services' ? 'border-blue-500 text-gray-900 dark:text-white' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:border-gray-300'"
              >
                Service Discovery
              </button>
              <button 
                 @click="currentTab = 'rpc'"
                 class="inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium transition-colors duration-200"
                 :class="currentTab === 'rpc' ? 'border-blue-500 text-gray-900 dark:text-white' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:border-gray-300'"
              >
                RPC Console
              </button>
            </div>
          </div>
          <div class="flex items-center gap-4">
             <!-- Dark Mode Toggle -->
             <button @click="toggleDark" class="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                <span v-if="isDark" class="text-xl">🌙</span>
                <span v-else class="text-xl">☀️</span>
             </button>
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
