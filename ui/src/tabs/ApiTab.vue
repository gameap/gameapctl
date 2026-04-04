<script setup>
import { ref, computed } from 'vue'
import {storeToRefs} from "pinia"
import {ChevronDoubleUpIcon, ArchiveBoxArrowDownIcon, ArchiveBoxXMarkIcon} from "@heroicons/vue/24/outline/index.js";

import { runAction, runActionWithoutDialog } from "../action.js";
import { unarySend } from "../websocket.js";

import {useServicesStore} from "../store/services.js"
const services = useServicesStore()

const { getServiceByName } = storeToRefs(services)

const gameapActive = computed(() => {
  return getServiceByName.value("gameap").status === "active"
})

const gameapAvailable = computed(() => {
  return getServiceByName.value("gameap").status === "active" ||
      getServiceByName.value("gameap").status === "inactive"
})


const servicePanelsCols = computed(() => {
  return window.innerWidth < 1000 ? 1 : 3
})

const hostOptions = ref([
  { label: "127.0.0.1", value: "127.0.0.1" }
])

const showInstallationAskModal = ref(false)
const showUninstallationAskModal = ref(false)

const installationFormRef = ref(null)
const installationForm = ref({
  host: "127.0.0.1",
  port: 80,
  database: "postgres",
  withDaemon: false,
})
const uninstallationFormRef = ref({})
const uninstallationForm = ref({
  withDaemon: false,
  withData: false,
  withServices: false,
})

const databaseOptionsV4 = [
  {label: "PostgreSQL", value: "postgres"},
  {label: "MySQL", value: "mysql"},
  {label: "SQLite", value: "sqlite"},
  {label: "None", value: "none"},
]

const installationFormRules = {

}

const uninstallationFormRules = {

}

function loadHostIPs() {
  unarySend("detect-ips", "detect-ips", (code, value) => {
    if (code === "payload" && value) {
      hostOptions.value = value.split(",").map(ip => ({ label: ip, value: ip }))
    }
  })
}

function onClickGameAPInstallationButton() {
  showInstallationAskModal.value = true
  loadHostIPs()
}

function onClickGameAPUpgradingButton() {
  runAction(
      "GameAP Upgrading",
      "Are you sure?",
      "gameap-upgrade",
      "gameap-upgrade",
  )
}

function onClickGameAPUninstallationButton() {
  showUninstallationAskModal.value = true
}

function handleInstallButtonClick(e) {
  showInstallationAskModal.value = false

  let params = "--version=v4" +
      " --host=" + installationForm.value.host +
      " --port=" + installationForm.value.port +
      " --database=" + installationForm.value.database

  if (installationForm.value.withDaemon) {
    params += " --with-daemon"
  }

  runActionWithoutDialog(
      "gameap-install",
      "gameap-install " + params,
  )
}


function handleUninstallButtonClick() {
  showUninstallationAskModal.value = false

  let params = []

  if (uninstallationForm.value.withDaemon) {
    params.push("--with-daemon=true")
  }

  if (uninstallationForm.value.withData) {
    params.push("--with-data=true")
  }

  if (uninstallationForm.value.withServices) {
    params.push("--with-services=true")
  }

  runActionWithoutDialog(
      "gameap-uninstall",
      "gameap-uninstall " + params.join(" "),
  )
}

</script>

<template>
  <div class="mt-6">
    <n-grid x-gap="12" :y-gap="10" :cols="1">
      <n-gi>
        <n-card title="API/Web" class="service-panels">

          <n-button-group>
            <n-button
                :disabled="gameapAvailable"
                @click="onClickGameAPInstallationButton()"
            >
              <template #icon>
                <ArchiveBoxArrowDownIcon />
              </template>
              Install
            </n-button>

            <n-button
                :disabled="!gameapAvailable"
                @click="onClickGameAPUpgradingButton()"
            >
              <template #icon>
                <ChevronDoubleUpIcon />
              </template>
              Upgrade
            </n-button>

            <n-button
                :disabled="!gameapAvailable"
                type="error"
                @click="onClickGameAPUninstallationButton()"
                >
              <template #icon>
                <ArchiveBoxXMarkIcon />
              </template>
              Remove
            </n-button>
          </n-button-group>

        </n-card>
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
        title="GameAP Installation"
        size="small"
        role="dialog"
        aria-modal="true"
    >
      <n-form
          ref="formRef"
          :model="installationFormRef"
          size="medium"
          label-placement="left"
          label-width="auto"
          :rules="installationFormRules"
      >
        <n-form-item label="Host" path="host">
          <n-input-group>
            <n-select
                v-model:value="installationForm.host"
                :options="hostOptions"
                filterable
                tag
                placeholder="Select or enter IP"
            />
            <n-input-number v-model:value="installationForm.port" placeholder="Port" />
          </n-input-group>
        </n-form-item>
        <n-form-item label="Database" path="database">
          <n-select v-model:value="installationForm.database" :options="databaseOptionsV4" />
        </n-form-item>
        <n-form-item label="&nbsp;" path="withDaemon">
          <n-checkbox v-model:checked="installationForm.withDaemon">
            Install Daemon
          </n-checkbox>
        </n-form-item>
      </n-form>

      <n-button type="primary" @click="handleInstallButtonClick">
        Install
      </n-button>
    </n-card>
  </n-modal>

  <n-modal
      v-model:show="showUninstallationAskModal"
      :mask-closable="true"
  >
    <n-card
        class="card"
        :bordered="false"
        title="GameAP Uninstallation"
        size="small"
        role="dialog"
        aria-modal="true"
    >
      <n-form
          ref="formRef"
          :model="uninstallationFormRef"
          size="medium"
          label-placement="left"
          label-width="auto"
          :rules="uninstallationFormRules"
      >
        <n-form-item label="With daemon (removes daemon service)">
          <n-input-group>
            <n-switch v-model:value="uninstallationForm.withDaemon" />
          </n-input-group>
        </n-form-item>

        <n-form-item label="With data (removes database and files)">
          <n-input-group>
            <n-switch v-model:value="uninstallationForm.withData"/>
          </n-input-group>
        </n-form-item>

        <n-form-item label="With services (removes nginx, php, etc.)">
          <n-input-group>
            <n-switch v-model:value="uninstallationForm.withServices">
              Remove Services (nginx, php, etc.)
            </n-switch>
          </n-input-group>
        </n-form-item>

      </n-form>

      <n-button type="primary" @click="handleUninstallButtonClick">
        Uninstall
      </n-button>
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