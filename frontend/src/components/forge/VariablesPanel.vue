<script setup lang="ts">
import type { VarInput } from "../../services/api";

const variables = defineModel<VarInput[]>("variables", { required: true });
const ipBindingEnabled = defineModel<boolean>("ipBindingEnabled", {
  default: false,
});

const emit = defineEmits<{
  addVariable: [];
  removeVariable: [index: number];
  expandSyntax: [variable: VarInput];
  showSyntaxHelp: [];
}>();
</script>

<template>
  <div
    class="min-h-[400px] md:h-full md:min-h-[700px] glass-panel border border-border flex flex-col overflow-hidden"
  >
    <div class="card-header">
      <div>
        <h2 class="card-header-title">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-info"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
            />
          </svg>
          变量映射
          <button
            @click="emit('showSyntaxHelp')"
            class="ml-2.5 text-text-muted hover:text-accent transition-colors cursor-help focus:outline-none"
            title="语法说明"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-[1.125rem] w-[1.125rem]"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </button>
        </h2>
      </div>
      <button @click="emit('addVariable')" class="btn btn-sm btn-secondary">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-3.5 w-3.5 text-accent"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 4v16m8-8H4"
          />
        </svg>
        添加变量
      </button>
    </div>
    <div class="flex-1 overflow-y-auto p-4 space-y-4">
      <div
        v-for="(v, index) in variables"
        :key="index"
        class="bg-bg-tertiary/40 border border-border backdrop-blur-sm flex flex-col rounded-xl shadow-sm hover:shadow-md transition-shadow group"
        :class="
          ipBindingEnabled && index === 0
            ? 'border-warning/40 ring-1 ring-warning/20'
            : ''
        "
      >
        <div
          class="flex items-center justify-between px-3 py-2 border-b border-border bg-bg-tertiary/40 rounded-t-xl"
        >
          <div class="relative flex items-center gap-2">
            <input
              :value="v.name"
              @input="
                (e) => {
                  const newVars = [...variables];
                  newVars[index] = {
                    name: (e.target as HTMLInputElement).value,
                    valueString: variables[index]!.valueString,
                  };
                  variables = newVars;
                }
              "
              class="input input-sm input-mono text-center tracking-wider w-[150px]"
              :class="
                ipBindingEnabled && index === 0 ? 'text-warning font-bold' : ''
              "
              :readonly="ipBindingEnabled && index === 0"
            />
            <!-- 第一个变量显示IP绑定开关 -->
            <div v-if="index === 0" class="flex items-center gap-1.5">
              <label class="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  v-model="ipBindingEnabled"
                  class="sr-only peer"
                />
                <div
                  class="w-8 h-4 bg-bg-secondary peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-text-inverse after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-text-muted after:rounded-full after:h-3 after:w-3 after:transition-all peer-checked:bg-warning peer-checked:after:bg-text-inverse"
                ></div>
              </label>
              <span
                v-if="ipBindingEnabled"
                class="text-xs px-1.5 py-0.5 rounded bg-warning/10 border border-warning/30 text-warning font-medium"
                >IP绑定</span
              >
            </div>
          </div>
          <button
            @click="emit('removeVariable', index)"
            class="text-text-muted hover:text-error hover:bg-error-bg p-1.5 rounded-md transition-all"
            title="删除变量"
            :disabled="ipBindingEnabled && index === 0"
            :class="
              ipBindingEnabled && index === 0
                ? 'opacity-30 cursor-not-allowed'
                : ''
            "
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
          </button>
        </div>
        <textarea
          :value="v.valueString"
          @input="
            (e) => {
              const newVars = [...variables];
              newVars[index] = {
                name: variables[index]!.name,
                valueString: (e.target as HTMLTextAreaElement).value,
              };
              variables = newVars;
            }
          "
          @blur="emit('expandSyntax', v)"
          class="input textarea h-16 input-mono rounded-b-xl rounded-t-none border-0 border-t border-border bg-bg-tertiary/30"
          :placeholder="
            ipBindingEnabled && index === 0
              ? '192.168.1.1, 192.168.1.2, ...'
              : index === 0
                ? '1, 2, 3...'
                : index === 1
                  ? '1-3'
                  : index === 2
                    ? 'vlan10-13'
                    : index === 3
                      ? '192.168.1.1-3'
                      : '...'
          "
          spellcheck="false"
        ></textarea>
      </div>
    </div>
  </div>
</template>
