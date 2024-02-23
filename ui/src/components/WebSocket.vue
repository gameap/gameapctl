<script setup>
import { ref } from "vue"
import { subscribe, unsubscribe, send } from "../websocket";

var conn;

const log = ref("")
const msg = ref("")

function sendMessage() {
  send("test", msg.value)
  subscribe("test", (id, code, message) => {
    if (code === "payload") {
      log.value += message
    }

    if (code === "end") {
      unsubscribe(id)
    }
  })
}

</script>

<template>
  <n-card>
    <n-input
        v-model:value="log"
        type="textarea"
        :autosize="{
            minRows: 15,
            maxRows: 15
          }"
    />

    <n-space vertical>
      <n-input v-model:value="msg" type="text" placeholder="Basic Input" />
      <n-button @click="sendMessage">Send</n-button>
    </n-space>
  </n-card>
</template>