<script setup>
import {computed, defineProps} from 'vue'
import {ArrowPathIcon, PlayIcon, StopIcon} from "@heroicons/vue/24/outline/index.js"
import { storeToRefs } from "pinia"
import {useServicesStore} from "../store/services.js"
import {runAction} from "../action.js";

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
  return service.value.status === undefined ||
      service.value.status === '' ||
      service.value.status === false ||
      (service.value.status !== 'active' &&
      service.value.status !== 'inactive')
}

function onClickStart() {
  runAction(
      "Service start",
      "Are you sure?",
      "service-command",
      "service-command start " + props.serviceId,
  )
}

function onClickStop() {
  runAction(
      "Service stop",
      "Are you sure?",
      "service-command",
      "service-command stop " + props.serviceId,
  )
}

function onClickRestart() {
  runAction(
      "Service restart",
      "Are you sure?",
      "service-command",
      "service-command restart " + props.serviceId,
  )
}

</script>

<template>
  <div class="mt-3">
    <n-button-group>
      <n-button
          :disabled="serviceUnavailable() || serviceActive()"
          @click="onClickStart()"
      >
        <template #icon>
          <PlayIcon />
        </template>
        Start
      </n-button>

      <n-button
          :disabled="serviceUnavailable() || serviceInactive()"
          @click="onClickStop()"
      >
        <template #icon>
          <StopIcon />
        </template>
        Stop
      </n-button>

      <n-button
          :disabled="serviceUnavailable() || serviceInactive()"
          @click="onClickRestart()"
      >
        <template #icon>
          <ArrowPathIcon />
        </template>
        Restart
      </n-button>
    </n-button-group>
  </div>
</template>