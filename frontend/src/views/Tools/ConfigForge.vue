<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";

// Composables
import { useColumnResize } from "@/composables/useColumnResize";
import { useConfigBuilder } from "@/composables/useConfigBuilder";
import { useIPBinding } from "@/composables/useIPBinding";

// Components
import TemplateEditor from "@/components/forge/TemplateEditor.vue";
import VariablesPanel from "@/components/forge/VariablesPanel.vue";
import OutputPreview from "@/components/forge/OutputPreview.vue";
import SendCommandModal from "@/components/forge/SendCommandModal.vue";
import SendTaskModal from "@/components/forge/SendTaskModal.vue";
import SyntaxHelpModal from "@/components/forge/SyntaxHelpModal.vue";
import UsageHelpModal from "@/components/forge/UsageHelpModal.vue";

import type { VarInput } from "../../services/api";

const router = useRouter();

// ==================== Composables ====================
const resize = useColumnResize();
const { windowWidth, leftColWidth, midColWidth, rightColWidth, startResize } =
  resize;

const {
  templateText,
  variables,
  isBuilding,
  isCopied,
  outputBlocks,
  addVariable,
  removeVariable,
  expandSyntaxSugar,
  copyAll,
} = useConfigBuilder();

const { ipBindingEnabled, invalidIPs, hasInvalidIP, bindingPreview } =
  useIPBinding(variables, templateText);

// ==================== Modal 状态 ====================
const showSyntaxHelp = ref(false);
const showUsageHelp = ref(false);
const showSendModal = ref(false);
const showTaskModal = ref(false);

const sendCommandModalRef = ref<InstanceType<typeof SendCommandModal> | null>(
  null,
);
const sendTaskModalRef = ref<InstanceType<typeof SendTaskModal> | null>(null);

// ==================== Toast 通知 ====================
const toastMessage = ref("");
const showToast = ref(false);
let toastTimer: ReturnType<typeof setTimeout> | null = null;

const triggerToast = (msg: string) => {
  toastMessage.value = msg;
  showToast.value = true;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    showToast.value = false;
  }, 3000);
};

// ==================== 结果提示 ====================
const sendResult = ref({
  show: false,
  success: true,
  message: "",
  createdCount: 0,
});

const showSendResult = (success: boolean, message: string, count: number) => {
  sendResult.value = {
    show: true,
    success,
    message,
    createdCount: count,
  };

  setTimeout(() => {
    sendResult.value.show = false;
  }, 5000);
};

// ==================== 事件处理 ====================
const handleAddVariable = () => {
  addVariable();
};

const handleRemoveVariable = (index: number) => {
  // 如果删除的是第一个变量且开启了IP绑定，关闭绑定模式
  if (index === 0 && ipBindingEnabled.value) {
    ipBindingEnabled.value = false;
  }
  removeVariable(index);
};

const handleExpandSyntax = (v: VarInput) => {
  expandSyntaxSugar(v);
};

const openSendModal = () => {
  sendCommandModalRef.value?.resetForm();
  showSendModal.value = true;
};

const closeSendModal = () => {
  showSendModal.value = false;
};

const handleSendSuccess = (message: string, count: number) => {
  showSendResult(true, message, count);
};

const openTaskModal = () => {
  sendTaskModalRef.value?.resetForm();
  showTaskModal.value = true;
};

const closeTaskModal = () => {
  showTaskModal.value = false;
};

const handleTaskSuccess = (message: string, count: number) => {
  showSendResult(true, message, count);
};

const goToCommands = () => {
  router.push("/commands");
};

const goToTaskExecution = () => {
  router.push("/task-execution");
};

