<script setup>
import { ref, computed } from 'vue'
import {storeToRefs} from "pinia"
import ServicePanel from "../components/ServicePanel.vue";
import {ChevronDoubleUpIcon, ArchiveBoxArrowDownIcon} from "@heroicons/vue/24/outline/index.js";

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

const defaultPath = computed(() => {
  return nodeOS.value === "windows"
      ? "C:\\gameap"
      : "/var/www/gameap"
})

const showInstallationAskModal = ref(false)

const installationFormRef = ref(null);
const installationForm = ref({
  path: defaultPath,
  host: "127.0.0.1",
  port: "80",
  source: "repo",
  branch: "master",
  webServer: "nginx",
  database: "mysql",
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
  {label: "MySQL", value: "mysql"},
  {label: "SQLite", value: "sqlite"},
  {label: "None", value: "none"},
]

const installationFormRules = {

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

function handleInstallButtonClick(e) {
  showInstallationAskModal.value = false

  let params = "--path=" + installationForm.value.path +
      " --host=" + installationForm.value.host +
      " --port=" + installationForm.value.port +
      " --web-server=" + installationForm.value.webServer +
      " --database=" + installationForm.value.database

  if (installationForm.value.source === "github") {
    params += " --source=github"
    params += " --branch=" + installationForm.value.branch
  }

  runActionWithoutDialog(
      "gameap-install",
      "gameap-install " + params,
  )
}

</script>

<template>
  <div class="mt-6">
    <n-grid x-gap="12" :y-gap="10" :cols="1">
      <n-gi>
        <n-card title="API/Web" class="service-panels">
          <n-button
              :disabled="gameapActive"
              @click="onClickGameAPInstallationButton()"
          >
            <template #icon>
              <ArchiveBoxArrowDownIcon />
            </template>
            Install
          </n-button>

          <n-button
              :disabled="!gameapActive"
              @click="onClickGameAPUpgradingButton()"
          >
            <template #icon>
              <ChevronDoubleUpIcon />
            </template>
            Upgrade
          </n-button>

        </n-card>
      </n-gi>
    </n-grid>
  </div>

  <div class="service-panels mt-6">
    <n-grid x-gap="12" :y-gap="10" :cols="3">
      <n-gi>
        <service-panel name="Nginx" service-id="nginx" />
      </n-gi>
      <n-gi>
        <service-panel name="PHP" service-id="php-fpm"/>
      </n-gi>
      <n-gi>
        <service-panel name="MySQL/MariaDB" service-id="mysql" />
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
          <n-input v-model:value="installationForm.host" placeholder="Host" />
        </n-form-item>
        <n-form-item label="Port" path="port">
          <n-input v-model:value="installationForm.port" placeholder="Port" />
        </n-form-item>
        <n-form-item label="Web Server" path="webServer">
          <n-select v-model:value="installationForm.webServer" :options="webServerOptions" />
        </n-form-item>
        <n-form-item label="Database" path="database">
          <n-select v-model:value="installationForm.database" :options="databaseOptions" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="handleInstallButtonClick">
        Install
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