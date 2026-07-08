import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { initAppZoom } from './composables/useAppZoom';
import './styles.css';

initAppZoom();

createApp(App).use(router).mount('#app');
