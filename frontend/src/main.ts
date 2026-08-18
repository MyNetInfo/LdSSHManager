import {createApp} from 'vue'
import App from './App.vue'
import './style.css';
import {initI18n} from './i18n';
import {initTheme} from './theme';

initI18n();
initTheme();

createApp(App).mount('#app')
