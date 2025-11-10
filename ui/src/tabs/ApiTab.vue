<script setup>
import { ref, computed } from 'vue'
import {storeToRefs} from "pinia"
import {ChevronDoubleUpIcon, ArchiveBoxArrowDownIcon, ArchiveBoxXMarkIcon} from "@heroicons/vue/24/outline/index.js";

import { runAction, runActionWithoutDialog } from "../action.js";

import {useServicesStore} from "../store/services.js"
import {useNodeStore} from "../store/node.js";

const services = useServicesStore()

const { getServiceByName } = storeToRefs(services)
const { getNodeInfo } = useNodeStore()

const nodeOS = computed(() => {
  return getNodeInfo("os")
})

const gameapActive = computed(() => {
  return getServiceByName.value("gameap").status === "active"
})

const gameapAvailable = computed(() => {
  return getServiceByName.value("gameap").status === "active" ||
      getServiceByName.value("gameap").status === "inactive"
})

const defaultPath = computed(() => {
  return nodeOS.value === "windows"
      ? "C:\\gameap\\web"
      : "/var/www/gameap"
})

const servicePanelsCols = computed(() => {
  return window.innerWidth < 1000 ? 1 : 3
})

const showInstallationAskModal = ref(false)
const showUninstallationAskModal = ref(false)

const installationFormRef = ref(null)
const installationForm = ref({
  version: "v4",
  path: defaultPath,
  host: "127.0.0.1",
  port: 80,
  source: "repo",
  branch: "master",
  webServer: "nginx",
  database: "mysql",
  withDaemon: false,
})
const uninstallationFormRef = ref({})
const uninstallationForm = ref({
  withDaemon: false,
  withData: false,
  withServices: false,
})

const githubBranchOptions = [
  {label: "develop", value: "develop"},
  {label: "master", value: "master"},
]
const webServerOptions = [
  {label: "Nginx", value: "nginx"},
  {label: "None", value: "none"},
]
const databaseOptions = [
  {label: "PostgreSQL", value: "postgres"},
  {label: "MySQL", value: "mysql"},
  {label: "SQLite", value: "sqlite"},
  {label: "None", value: "none"},
]

const installationFormRules = {

}

const uninstallationFormRules = {

}

function onClickGameAPInstallationButton() {
  showInstallationAskModal.value = true
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

  let params = ""

  if (installationForm.value.version === "v4") {
    params = "--version=v4" +
        " --host=" + installationForm.value.host +
        " --port=" + installationForm.value.port +
        " --database=" + installationForm.value.database
  }

  if (installationForm.value.version === "v3") {
    params = "--version=v3" +
        " --path=" + installationForm.value.path +
        " --host=" + installationForm.value.host +
        " --port=" + installationForm.value.port +
        " --web-server=" + installationForm.value.webServer +
        " --database=" + installationForm.value.database

    if (installationForm.value.source === "github") {
      params += " --github"
      params += " --branch=" + installationForm.value.branch
    }
  }

  if (installationForm.value.withDaemon) {
    params += " --with-daemon"
  }

  runActionWithoutDialog(
      "gameap-install",
      "gameap-install " + params,
  )
}

function handleChangeVersionTab(tabName) {
  installationForm.value.version = tabName
}

function handleUninstallButtonClick() {
  showUninstallationAskModal.value = false

  let params = ""

  if (uninstallationForm.value.withDaemon) {
    params += " --with-daemon"
  }

  if (uninstallationForm.value.withData) {
    params += " --with-data"
  }

  if (uninstallationForm.value.withServices) {
    params += " --with-services"
  }

  runActionWithoutDialog(
      "gameap-uninstall",
      "gameap-uninstall " + params,
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
      <n-tabs :bar-width="28" size="large" type="line" justify-content="space-evenly" @update:value="handleChangeVersionTab">
        <n-tab-pane name="v4" tab="version 4 (latest)">
          <p class="mb-5">GameAP v4 is the latest stable release series. It is recommended for most users.
            It written in Go and Vue.js and provides better performance and stability.</p>

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
                <n-input v-model:value="installationForm.host" placeholder="Host" />
                <n-input-number v-model:value="installationForm.port" placeholder="Port" />
              </n-input-group>
            </n-form-item>
            <n-form-item label="Database" path="database">
              <n-select v-model:value="installationForm.database" :options="databaseOptions" />
            </n-form-item>
            <n-form-item label="&nbsp;" path="withDaemon">
              <n-checkbox v-model:checked="installationForm.withDaemon">
                Install Daemon
              </n-checkbox>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <n-tab-pane name="v3" tab="version 3">
          <p class="mb-5">GameAP v3 is the legacy release series. It is written in PHP and Vue.js.</p>

          <n-form
              ref="formRef"
              :model="installationFormRef"
              size="medium"
              label-placement="left"
              label-width="auto"
              :rules="installationFormRules"
          >
            <n-form-item label="Path" path="path">
              <n-input
                  v-model:value="installationForm.path"
                  :disabled="nodeOS === 'windows'"
                  placeholder="Path"
              />
            </n-form-item>
            <n-form-item label="Source" path="source">
              <n-radio-group v-model:value="installationForm.source">
                <n-radio value="repo">
                  Official Release Repo
                </n-radio>
                <n-radio value="github">
                  GitHub
                </n-radio>
              </n-radio-group>
            </n-form-item>
            <n-form-item v-if="installationForm.source === 'github'" label="Branch" path="branch">
              <n-select v-model:value="installationForm.branch" :options="githubBranchOptions" />
            </n-form-item>
            <n-form-item label="Host" path="host">
              <n-input-group>
                <n-input v-model:value="installationForm.host" placeholder="Host" />
                <n-input-number v-model:value="installationForm.port" placeholder="Port" />
              </n-input-group>
            </n-form-item>
            <n-form-item label="Web Server" path="webServer">
              <n-select v-model:value="installationForm.webServer" :options="webServerOptions" />
            </n-form-item>
            <n-form-item label="Database" path="database">
              <n-select v-model:value="installationForm.database" :options="databaseOptions" />
            </n-form-item>
            <n-form-item label="&nbsp;" path="withDaemon">
              <n-checkbox v-model:checked="installationForm.withDaemon">
                Install Daemon
              </n-checkbox>
            </n-form-item>
          </n-form>
        </n-tab-pane>
      </n-tabs>

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