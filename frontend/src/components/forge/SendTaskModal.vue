<script setup lang="ts">
import { ref } from "vue";
import { TaskGroupAPI, DeviceAPI } from "../../services/api";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const props = defineProps<{
  show: boolean;
  bindingPreview: { ip: string; commands: string }[];
  hasInvalidIP: boolean;
  invalidIPs: string[];
}>();

const emit = defineEmits<{
  close: [];
  success: [message: string, count: number];
  toast: [message: string];
}>();

const saving = ref(false);
const name = ref("");
const description = ref("从 ConfigForge (IP绑定) 生成的任务");
const tags = ref(["ConfigForge", "IPBinding"]);
const newTag = ref("");

// 重置表单
const resetForm = () => {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  const h = String(now.getHours()).padStart(2, "0");
  const mi = String(now.getMinutes()).padStart(2, "0");
  const s = String(now.getSeconds()).padStart(2, "0");
  name.value = `ConfigForge_${y}${m}${d}_${h}${mi}${s}`;
  description.value = "从 ConfigForge (IP绑定) 生成的任务";
  tags.value = ["ConfigForge", "IPBinding"];
  newTag.value = "";
};

// 添加标签
const addTag = () => {
  const tag = newTag.value.trim();
  if (tag && !tags.value.includes(tag)) {
    tags.value.push(tag);
  }
  newTag.value = "";
};

// 删除标签
const removeTag = (index: number) => {
  tags.value.splice(index, 1);
};

// 执行发送
const executeSend = async () => {
  if (saving.value) return;
  if (!name.value.trim()) {
    emit("toast", "请输入任务名称");
    return;
  }
  if (props.bindingPreview.length === 0) {
    emit("toast", "没有可发送的绑定配置");
    return;
  }

  saving.value = true;
  try {
    // 获取设备映射
    const devices = await DeviceAPI.listDevices();
    const deviceMap = new Map(devices.map((d: any) => [d.ip, d.id]));

    const items = props.bindingPreview.map(
      (b: { ip: string; commands: string }) => {
        const deviceID = deviceMap.get(b.ip);
        if (!deviceID) {
          throw new Error(`设备 ${b.ip} 不存在于设备列表中`);
        }
        return {
          commandGroupId: "",
          commands: b.commands
            .split("\n")
            .map((l: string) => l.trim())
            .filter((l: string) => l !== ""),
          deviceIDs: [deviceID as number],
        };
      },
    );

    const taskGroup = {
      id: 0,
      name: name.value.trim(),
      description: description.value.trim(),
      deviceGroup: "",
      commandGroup: "",
      maxWorkers: 10,
      timeout: 60,
      taskType: "normal",
      topologyVendor: "",
      topologyFieldOverrides: [],
      autoBuildTopology: false,
      mode: "binding" as const,
      items,
      tags: tags.value,
      enableRawLog: false,
      backupSaveRootPath: "",
      backupDirNamePattern: "",
      backupFileNamePattern: "",
      backupStartupCommand: "",
      backupSftpTimeoutSec: 0,
      status: "",
      createdAt: "",
      updatedAt: "",
    };

    await TaskGroupAPI.createTaskGroup(taskGroup);
    emit(
      "success",
      `任务「${name.value.trim()}」已发送到任务执行`,
      props.bindingPreview.length,
    );
    emit("close");
  } catch (err: any) {
    logger.error("发送到任务执行失败", 'SendTaskModal', err);
    emit("toast", "发送失败: " + (err.message || err));
  } finally {
    saving.value = false;
  }
};

// 暴露方法供父组件调用
defineExpose({ resetForm });
</script>

<template>
  <Transition name="modal">
    <div v-if="show" class="modal-container modal-active">
      <div class="modal-overlay" @click="$emit('close')"></div>
      <div class="modal modal-lg modal-glass">
        <div class="modal-header">
          <h3 class="modal-header-title">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-warning"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <polygon
                points="5 3 19 12 5 21 5 3"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              />
            </svg>
            发送到任务执行
          </h3>
          <button @click="$emit('close')" class="modal-close">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
        <div class="modal-body space-y-5">
          <!-- 无效 IP 警告 -->
          <div
            v-if="hasInvalidIP"
            class="flex items-start gap-3 p-4 rounded-xl bg-error/10 border border-error/30"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-error mt-0.5 shrink-0"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path
                fill-rule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
                clip-rule="evenodd"
              />
            </svg>
            <div class="flex-1">
              <p class="text-sm font-medium text-error">检测到无效 IP 地址</p>
              <p class="text-xs text-text-muted mt-1">
                以下 IP 格式无效，将被过滤：{{
                  invalidIPs.slice(0, 3).join(", ")
                }}{{
                  invalidIPs.length > 3 ? ` 等 ${invalidIPs.length} 个` : ""
                }}
              </p>
            </div>
          </div>
          <!-- 名称 -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-text-primary"
              >任务名称 <span class="text-error">*</span></label
            >
            <input
              v-model="name"
              type="text"
              class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
            />
          </div>
          <!-- 描述 -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-text-primary">描述</label>
            <input
              v-model="description"
              type="text"
              class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
            />
          </div>
          <!-- 标签 -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium text-text-primary">标签</label>
            <div class="flex flex-wrap gap-2 mb-2">
              <span
                v-for="(tag, idx) in tags"
                :key="idx"
                class="inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded-full bg-accent/10 text-accent border border-accent/20"
              >
                {{ tag }}
                <button
                  @click="removeTag(idx)"
                  class="hover:text-error transition-colors"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="w-3 h-3"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="3"
                  >
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
              </span>
            </div>
            <div class="flex gap-2">
              <input
                v-model="newTag"
                type="text"
                class="flex-1 px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
                placeholder="添加标签"
                @keyup.enter="addTag"
              />
              <button
                @click="addTag"
                class="px-3 py-2 rounded-lg bg-accent/10 border border-accent/30 text-accent text-sm font-medium hover:bg-accent/20 transition-colors"
              >
                添加
              </button>
            </div>
          </div>
          <!-- 绑定预览 -->
          <div class="preview-box">
            <div class="preview-icon">🔗</div>
            <div class="preview-content">
              <span class="preview-text"
                >共
                <strong class="text-warning">{{
                  bindingPreview.length
                }}</strong>
                台设备的 IP 绑定任务</span
              >
              <div
                class="mt-2 space-y-1 max-h-32 overflow-auto scrollbar-custom"
              >
                <div
                  v-for="(b, i) in bindingPreview.slice(0, 5)"
                  :key="i"
                  class="flex items-center gap-2 text-xs"
                >
                  <span class="font-mono text-warning">{{ b.ip }}</span>
                  <span class="text-text-muted">→</span>
                  <span class="text-text-secondary truncate"
                    >{{ b.commands.split("\n").length }} 行命令</span
                  >
                </div>
                <div
                  v-if="bindingPreview.length > 5"
                  class="text-xs text-text-muted"
                >
                  +{{ bindingPreview.length - 5 }} 台设备...
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button @click="$emit('close')" class="btn btn-secondary">
            取消
          </button>
          <button
            @click="executeSend"
            :disabled="saving"
            class="btn btn-primary"
          >
            {{ saving ? "发送中..." : "确认发送" }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 预览信息框 */
.preview-box {
  @apply flex items-start gap-3 p-4 rounded-xl bg-info/10 border border-info/20;
}
.preview-icon {
  @apply text-lg;
}
.preview-content {
  @apply flex-1;
}
.preview-text {
  @apply text-sm text-text-secondary;
}
</style>
