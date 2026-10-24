import { reactive } from "vue";

export interface Toast { id: number; text: string; kind: "ok" | "error" }

export const toasts = reactive<Toast[]>([]);
let next = 1;

export function toast(text: string, kind: Toast["kind"] = "ok") {
  const id = next++;
  toasts.push({ id, text, kind });
  setTimeout(() => {
    const i = toasts.findIndex((t) => t.id === id);
    if (i >= 0) toasts.splice(i, 1);
  }, kind === "error" ? 6000 : 2500);
}
