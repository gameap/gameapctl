<script setup>
import {computed, defineProps} from 'vue'
import {ArrowPathIcon, PlayIcon, StopIcon} from "@heroicons/vue/24/outline/index.js"
import { storeToRefs } from "pinia"
import {useServicesStore} from "../store/services.js"

const props = defineProps({
  serviceId: String
})

const services = useServicesStore()

const { getServiceByName } = storeToRefs(services)

const service = computed(() => {
  return getServiceByName.value(props.serviceId)
})

function serviceActive() {
  return service.value.status === 'active'
}

function serviceInactive() {
  return service.value.status === 'inactive'
}

function serviceUnavailable() {
  return service.value.status === undefined || service.value.status === '' || service.value.status === false
}

</script>

<template>
  <div class="mt-3">
    <n-button-group>
      <n-button
          :disabled="serviceUnavailable() || serviceInactive()"
      >
        <template #icon>
          <PlayIcon />
        </template>
        Start
      </n-button>

      <n-button
          :disabled="serviceUnavailable() || serviceActive()"
      >
        <template #icon>
          <StopIcon />
        </template>
        Stop
      </n-button>

      <n-button
          :disabled="serviceUnavailable() || serviceActive()"
      >
        <template #icon>
          <ArrowPathIcon />
        </template>
        Restart
      </n-button>
    </n-button-group>
  </div>
</template>