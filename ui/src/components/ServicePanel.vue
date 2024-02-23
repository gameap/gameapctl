<script setup>
import {computed, defineProps} from 'vue'
import ServiceButtons from "./ServiceButtons.vue"
import {useServicesStore} from "../store/services.js"
import {storeToRefs} from "pinia"

const props = defineProps({
  name: String,
  serviceId: String
})

const services = useServicesStore()

const { getServiceByName } = storeToRefs(services)

const service = computed(() => {
  return getServiceByName.value(props.serviceId)
})

function serviceStatus() {
  if (service.value.status === undefined || service.value.status === false || service.value.status === "") {
    return "unknown"
  }

  return service.value.status
}

</script>

<template>
  <n-card :title="name">
    <n-tag v-if="serviceStatus() === 'inactive'" :bordered="false" type="error">
      Stopped
    </n-tag>

    <n-tag v-if="serviceStatus() === 'active'" :bordered="false" type="success">
      Running
    </n-tag>

    <n-tag v-else :bordered="false">
      Unavailable / Not Found
    </n-tag>

    <div class="mt-3">
      <service-buttons :service-id="serviceId" />
    </div>

    <slot name="extra-buttons"></slot>
  </n-card>
</template>