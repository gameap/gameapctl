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
                <img class="h-8 w-auto" src="https://gameap.com/img/gameap_white.png" alt="GameAP" />
              </div>
              <div class="hidden sm:ml-6 sm:block">
                <div class="flex space-x-4">
                  <a v-for="item in navigation" :key="item.name" :href="item.href" :class="[item.current ? 'bg-gray-900 text-white' : 'text-gray-300 hover:bg-gray-700 hover:text-white', 'rounded-md px-3 py-2 text-sm font-medium']" :aria-current="item.current ? 'page' : undefined">{{ item.name }}</a>
                </div>
              </div>
            </div>
          </div>
        </div>

        <DisclosurePanel class="sm:hidden">
          <div class="space-y-1 px-2 pb-3 pt-2">
            <DisclosureButton v-for="item in navigation" :key="item.name" as="a" :href="item.href" :class="[item.current ? 'bg-gray-900 text-white' : 'text-gray-300 hover:bg-gray-700 hover:text-white', 'block rounded-md px-3 py-2 text-base font-medium']" :aria-current="item.current ? 'page' : undefined">{{ item.name }}</DisclosureButton>
          </div>
        </DisclosurePanel>
      </Disclosure>

      <Action />

      <ApiTab />
      <DaemonTab />

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
import { useServicesStore } from "./store/services.js";
import {unarySend} from "./websocket.js"

const servicesStore = useServicesStore()

onBeforeMount(() => {
  unarySend("service-daemon-status", "service-status gameap-daemon", (code, message) => {
    if (code === "payload") {
      servicesStore.updateService("gameap-daemon", {status: message})
    }
  })
  unarySend("service-nginx-status", "service-status nginx", (code, message) => {
    if (code === "payload") {
      servicesStore.updateService("nginx", {status: message})
    }
  })
  unarySend("service-mysql-status", "service-status mysql", (code, message) => {
    if (code === "payload") {
      servicesStore.updateService("mysql", {status: message})
    }
  })
  unarySend("service-php-status", "service-status php-fpm", (code, message) => {
    if (code === "payload") {
      servicesStore.updateService("php-fpm", {status: message})
    }
  })
})

const navigation = ref([
  { name: 'GameAP Control', href: '#', current: true },
])
</script>