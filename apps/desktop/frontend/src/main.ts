import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import './lib/theme' // applies data-theme before mount

createApp(App).use(createPinia()).mount('#app')
