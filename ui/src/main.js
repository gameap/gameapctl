import { createApp } from 'vue'
import { createPinia } from 'pinia'

import naive from "naive-ui";

import './style.css'

import App from './App.vue'

const app = createApp(App)
const pinia = createPinia()

app.use(naive)
app.use(pinia)

const meta = document.createElement('meta')
meta.name = 'naive-ui-style'
document.head.appendChild(meta)

app.mount('#app')
