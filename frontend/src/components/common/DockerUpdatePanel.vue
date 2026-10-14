<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDockerUpdateStatus, startDockerUpdate, type DockerUpdateStatus } from '@/api/admin/system'

const props = defineProps<{ currentVersion: string; latestVersion: string; hasUpdate: boolean; manualCommand: string }>()
const emit = defineEmits<{ completed: [] }>()
const { t } = useI18n()
const state = ref<DockerUpdateStatus | null>(null)
const starting = ref(false)
const reconnecting = ref(false)
const error = ref('')
const loading = ref(true)
const active = computed(() => starting.value || !!state.value?.active)
const knownStages = new Set(['idle', 'queued', 'pulling', 'backing_up', 'recreating', 'verifying', 'completed', 'rolling_back', 'rolled_back', 'failed', 'interrupted'])
const stage = computed(() => reconnecting.value ? 'reconnecting' : knownStages.has(state.value?.stage || '') ? state.value!.stage : 'unavailable')
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false
let completedJob = ''

function isNewerVersion(version: string | undefined): boolean {
  if (!version || !/^\d+\.\d+\.\d+$/.test(version) || !/^\d+\.\d+\.\d+$/.test(props.currentVersion)) return false
  const target = version.split('.').map(Number)
  const current = props.currentVersion.split('.').map(Number)
  for (let index = 0; index < target.length; index++) {
    if (target[index] !== current[index]) return target[index] > current[index]
  }
  return false
}

function schedule() {
  clearTimeout(timer)
  if (!disposed) timer = setTimeout(() => { void refresh() }, 3000)
}

async function refresh() {
  try {
    const result = await getDockerUpdateStatus()
    if (disposed) return
    // A restarting app/host service may be temporarily unavailable. Keep the
    // known active job until its durable status can be read again.
    if (!result.available && state.value?.active) {
      reconnecting.value = true
      schedule()
      return
    }
    state.value = result
    reconnecting.value = false
    if (result.stage === 'completed' && result.job_id && completedJob !== result.job_id && isNewerVersion(result.version)) {
      completedJob = result.job_id
      emit('completed')
    }
    if (result.active) schedule()
  } catch {
    if (disposed) return
    reconnecting.value = true
    schedule()
  } finally {
    if (!disposed) loading.value = false
  }
}

async function start() {
  if (active.value || !state.value?.available) return
  starting.value = true
  error.value = ''
  try {
    const result = await startDockerUpdate()
    if (disposed) return
    state.value = result
    schedule()
  } catch {
    if (disposed) return
    // The POST may have been accepted before a network interruption. Read the
    // host's durable state instead of assuming it is safe to submit again.
    error.value = t('version.dockerJob.startUncertain')
    await refresh()
  } finally {
    if (!disposed) starting.value = false
  }
}

onMounted(() => { void refresh() })
onBeforeUnmount(() => { disposed = true; clearTimeout(timer) })
</script>

<template>
  <div class="space-y-3" aria-live="polite">
    <p v-if="loading" class="text-sm text-gray-500">{{ t('version.dockerJob.connecting') }}</p>
    <template v-else-if="state?.available || reconnecting">
      <div class="rounded-lg bg-blue-50 p-3 text-sm text-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
        <p>{{ t(`version.dockerJob.${stage}`) }}</p>
        <p v-if="state?.version" class="mt-1 font-mono text-xs">v{{ state.version }}</p>
      </div>
      <p v-if="hasUpdate && !active" class="text-xs text-gray-500">{{ t('version.dockerJob.hint') }}</p>
      <p v-if="error" class="text-xs text-amber-600">{{ error }}</p>
      <button
        v-if="hasUpdate"
        data-testid="docker-update-start"
        :disabled="active || reconnecting || !state?.available"
        class="w-full rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-50"
        @click="start"
      >{{ active ? t('version.updating') : `${t('version.updateNow')} · v${latestVersion}` }}</button>
      <button class="w-full text-xs text-gray-500 hover:underline" @click="refresh">{{ t('version.dockerJob.refresh') }}</button>
    </template>
    <template v-else>
      <p class="rounded-lg bg-blue-50 p-3 text-xs text-blue-700 dark:bg-blue-900/20 dark:text-blue-200">{{ t('version.dockerJob.unavailable') }}</p>
      <pre v-if="hasUpdate" class="max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-lg bg-gray-900 p-3 text-xs text-gray-100">{{ manualCommand }}</pre>
    </template>
  </div>
</template>
