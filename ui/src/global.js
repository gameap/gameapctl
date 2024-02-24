import { useServicesStore } from "./store/services.js";
import {unarySend} from "./websocket.js"

function reloadServices() {
    const servicesStore = useServicesStore()

    unarySend("service-gameap-status", "service-status gameap", (code, message) => {
        if (code === "payload") {
            servicesStore.updateService("gameap", {status: message})
        }
    })
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
}

export { reloadServices }