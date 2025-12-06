<script setup lang="ts">
import { ref } from 'vue'

const roundInfo = ref<any>(null)
const loading = ref(false)
const error = ref('')

async function getRound() {
  loading.value = true
  error.value = ''
  roundInfo.value = null
  try {
    const res = await fetch(`/api/color_game/round`)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || 'Failed')
    roundInfo.value = data
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="p-4 border rounded-lg bg-white shadow mt-4">
    <h2 class="text-xl font-bold mb-4 text-gray-800">Color Game Ops</h2>
    
    <button @click="getRound" class="bg-purple-500 text-white px-4 py-2 rounded hover:bg-purple-600 mb-4" :disabled="loading">
      Get Current Round
    </button>

    <div v-if="roundInfo" class="bg-gray-100 p-4 rounded overflow-auto text-sm font-mono">
      <pre>{{ JSON.stringify(roundInfo, null, 2) }}</pre>
    </div>

    <div v-if="error" class="mt-4 p-2 bg-red-100 text-red-700 rounded">{{ error }}</div>
  </div>
</template>
