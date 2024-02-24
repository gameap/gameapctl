<script setup>
import { ref } from 'vue'
import { useMessage, useDialog } from "naive-ui";
import {XMarkIcon} from "@heroicons/vue/24/outline/index.js";
import {send, subscribe, unsubscribe} from "../websocket.js";
import {subscribeAction} from "../action.js";
import {reloadServices} from "../global.js";

const dialog = useDialog();

const dialogTitleRef = ref("Confirm");
const dialogContentRef = ref("Are you sure?");
const topicRef = ref(String);

const actionMessage = ref("")

subscribeAction((dialogTitle, dialogContent, topic, message) => {
  dialogTitleRef.value = dialogTitle
  dialogContentRef.value = dialogContent
  topicRef.value = topic
  actionMessage.value = message

  if (dialogTitleRef.value !== "" || dialogContentRef.value !== "") {
    showDialog()
  } else {
    showModal.value = true
    log.value = ""
    complete.value = false
    run()
  }
})

const log = ref("")
const complete = ref(false)
const showModal = ref(false)

function showDialog() {
  dialog.success({
    title: dialogTitleRef.value,
    content: dialogContentRef.value,
    positiveText: "Yes",
    negativeText: "No",
    onPositiveClick: () => {
      log.value = ""
      complete.value = false
      showModal.value = true
      run()
    },
    onNegativeClick: () => {
    }
  });
}

function run() {
  send(topicRef.value, actionMessage.value)
  subscribe(topicRef.value, (id, code, message) => {
    if (code === "payload") {
      log.value += message
    }

    if (code === "error") {
      unsubscribe(id)
      complete.value = true
      log.value += "\nError:\n" + message + "\n"
      reloadServices()
    }

    if (code === "end" && complete.value === false) {
      unsubscribe(id)
      log.value += "\n" + "Completed" + "\n"
      complete.value = true
      reloadServices()
    }
  })
}

</script>

<template>
  <n-modal
      :show="showModal"
      :mask-closable="false"
  >
    <n-card
        class="card"
        :bordered="false"
        :title="dialogTitleRef"
        size="huge"
        role="dialog"
        aria-modal="true"
    >
      <template #default>
        <div class="log mr-3">
          <n-code :trim="true" v-model:code="log"/>
        </div>
      </template>
      <template #footer>
        <n-button type="error" v-if="complete" @click="showModal=false">
          <template #icon>
            <XMarkIcon />
          </template>
          Close
        </n-button>
      </template>
    </n-card>
  </n-modal>

</template>

<style scoped>
.card {
  width: 800px;
  height: 500px;
}
.log {
  overflow-y: scroll;
  height: 300px;
  background-color: black;
  color: white;
  padding: 15px;
}
</style>