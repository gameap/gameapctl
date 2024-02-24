import { defineStore } from 'pinia'

export const useNodeStore = defineStore('node', {
    state: () => ({
        node: {},
    }),
    actions: {
        updateNode(node) {
            this.node = node
        },
    },
    getters: {
        getNodeInfo: (state) => {
            return (key) => {
                return state.node[key]
            }
        },
    },
})