<script setup lang="ts">
import { ref } from 'vue'

const userId = ref(1001) // Default ID
const balance = ref<number | null>(null)
const addAmount = ref(100)
const loading = ref(false)
const error = ref('')
const msg = ref('')

async function checkBalance() {
  loading.value = true
  error.value = ''
  msg.value = ''
  try {
    const res = await fetch(`/api/wallet/balance/${userId.value}`)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error)
    balance.value = data.balance
    msg.value = 'Query Success'
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function addBalance() {
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(`/api/wallet/add`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({
        user_id: Number(userId.value),
        amount: Number(addAmount.value),
        reason: 'ops_manual_add'
      })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error)
    balance.value = data.new_balance
    msg.value = `Added ${addAmount.value}. New Balance: ${data.new_balance}`
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="p-4 border rounded-lg bg-white shadow">
    <h2 class="text-xl font-bold mb-4 text-gray-800">User / Wallet Ops</h2>
    
    <div class="flex gap-4 mb-4 items-center">
      <label class="font-semibold">User ID:</label>
      <input v-model="userId" type="number" class="border p-1 rounded" />
      <button @click="checkBalance" class="bg-blue-500 text-white px-4 py-1 rounded hover:bg-blue-600" :disabled="loading">
        Check Balance
      </button>
    </div>

    <div v-if="balance !== null" class="mb-4 text-2xl font-mono text-green-600">
      Balance: {{ balance }}
    </div>

    <div class="flex gap-4 items-center border-t pt-4">
      <label class="font-semibold">Add Amount:</label>
      <input v-model="addAmount" type="number" class="border p-1 rounded" />
      <button @click="addBalance" class="bg-green-500 text-white px-4 py-1 rounded hover:bg-green-600" :disabled="loading">
        Add Funds
      </button>
    </div>

    <div v-if="error" class="mt-4 p-2 bg-red-100 text-red-700 rounded">{{ error }}</div>
    <div v-if="msg" class="mt-4 p-2 bg-green-100 text-green-700 rounded">{{ msg }}</div>
  </div>
</template>
