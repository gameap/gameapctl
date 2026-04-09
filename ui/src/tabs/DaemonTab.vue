<script setup>

import ServicePanel from "../components/ServicePanel.vue";
import {ArchiveBoxArrowDownIcon, ChevronDoubleUpIcon, QuestionMarkCircleIcon} from "@heroicons/vue/24/outline/index.js";
import {useServicesStore} from "../store/services.js";
import {storeToRefs} from "pinia";
import {computed, ref} from "vue";
import {runAction, runActionWithoutDialog} from "../action.js";

const services = useServicesStore()

const { getServiceByName } = storeToRefs(services)

const daemonService = computed(() => {
  return getServiceByName.value("gameap-daemon")
})

function daemonAvailable() {
  return daemonService.value.status !== undefined &&
      daemonService.value.status !== "" &&
      daemonService.value.status !== null &&
      daemonService.value.status !== false &&
      (daemonService.value.status === 'active' || daemonService.value.status === 'inactive')
}

const showInstallationAskModal = ref(false)

const installationFormRef = ref(null)
const installationForm = ref({
  connectURL: "",
  host: "",
  installationToken: "",
})
const installationFormRules = {

}

const isConnectMode = computed(() => {
  return installationForm.value.connectURL !== ""
})

const isLegacyMode = computed(() => {
  return installationForm.value.host !== "" || installationForm.value.installationToken !== ""
})

function onClickDaemonInstallationButton() {
  showInstallationAskModal.value = true
}

function handleInstallButtonClick(e) {
  showInstallationAskModal.value = false

  let params
  if (installationForm.value.connectURL) {
    params = "--connect-url=" + installationForm.value.connectURL
  } else {
    params = "--host=" + installationForm.value.host +
        " --installation-token=" + installationForm.value.installationToken
  }

  runActionWithoutDialog(
      "daemon-install",
      "daemon-install " + params,
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

  <n-modal
      v-model:show="showInstallationAskModal"
      :mask-closable="true"
      >
    <n-card
        class="card"
        :bordered="false"
        title="GameAP Daemon Installation"
        size="huge"
        role="dialog"
        aria-modal="true"
        >
      <n-form
          ref="installationFormRef"
          :model="installationForm"
          :rules="installationFormRules"
          label-placement="left"
          label-width="auto"
          >
        <n-form-item label="Connect URL" prop="connectURL">
          <n-input
              v-model:value="installationForm.connectURL"
              placeholder="grpc://host:port/key"
              :disabled="isLegacyMode"
          />

          <n-tooltip placement="top-start" trigger="hover">
            <template #trigger>
              <QuestionMarkCircleIcon class="ml-1 h-5 w-5 text-gray-400" />
            </template>
            Paste the connect URL from the GameAP panel (grpc://host:port/key). This is the recommended way to connect.
          </n-tooltip>
        </n-form-item>

        <n-divider>or</n-divider>

        <n-form-item label="Host" prop="host">
          <n-input
              v-model:value="installationForm.host"
              placeholder="http://<your host / IP>"
              :disabled="isConnectMode"
          />

          <n-tooltip placement="top-start" trigger="hover">
            <template #trigger>
              <QuestionMarkCircleIcon class="ml-1 h-5 w-5 text-gray-400" />
            </template>
            Write the IP address or domain name of the server where the GameAP Daemon will be installed.
          </n-tooltip>

        </n-form-item>
        <n-form-item label="Installation token" prop="installationToken">
          <n-input
              v-model:value="installationForm.installationToken"
              placeholder="Token"
              :disabled="isConnectMode"
          />

          <n-tooltip placement="top-start" trigger="hover">
            <template #trigger>
              <QuestionMarkCircleIcon class="ml-1 h-5 w-5 text-gray-400" />
            </template>
            Open GameAP, go to Dedicated Servers and click "Create" button, then copy the token from the installation form.
          </n-tooltip>
        </n-form-item>
        <n-form-item>
          <n-button type="primary" @click="handleInstallButtonClick">
            Install
          </n-button>
        </n-form-item>
      </n-form>
    </n-card>
  </n-modal>
</template>

<style scoped>
.card {
  width: 600px;
}
.service-panels {
  text-align: center;
}
</style>