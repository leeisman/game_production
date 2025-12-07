<script setup lang="ts">
import { ref, watch } from 'vue';

const methodWithDefaults = {
  'ValidateToken': { "token": "your_jwt_token_here" },
  'GetBalance': { "user_id": 1001 },
  'AddBalance': { "user_id": 1001, "amount": 1000, "reason": "bonus" },
  'GetCurrentRound': { "user_id": 1001 },
  'GetState': { "user_id": 1001 },
  'TestBroadcast': { "game_code": "color_game", "round_id": "test_round_123" }
}

const props = defineProps<{ projectId: string }>()

const selectedMethod = ref('ValidateToken')
const payloadStr = ref(JSON.stringify(methodWithDefaults['ValidateToken'], null, 2))
const response = ref<any>(null)
const loading = ref(false)
const error = ref('')

watch(selectedMethod, (newVal) => {
  // Update default payload when method changes
  payloadStr.value = JSON.stringify(methodWithDefaults[newVal as keyof typeof methodWithDefaults], null, 2)
  response.value = null
  error.value = ''
})

async function execute() {
  console.log("Starting execute...", selectedMethod.value)
  loading.value = true
  error.value = ''
  response.value = null
  
  try {
    // Validate JSON
    let payload = {}
    try {
      payload = JSON.parse(payloadStr.value)
      console.log("Payload parsed:", payload)
    } catch (e) {
      throw new Error("Invalid JSON Payload")
    }

    console.log("Fetching /api/grpc_call...")
    const res = await fetch('/api/grpc_call', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({
        project: props.projectId,
        method: selectedMethod.value,
        payload: payload
      })
    })

    console.log("Fetch status:", res.status)
    const data = await res.json()
    console.log("Response data:", data)

    if (!res.ok) {
      throw new Error(data.error || 'Request failed')
    }
    response.value = data
  } catch (e: any) {
    console.error("Execute error:", e)
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-lg">
    <h2 class="text-xl font-bold mb-4 text-gray-800 dark:text-white border-b dark:border-gray-700 pb-2">Generic gRPC Caller</h2>
    
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <!-- Left Column: Input -->
      <div class="space-y-4">
        <div>
          <label class="block font-semibold mb-1 text-gray-900 dark:text-gray-200">Method</label>
          <select v-model="selectedMethod" class="w-full border dark:border-gray-600 p-2 rounded bg-gray-50 dark:bg-gray-700 dark:text-gray-100">
            <option v-for="(_, key) in methodWithDefaults" :key="key" :value="key">
              {{ key }}
            </option>
          </select>
        </div>

        <div>
           <label class="block font-semibold mb-1 text-gray-900 dark:text-gray-200">JSON Payload</label>
           <textarea 
             v-model="payloadStr" 
             rows="8" 
             class="w-full border dark:border-gray-600 p-2 rounded font-mono text-sm bg-gray-50 dark:bg-gray-700 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 outline-none"
           ></textarea>
        </div>

        <button 
          @click="execute" 
          class="w-full bg-blue-600 text-white py-2 rounded hover:bg-blue-700 transition"
          :class="{'opacity-50 cursor-not-allowed': loading}"
          :disabled="loading"
        >
          {{ loading ? 'Executing...' : 'Execute RPC' }}
        </button>
      </div>

      <!-- Right Column: Output -->
      <div class="bg-gray-100 dark:bg-gray-900 p-4 rounded border dark:border-gray-700 h-full min-h-[300px] overflow-auto">
        <label class="block font-semibold mb-2 text-gray-600 dark:text-gray-400">Response</label>
        
        <div v-if="error" class="text-red-600 dark:text-red-400 font-mono whitespace-pre-wrap">{{ error }}</div>
        
        <div v-if="response" class="text-gray-800 dark:text-gray-200 font-mono text-sm whitespace-pre-wrap">
{{ JSON.stringify(response, null, 2) }}
        </div>

        <div v-if="!error && !response && !loading" class="text-gray-400 italic text-center mt-20">
          Result will appear here...
        </div>
      </div>
    </div>
  </div>
</template>