// ==================== 下载功能 ====================
const downloadAll = async () => {
  if (outputBlocks.value.length === 0) return;
  const text = outputBlocks.value.join("\n\n");

  try {
    if ("showSaveFilePicker" in window) {
      const handle = await (window as any).showSaveFilePicker({
        suggestedName: "ConfigForge_All.txt",
        types: [
          {
            description: "Text Files",
            accept: { "text/plain": [".txt"] },
          },
        ],
      });
      const writable = await handle.createWritable();
      await writable.write(text);
      await writable.close();
      triggerToast("配置文件已成功保存到本地！");
    } else {
      const blob = new Blob([text], { type: "text/plain;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "ConfigForge_All.txt";
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      triggerToast("配置文件已开始下载！");
    }
  } catch (err: any) {
    if (err.name !== "AbortError") {
      console.error("Failed to save file:", err);
      triggerToast("保存文件失败，请检查浏览器授权。");
    }
  }
};

const downloadSplit = async () => {
  if (outputBlocks.value.length === 0) return;

  try {
    if ("showDirectoryPicker" in window) {
      const dirHandle = await (window as any).showDirectoryPicker({
        mode: "readwrite",
      });

      for (let i = 0; i < outputBlocks.value.length; i++) {
        const block = outputBlocks.value[i];
        const fileHandle = await dirHandle.getFileHandle(
          `ConfigForge_Block_${i + 1}.txt`,
          { create: true },
        );
        const writable = await fileHandle.createWritable();
        await writable.write(block);
        await writable.close();
      }
      triggerToast(
        `成功在规定目录下生成并保存 ${outputBlocks.value.length} 个配置文件！`,
      );
    } else {
      outputBlocks.value.forEach((block, idx) => {
        setTimeout(() => {
          const blob = new Blob([block], { type: "text/plain;charset=utf-8" });
          const url = URL.createObjectURL(blob);
          const link = document.createElement("a");
          link.href = url;
          link.download = `ConfigForge_Block_${idx + 1}.txt`;
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
          URL.revokeObjectURL(url);
        }, idx * 200);
      });
      triggerToast("分块配置文件已开始下载！");
    }
  } catch (err: any) {
    if (err.name !== "AbortError") {
      console.error("Failed to save files:", err);
      triggerToast("批量保存文件失败，请检查浏览器授权。");
    }
  }
};

// ==================== 复制功能 ====================
const handleCopyAll = async () => {
  await copyAll();
};
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    <!-- 全局 Toast 提示 -->
    <div
      class="toast-container toast-container-top-center"
      :class="showToast ? 'visible' : 'invisible'"
    >
      <div
        class="toast toast-success"
        :class="showToast ? 'toast-visible' : ''"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="toast-icon"
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            fill-rule="evenodd"
            d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
            clip-rule="evenodd"
          />
        </svg>
        <span class="toast-message font-medium">{{ toastMessage }}</span>
      </div>
    </div>

    <!-- Main Workspace -->
    <main
      :ref="(el) => (resize.workspaceRef.value = el as HTMLElement)"
      class="flex-1 flex overflow-y-auto md:overflow-hidden flex-col md:flex-row gap-3 z-10"
    >
      <!-- Column 1: 配置模版 -->
      <TemplateEditor
        v-model="templateText"
        class="w-full md:w-auto"
        :style="{
          flex: windowWidth < 768 ? 'none' : `${leftColWidth} 1 0%`,
        }"
      />

      <!-- Left Resizer -->
      <div
        class="hidden md:flex w-1.5 cursor-col-resize hover:bg-accent/50 rounded-full active:bg-accent/80 transition-all z-20 flex-shrink-0"
        @mousedown="startResize('left')"
      ></div>

      <!-- Column 2: Variables Mapping -->
      <VariablesPanel
        v-model:variables="variables"
        v-model:ipBindingEnabled="ipBindingEnabled"
        class="w-full md:w-auto"
        :style="{
          flex: windowWidth < 768 ? 'none' : `${midColWidth} 1 0%`,
        }"
        @add-variable="handleAddVariable"
        @remove-variable="handleRemoveVariable"
        @expand-syntax="handleExpandSyntax"
        @show-syntax-help="showSyntaxHelp = true"
      />

      <!-- Right Resizer -->
      <div
        class="hidden md:flex w-1.5 cursor-col-resize hover:bg-info/50 rounded-full active:bg-info/80 transition-all z-20 flex-shrink-0"
        @mousedown="startResize('right')"
      ></div>

      <!-- Column 3: Output Preview -->
      <OutputPreview
        :output-blocks="outputBlocks"
        :is-building="isBuilding"
        :ip-binding-enabled="ipBindingEnabled"
        :is-copied="isCopied"
        class="w-full md:w-auto"
        :style="{
          flex: windowWidth < 768 ? 'none' : `${rightColWidth} 1 0%`,
        }"
        @send-command="openSendModal"
        @send-task="openTaskModal"
        @download-all="downloadAll"
        @download-split="downloadSplit"
        @copy-all="handleCopyAll"
      />
    </main>

    <!-- 右下角悬浮帮助按钮 -->
    <button
      @click="showUsageHelp = true"
      class="fixed bottom-6 right-6 z-40 w-12 h-12 flex items-center justify-center rounded-full bg-gradient-to-r from-accent to-info text-white shadow-lg shadow-accent/30 hover:shadow-accent/50 hover:scale-110 transition-all duration-300 cursor-help focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2"
      title="使用简介"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-6 w-6"
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

    <!-- 发送到命令管理弹窗 -->
    <SendCommandModal
      ref="sendCommandModalRef"
      :show="showSendModal"
      :output-blocks="outputBlocks"
      @close="closeSendModal"
      @success="handleSendSuccess"
      @toast="triggerToast"
    />

    <!-- 发送到任务执行弹窗 -->
    <SendTaskModal
      ref="sendTaskModalRef"
      :show="showTaskModal"
      :binding-preview="bindingPreview"
      :has-invalid-i-p="hasInvalidIP"
      :invalid-i-ps="invalidIPs"
      @close="closeTaskModal"
      @success="handleTaskSuccess"
      @toast="triggerToast"
    />

    <!-- 创建成功提示 -->
    <Transition name="toast">
      <div v-if="sendResult.show" class="success-toast">
        <div class="toast-icon">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              fill-rule="evenodd"
              d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
              clip-rule="evenodd"
            />
          </svg>
        </div>
        <div class="toast-content">
          <div class="toast-title">{{ sendResult.message }}</div>
          <button
            v-if="sendResult.message.includes('任务执行')"
            @click="goToTaskExecution"
            class="toast-link"
          >
            查看任务执行 →
          </button>
          <button v-else @click="goToCommands" class="toast-link">
            查看命令管理 →
          </button>
        </div>
      </div>
    </Transition>

    <!-- 使用简介弹窗 -->
    <UsageHelpModal :show="showUsageHelp" @close="showUsageHelp = false" />

    <!-- 语法说明弹窗 -->
    <SyntaxHelpModal :show="showSyntaxHelp" @close="showSyntaxHelp = false" />
  </div>
</template>

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 成功提示 */
.success-toast {
  @apply fixed bottom-6 left-1/2 -translate-x-1/2 z-[100] 
         flex items-center gap-3 px-5 py-4 rounded-xl 
         bg-success/95 text-white shadow-lg shadow-success/20;
}
.success-toast .toast-icon {
  @apply flex items-center justify-center w-8 h-8 rounded-full bg-white/20;
}
.success-toast .toast-content {
  @apply flex flex-col;
}
.success-toast .toast-title {
  @apply text-sm font-medium;
}
.success-toast .toast-link {
  @apply text-xs text-white/80 hover:text-white underline underline-offset-2 mt-1 text-left;
}

/* Toast 动画 */
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translate(-50%, 20px);
}

.toast-leave-to {
  opacity: 0;
  transform: translate(-50%, -10px);
}
</style>
