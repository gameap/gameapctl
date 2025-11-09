<template>
  <n-dialog-provider>
    <n-modal-provider>
      <Disclosure as="nav" class="bg-gray-800" v-slot="{ open }">
        <div class="mx-auto max-w-7xl px-2 sm:px-6 lg:px-8">
          <div class="relative flex h-16 items-center justify-between">
            <div class="absolute inset-y-0 left-0 flex items-center sm:hidden">
              <!-- Mobile menu button-->
              <DisclosureButton class="relative inline-flex items-center justify-center rounded-md p-2 text-gray-400 hover:bg-gray-700 hover:text-white focus:outline-none focus:ring-2 focus:ring-inset focus:ring-white">
                <span class="absolute -inset-0.5" />
                <span class="sr-only">Open main menu</span>
                <Bars3Icon v-if="!open" class="block h-6 w-6" aria-hidden="true" />
                <XMarkIcon v-else class="block h-6 w-6" aria-hidden="true" />
              </DisclosureButton>
            </div>
            <div class="flex flex-1 items-center justify-center sm:items-stretch sm:justify-start">
              <div class="flex flex-shrink-0 items-center">
                <img class="h-8 w-auto" src="/gameap.svg" alt="GameAP" />
              </div>
              <div class="hidden sm:ml-6 sm:block">
                <div class="flex space-x-4">
                  <a v-for="item in navigation" :key="item.name" :href="item.href" :class="[item.current ? 'bg-gray-900 text-white' : 'text-gray-300 hover:bg-gray-700 hover:text-white', 'rounded-md px-3 py-2 text-sm font-medium']" :aria-current="item.current ? 'page' : undefined">{{ item.name }}</a>
                </div>
              </div>
            </div>
            <div class="hidden md:block">
              <div class="ml-4 flex items-center md:ml-6">
                <button @click="onClickExit()" class="text-red-500 hover:bg-red-600 hover:text-white px-3 py-2 rounded-md text-sm font-medium">
                  <XMarkIcon class="h-6 w-6" aria-hidden="true" />
                  Exit
                </button>
              </div>
            </div>
          </div>
        </div>

        <DisclosurePanel class="sm:hidden">
          <div class="space-y-1 px-2 pb-3 pt-2">
            <DisclosureButton v-for="item in navigation" :key="item.name" as="a" :href="item.href" :class="[item.current ? 'bg-gray-900 text-white' : 'text-gray-300 hover:bg-gray-700 hover:text-white', 'block rounded-md px-3 py-2 text-base font-medium']" :aria-current="item.current ? 'page' : undefined">{{ item.name }}</DisclosureButton>
            <DisclosureButton @click="onClickExit()" class="text-red-500 hover:bg-red-600 hover:text-white block rounded-md px-3 py-2 text-base font-medium">Exit</DisclosureButton>
          </div>
        </DisclosurePanel>



      </Disclosure>

      <Action />

      <div class="service-panels mt-6">
        <div class="grid lg:grid-cols-2 gap-y-10 lg:gap-x-12">
          <ApiTab />
          <DaemonTab />
        </div>
      </div>

      <div class="service-panels mt-6">
        <div class="grid lg:grid-cols-2 gap-y-10 lg:gap-x-12">
          <service-panel name="PostgreSQL" service-id="postgresql" />
          <service-panel name="MySQL/MariaDB" service-id="mysql" />
          <service-panel name="Nginx" service-id="nginx" />
          <service-panel name="PHP" service-id="php-fpm"/>
        </div>
      </div>

    </n-modal-provider>
  </n-dialog-provider>
</template>

<script setup>
import { ref, onBeforeMount } from 'vue'
import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/vue'
import { Bars3Icon, XMarkIcon } from '@heroicons/vue/24/outline'

import ApiTab from "./tabs/ApiTab.vue"
import DaemonTab from "./tabs/DaemonTab.vue"
import Action from "./components/Action.vue"
import {reloadServices} from "./global.js"
import {unarySend} from "./websocket.js"
import {useNodeStore} from "./store/node.js"
import {runAction} from "./action.js";
import ServicePanel from "./components/ServicePanel.vue";

const nodeStore = useNodeStore()

onBeforeMount(() => {
  unarySend("node-info", "node-info", (code, message) => {
    if (code === "payload") {
      const lines = message.split('\n');
      const nodeInfo = {};

      lines.forEach(line => {
        const [key, value] = line.split(':');
        nodeInfo[key.trim().toLowerCase()] = value.trim();
      });

      nodeStore.updateNode(nodeInfo)
    }
  })
  reloadServices()
})

function onClickExit() {
  runAction(
      "Exit",
      "Are you want to exit?",
      "exit",
      "exit",
  )
}

const navigation = ref([
  { name: 'GameAP Control', href: '#', current: true },
])
</script>