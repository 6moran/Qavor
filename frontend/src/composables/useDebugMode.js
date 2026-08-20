import { ref } from 'vue'

const debugMode = ref(false)

export function useDebugMode() {
  function toggleDebugMode() {
    debugMode.value = !debugMode.value
  }

  return {
    debugMode,
    toggleDebugMode
  }
}