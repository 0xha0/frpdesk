import { createRouter, createWebHistory } from "vue-router";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", name: "login", component: () => import("./views/LoginView.vue") },
    { path: "/", name: "home", component: () => import("./views/DashboardView.vue"), children: [
      { path: "", redirect: { name: "server" } },
      { path: "server", name: "server", component: () => import("./components/ServerPanel.vue") },
      { path: "users", name: "users", component: () => import("./components/UsersPanel.vue") },
      { path: "proxies", name: "proxies", component: () => import("./components/ProxiesPanel.vue") },
    ] },
    { path: "/:any(.*)", redirect: "/" },
  ],
});

export default router;
