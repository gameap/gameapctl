<script setup>

import ServicePanel from "../components/ServicePanel.vue";
import {ArchiveBoxArrowDownIcon, ChevronDoubleUpIcon} from "@heroicons/vue/24/outline/index.js";
import {useServicesStore} from "../store/services.js";
import {storeToRefs} from "pinia";
import {computed} from "vue";
import {runAction} from "../action.js";

const services = useServicesStore()

const { getServiceByName } = storeToRefs(services)

const daemonService = computed(() => {
  return getServiceByName.value("gameap-daemon")
})

function daemonAvailable() {
  return daemonService.value.status !== undefined &&
      daemonService.value.status !== "" &&
      daemonService.value.status !== null &&
      daemonService.value.status !== false
}

function onClickDaemonInstallationButton() {
  runAction(
      "GameAP Daemon Installation",
      "Are you sure?",
      "daemon-install",
      "daemon-install",
  )
}

function onClickDaemonUpgradingButton() {
  runAction(
      "GameAP Daemon Upgrading",
      "Are you sure?",
      "daemon-upgrade",
      "daemon-upgrade",
  )
}

</script>

<template>
  <div class="service-panels mt-6">
    <n-grid x-gap="12" :y-gap="10" :cols="1">
      <n-gi>
        <service-panel name="GameAP Daemon" service-id="gameap-daemon">
          <template #extra-buttons>
            <div class="mt-3">
              <n-button
                  :disabled="daemonAvailable()"
                  @click="onClickDaemonInstallationButton()"
              >
                <template #icon>
                  <ArchiveBoxArrowDownIcon />
                </template>
                Install
              </n-button>

              <n-button
                  :disabled="!daemonAvailable()"
                  @click="onClickDaemonUpgradingButton()"
              >
                <template #icon>
                  <ChevronDoubleUpIcon />
                </template>
                Upgrade
              </n-button>
            </div>
          </template>
        </service-panel>
      </n-gi>
    </n-grid>
  </div>
</template>

<style scoped>
.service-panels {
  text-align: center;
}
</style>