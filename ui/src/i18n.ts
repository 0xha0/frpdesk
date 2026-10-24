// a small i18n: the source strings are English keys; zh has the translations. The choice is kept
// in localStorage (frpdesk.lang); the first visit follows the browser.
import { ref } from "vue";

export type Lang = "en" | "zh";

const zh: Record<string, string> = {
  "Sign in": "登录",
  "Username": "用户名",
  "Password": "密码",
  "Signing in…": "登录中…",
  "Wrong username or password": "用户名或密码错误",
  "Sign out": "退出",
  "Users": "用户",
  "Proxies": "隧道",
  "Server": "服务器",
  "Search users": "搜索用户",
  "New user": "新建用户",
  "User": "用户",
  "Token": "令牌",
  "Ports": "端口",
  "Domains": "域名",
  "Subdomains": "子域名",
  "Notes": "备注",
  "Status": "状态",
  "Enabled": "启用",
  "Disabled": "停用",
  "Enable": "启用",
  "Disable": "停用",
  "Edit": "编辑",
  "Remove": "删除",
  "Export frpc.toml": "导出 frpc.toml",
  "Copy": "复制",
  "Copied": "已复制",
  "Generate": "生成",
  "Save": "保存",
  "Cancel": "取消",
  "Edit user": "编辑用户",
  "No users yet. frps lets no client in until there is one.": "还没有用户。没有用户时 frps 不放行任何客户端。",
  "No users match.": "没有匹配的用户。",
  "Any port": "任意端口",
  "Any": "不限",
  "Comma-separated: 30000, 30100-30199": "逗号分隔：30000, 30100-30199",
  "Comma-separated domains": "逗号分隔的域名",
  "Comma-separated subdomains": "逗号分隔的子域名",
  "Letters, digits and _": "字母、数字和下划线",
  "Remove user {name}?": "删除用户 {name}？",
  "Its clients lose their login at once; the ports go free.": "它的客户端会立刻掉线，端口释放。",
  "User saved": "已保存",
  "User removed": "已删除",
  "Type": "类型",
  "Name": "名称",
  "Remote": "公网侧",
  "Client": "客户端",
  "Today in / out": "今日入 / 出",
  "Connections": "连接数",
  "Last start": "最近启动",
  "online": "在线",
  "offline": "离线",
  "No proxies of this type.": "没有这种类型的隧道。",
  "Refresh": "刷新",
  "Version": "版本",
  "Bind port": "连接端口",
  "KCP port": "KCP 端口",
  "QUIC port": "QUIC 端口",
  "HTTP port": "HTTP 端口",
  "HTTPS port": "HTTPS 端口",
  "TCPMUX port": "TCPMUX 端口",
  "Subdomain host": "子域名根",
  "Max pool count": "最大连接池",
  "Max ports per client": "每客户端最大端口数",
  "Heartbeat timeout": "心跳超时",
  "Allowed ports": "允许的端口",
  "TLS only": "强制 TLS",
  "Total traffic in": "总入站流量",
  "Total traffic out": "总出站流量",
  "Current connections": "当前连接",
  "Clients": "客户端数",
  "Proxies by type": "各类型隧道数",
  "frps dashboard unreachable": "连不上 frps 的面板",
  "Clients connect to": "客户端连接地址",
  "Loading…": "加载中…",
  "Failed": "失败",
  "Signed out": "已退出",
  "yes": "是",
  "no": "否",
};

function initial(): Lang {
  try {
    const saved = localStorage.getItem("frpdesk.lang");
    if (saved === "en" || saved === "zh") return saved;
  } catch {}
  return navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

export const lang = ref<Lang>(initial());

export function setLang(l: Lang) {
  lang.value = l;
  document.documentElement.lang = l;
  try { localStorage.setItem("frpdesk.lang", l); } catch {}
}

export function t(key: string, vars?: Record<string, string | number>): string {
  let s = lang.value === "zh" ? zh[key] ?? key : key;
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.replace(`{${k}}`, String(v));
  return s;
}
