import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import { lang } from "./i18n";
import "./style.css";

document.documentElement.lang = lang.value;
createApp(App).use(router).mount("#app");
