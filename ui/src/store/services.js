import { defineStore } from 'pinia'

export const useServicesStore =
    defineStore('services', {
        state: () => ({
            services: new Map(),
        }),
        actions: {
            updateService(name, service) {
                this.services.set(name, service)
            },
        },
        getters: {
            getServiceByName: (state) => {
                return (name) => {
                    if (state.services === undefined || !state.services.has(name)) {
                        return {}
                    }

                    return state.services.get(name)
                }
            },
        },
    }
)