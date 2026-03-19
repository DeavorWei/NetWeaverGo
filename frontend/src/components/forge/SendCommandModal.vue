<script setup lang="ts">
import { ref, computed } from "vue";
import { CommandGroupAPI } from "../../services/api";
import type { CommandGroup } from "../../services/api";

const props = defineProps<{
  show: boolean;
  outputBlocks: string[];
}>();

const emit = defineEmits<{
  close: [];
  success: [message: string, count: number, ids: string[]];
  toast: [message: string];
}>();

const saving = ref(false);
const mode = ref<"merge" | "split">("merge");
const form = ref({
  name: "",
  description: "从 ConfigForge 生成的配置",
  tags: ["ConfigForge"],
});
const newTag = ref("");

// 默认名称
const defaultGroupName = computed(() => {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  const hour = String(now.getHours()).padStart(2, "0");
  const minute = String(now.getMinutes()).padStart(2, "0");
  const second = String(now.getSeconds()).padStart(2, "0");
  return `ConfigForge_${year}${month}${day}_${hour}${minute}${second}`;
});

// 发送预览
const sendPreview = computed(() => {
  const count = props.outputBlocks.length;
  if (mode.value === "merge") {
    return {
      type: "merge",
      commandCount: count,
      message: `共 ${count} 条命令将被添加`,
      examples: [],
    };
  } else {
    const examples: string[] = [];
    const prefix = form.value.name || "ConfigForge_";
    for (let i = 0; i < Math.min(count, 3); i++) {
      examples.push(`${prefix}${String(i + 1).padStart(2, "0")}`);
    }
    return {
      type: "split",
      groupCount: count,
      message: `将创建 ${count} 个命令组`,
      examples,
    };
  }
});

// 重置表单
const resetForm = () => {
  mode.value = "merge";
  form.value = {
    name: defaultGroupName.value,
    description: "从 ConfigForge 生成的配置",
    tags: ["ConfigForge"],
  };
  newTag.value = "";
};

// 添加标签
const addTag = () => {
  const tag = newTag.value.trim();
  if (tag && !form.value.tags.includes(tag)) {
    form.value.tags.push(tag);
  }
  newTag.value = "";
};

// 删除标签
const removeTag = (index: number) => {
  form.value.tags.splice(index, 1);
};

// 执行发送
const executeSend = async () => {
  if (saving.value) return;

  const { name, description, tags } = form.value;
  if (!name.trim()) {
    emit("toast", "请输入命令组名称");
    return;
  }

  if (props.outputBlocks.length === 0) {
    emit("toast", "没有可发送的配置");
    return;
  }

  saving.value = true;

  try {
    if (mode.value === "merge") {
      const allLines: string[] = [];
      props.outputBlocks.forEach((block: string) => {
        if (block) {
          const lines = block
            .split("\n")
            .map((l: string) => l.trim())
            .filter((l: string) => l !== "");
          allLines.push(...lines);
        }
      });

      const groupData: Partial<CommandGroup> = {
        name: name.trim(),
        description: description.trim(),
        tags: tags,
        commands: allLines,
      };

      await CommandGroupAPI.createCommandGroup(groupData as CommandGroup);
      emit("success", `命令组「${name.trim()}」创建成功`, 1, []);
    } else {
      const createdIds: string[] = [];
      const prefix = name.trim();

      for (let i = 0; i < props.outputBlocks.length; i++) {
        const block = props.outputBlocks[i];
        if (!block) continue;

        const blockLines = block
          .split("\n")
          .map((l: string) => l.trim())
          .filter((l: string) => l !== "");
        if (blockLines.length === 0) continue;

        const seq = String(i + 1).padStart(2, "0");

        const groupData: Partial<CommandGroup> = {
          name: `${prefix}${seq}`,
          description: description.trim(),
          tags: tags,
          commands: blockLines,
        };

        const result = await CommandGroupAPI.createCommandGroup(
          groupData as CommandGroup,
        );
        createdIds.push(String(result?.id ?? ""));
      }

      emit(
        "success",
        `成功创建 ${createdIds.length} 个命令组`,
        createdIds.length,
        createdIds,
      );
    }

    emit("close");
  } catch (err: any) {
    console.error("创建命令组失败:", err);
    emit("toast", "创建失败: " + (err.message || err));
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
              class="h-5 w-5 text-success"
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
            发送到命令管理
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
          <!-- 模式选择 -->
          <div class="space-y-3">
            <label class="text-sm font-medium text-text-primary"
              >创建模式</label
            >
            <div class="grid grid-cols-2 gap-3">
              <div
                class="mode-card"
                :class="{ active: mode === 'merge' }"
                @click="mode = 'merge'"
              >
                <div class="mode-icon">📦</div>
                <div class="mode-title">合并为一个命令组</div>
                <div class="mode-desc">所有配置块合并</div>
              </div>
              <div
                class="mode-card"
                :class="{ active: mode === 'split' }"
                @click="mode = 'split'"
              >
                <div class="mode-icon">📂</div>
                <div class="mode-title">分开创建多个命令组</div>
                <div class="mode-desc">每个配置块独立</div>
              </div>
            </div>
          </div>

          <!-- 基本信息 -->
          <div class="space-y-4">
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">
                {{ mode === "merge" ? "命令组名称" : "名称前缀" }}
                <span class="text-error">*</span>
              </label>
              <input
                v-model="form.name"
                type="text"
                class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
              />
            </div>

            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">描述</label>
              <input
                v-model="form.description"
                type="text"
                class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
              />
            </div>

            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">标签</label>
              <div class="flex flex-wrap gap-2 mb-2">
                <span
                  v-for="(tag, idx) in form.tags"
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

            <div class="preview-box">
              <div class="preview-icon">📊</div>
              <div class="preview-content">
                <span class="preview-text">{{ sendPreview.message }}</span>
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
            {{ saving ? "创建中..." : "确认创建" }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 模式选择卡片 */
.mode-card {
  @apply relative p-4 rounded-xl border border-border bg-bg-secondary/50 
         cursor-pointer transition-all duration-200;
}
.mode-card:hover {
  @apply border-accent/30 bg-bg-secondary/80;
}
.mode-card.active {
  @apply border-accent bg-accent/10 ring-1 ring-accent/20;
}
.mode-icon {
  @apply text-2xl mb-2;
}
.mode-title {
  @apply text-sm font-semibold text-text-primary;
}
.mode-desc {
  @apply text-xs text-text-muted mt-1;
}

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
