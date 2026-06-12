import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import './styles.css';
import { installDesktopZoomShortcuts } from './api/desktop';

installDesktopZoomShortcuts();

createApp(App).use(router).mount('#app');
