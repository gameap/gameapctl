<script setup>
import { ref } from 'vue'

import ServicePanel from "../components/ServicePanel.vue";
import {ChevronDoubleUpIcon, ArchiveBoxArrowDownIcon} from "@heroicons/vue/24/outline/index.js";

import { runAction, runActionWithoutDialog } from "../action.js";

const showInstallationAskModal = ref(false)

const installationFormRef = ref(null);
const installationForm = ref({
  path: "/var/www/gameap",
  host: "127.0.0.1",
  port: "80",
  source: "repo",
  branch: "master",
})
const githubBranchOptions = [
  {label: "develop", value: "develop"},
  {label: "master", value: "master"},
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
  runActionWithoutDialog(
      "gameap-install",
      "gameap-install some-values=123,somevalue=3",
  )
}

</script>

<template>
  <div class="mt-6">
    <n-grid x-gap="12" :y-gap="10" :cols="1">
      <n-gi>
        <n-card title="API/Web" class="service-panels">
          <n-button @click="onClickGameAPInstallationButton()">
            <template #icon>
              <ArchiveBoxArrowDownIcon />
            </template>
            Install
          </n-button>

          <n-button @click="onClickGameAPUpgradingButton()">
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
          <n-input v-model:value="installationForm.path" placeholder="Path" />
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