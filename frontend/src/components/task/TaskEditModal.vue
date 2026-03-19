<template>
  <Transition name="modal">
    <div v-if="modelValue" class="fixed inset-0 z-50 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="closeModal"></div>
      <div class="relative w-full max-w-6xl mx-4 max-h-[90vh] overflow-hidden rounded-2xl border border-border bg-bg-card shadow-2xl animate-slide-in flex flex-col">
        <div class="flex items-start justify-between gap-4 px-6 py-5 border-b border-border bg-bg-panel">
          <div>
            <h3 class="text-base font-semibold text-text-primary">编辑任务</h3>
            <p class="text-xs text-text-muted mt-1">任务执行中不可编辑，模式不可切换</p>
          </div>
          <button
            @click="closeModal"
            class="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-secondary transition-colors"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div class="flex-1 overflow-y-auto scrollbar-custom p-6">
          <div v-if="loading" class="h-64 flex items-center justify-center">
            <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
          </div>
          <div v-else-if="!task" class="h-64 flex items-center justify-center text-sm text-text-muted">暂无可编辑任务</div>
          <template v-else>
            <div class="grid grid-cols-2 gap-4">
              <div class="space-y-4">
                <div>
                  <label class="block text-sm font-medium text-text-primary mb-2">任务名称</label>
                  <input
                    v-model="form.name"
                    type="text"
                    class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50"
                    placeholder="输入任务名称"
                  />
                </div>

                <div>
                  <label class="block text-sm font-medium text-text-primary mb-2">任务描述</label>
                  <textarea
                    v-model="form.description"
                    rows="3"
                    class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 resize-none"
                    placeholder="输入任务描述"
                  ></textarea>
                </div>

                <div>
                  <label class="block text-sm font-medium text-text-primary mb-2">标签</label>
                  <div class="flex flex-wrap gap-2 mb-3">
                    <span
                      v-for="(tag, index) in form.tags"
                      :key="`${tag}-${index}`"
                      class="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs bg-accent/10 border border-accent/20 text-accent"
                    >
                      {{ tag }}
                      <button @click="removeTag(index)" class="hover:text-error transition-colors">
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <line x1="18" y1="6" x2="6" y2="18"/>
                          <line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                      </button>
                    </span>
                  </div>
                  <div class="flex gap-2">
                    <input
                      v-model="newTag"
                      type="text"
                      class="flex-1 px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50"
                      placeholder="添加标签"
                      @keydown.enter.prevent="addTag"
                    />
                    <button
                      @click="addTag"
                      class="px-4 py-2 rounded-lg text-sm font-medium bg-accent/10 border border-accent/30 text-accent hover:bg-accent hover:text-white"
                    >
                      添加
                    </button>
                  </div>
                </div>

                <div class="rounded-xl border border-border bg-bg-panel p-4">
                  <label class="flex items-start justify-between gap-4">
                    <div>
                      <div class="text-sm font-medium text-text-primary">原始日志</div>
                      <p class="text-xs text-text-muted mt-1">开启后为每台设备额外保存完整 SSH 字节流，便于深度排障。</p>
                    </div>
                    <input
                      v-model="form.enableRawLog"
                      type="checkbox"
                      class="mt-1 h-4 w-4"
                    />
                  </label>
                </div>
              </div>

              <div class="rounded-xl border border-border bg-bg-panel p-4 space-y-3 h-fit">
                <div class="flex items-center justify-between">
                  <span class="text-sm text-text-muted">模式</span>
                  <span class="text-sm font-semibold text-text-primary">{{ modeLabel(task.mode) }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span class="text-sm text-text-muted">状态</span>
                  <span class="text-sm font-semibold text-text-primary">{{ statusLabel(task.status) }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span class="text-sm text-text-muted">设备库存</span>
                  <span class="text-sm font-mono text-text-primary">{{ allDevices.length }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span class="text-sm text-text-muted">命令组数量</span>
                  <span class="text-sm font-mono text-text-primary">{{ commandGroups.length }}</span>
                </div>
              </div>
            </div>

            <div v-if="task.mode === 'group'" class="mt-6 grid grid-cols-[320px,1fr] gap-4">
              <div class="rounded-xl border border-border bg-bg-panel p-4 space-y-3">
                <label class="block text-sm font-medium text-text-primary">命令组</label>
                <select
                  v-model="groupForm.commandGroupId"
                  class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50"
                >
                  <option value="">请选择命令组</option>
                  <option v-for="group in commandGroups" :key="group.id" :value="group.id">
                    {{ group.name }} ({{ group.commands.length }} 条命令)
                  </option>
                </select>
                <div class="rounded-lg border border-border bg-bg-card p-3 max-h-72 overflow-y-auto scrollbar-custom">
                  <div
                    v-for="(command, index) in selectedGroupCommands"
                    :key="`${groupForm.commandGroupId}-${index}`"
                    class="font-mono text-sm text-text-primary py-1 border-b border-border/40 last:border-0"
                  >
                    <span class="text-text-muted mr-2">{{ index + 1 }}.</span>{{ command }}
                  </div>
                  <div v-if="selectedGroupCommands.length === 0" class="text-sm text-text-muted">当前命令组暂无命令</div>
                </div>
              </div>

              <div class="rounded-xl border border-border bg-bg-panel p-4">
                <div class="flex items-center justify-between mb-3">
                  <h4 class="text-sm font-semibold text-text-primary">选择设备</h4>
                  <span class="text-xs text-text-muted">已选 {{ groupForm.deviceIDs.length }} 台</span>
                </div>
                <div class="grid grid-cols-2 gap-2 max-h-[420px] overflow-y-auto scrollbar-custom">
                  <label
                    v-for="device in allDevices"
                    :key="device.id"
                    class="flex items-start gap-3 rounded-lg border border-border bg-bg-card px-3 py-2 hover:border-accent/40 transition-colors cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      :checked="groupForm.deviceIDs.includes(device.id)"
                      @change="toggleGroupDevice(device.id)"
                      class="mt-1"
                    />
                    <div class="min-w-0">
                      <div class="font-mono text-sm text-text-primary">{{ device.ip }}</div>
                      <div class="text-xs text-text-muted mt-1">分组: {{ device.group || '未分组' }}</div>
                      <div class="flex flex-wrap gap-1 mt-2">
                        <span
                          v-for="tag in device.tags"
                          :key="tag"
                          class="px-1.5 py-0.5 rounded text-xs bg-accent/10 text-accent"
                        >
                          {{ tag }}
                        </span>
                        <span v-if="device.tags.length === 0" class="text-xs text-text-muted">无标签</span>
                      </div>
                    </div>
                  </label>
                </div>
              </div>
            </div>

            <div v-else class="mt-6 space-y-4">
              <div class="flex items-center justify-between">
                <h4 class="text-sm font-semibold text-text-primary">任务项编辑</h4>
                <button
                  @click="addBindingItem"
                  class="px-4 py-2 rounded-lg text-sm font-medium bg-accent text-white hover:bg-accent-glow"
                >
                  新增任务项
                </button>
              </div>

              <div
                v-for="(item, index) in bindingForm.items"
                :key="`binding-item-${index}`"
                class="rounded-xl border border-border bg-bg-panel p-4 space-y-4"
              >
                <div class="flex items-center justify-between">
                  <h5 class="text-sm font-semibold text-text-primary">任务项 {{ index + 1 }}</h5>
                  <button
                    @click="removeBindingItem(index)"
                    class="px-3 py-1.5 rounded-lg text-xs font-medium bg-error/10 border border-error/30 text-error hover:bg-error hover:text-white"
                  >
                    删除
                  </button>
                </div>

                <div class="grid grid-cols-[1fr,1.2fr] gap-4">
                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <label class="text-sm font-medium text-text-primary">设备概览</label>
                      <span class="text-xs text-text-muted">已选 {{ item.deviceIDs.length }} 台</span>
                    </div>
                    <div class="max-h-64 overflow-y-auto scrollbar-custom grid grid-cols-1 gap-2">
                      <label
                        v-for="device in allDevices"
                        :key="`${index}-${device.id}`"
                        class="flex items-start gap-3 rounded-lg border border-border bg-bg-card px-3 py-2 hover:border-accent/40 transition-colors cursor-pointer"
                      >
                        <input
                          type="checkbox"
                          :checked="item.deviceIDs.includes(device.id)"
                          @change="toggleBindingDevice(index, device.id)"
                          class="mt-1"
                        />
                        <div class="min-w-0">
                          <div class="font-mono text-sm text-text-primary">{{ device.ip }}</div>
                          <div class="text-xs text-text-muted mt-1">分组: {{ device.group || '未分组' }}</div>
                          <div class="flex flex-wrap gap-1 mt-2">
                            <span
                              v-for="tag in device.tags"
                              :key="tag"
                              class="px-1.5 py-0.5 rounded text-xs bg-accent/10 text-accent"
                            >
                              {{ tag }}
                            </span>
                            <span v-if="device.tags.length === 0" class="text-xs text-text-muted">无标签</span>
                          </div>
                        </div>
                      </label>
                    </div>
                  </div>

                  <div class="space-y-2">
                    <label class="text-sm font-medium text-text-primary">命令内容</label>
                    <textarea
                      v-model="item.commandsText"
                      rows="12"
                      class="w-full h-[280px] p-4 rounded-lg bg-terminal-bg text-terminal-text border border-border font-mono text-sm resize-none focus:outline-none focus:border-accent/50"
                      placeholder="每行输入一条命令"
                    ></textarea>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="formError" class="mt-5 rounded-lg border border-error/30 bg-error/10 px-4 py-3 text-sm text-error">
              {{ formError }}
            </div>
          </template>
        </div>

        <div class="flex justify-end gap-3 px-6 py-4 border-t border-border bg-bg-panel">
          <button
            @click="closeModal"
            class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-card border border-border text-text-secondary hover:text-text-primary"
          >
            取消
          </button>
          <button
            @click="submit"
            :disabled="loading || saving || !task"
            class="px-5 py-2 rounded-lg text-sm font-semibold bg-accent text-white hover:bg-accent-glow disabled:opacity-60"
          >
            {{ saving ? '保存中...' : '保存任务' }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { CommandGroup, DeviceAsset, TaskGroup, TaskItem } from '../../services/api'

type BindingItemForm = {
  deviceIDs: number[]
  commandsText: string
}

const props = defineProps<{
  modelValue: boolean
  task: TaskGroup | null
  allDevices: DeviceAsset[]
  commandGroups: CommandGroup[]
  loading: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'save', payload: TaskGroup): void
}>()

const form = reactive({
  name: '',
  description: '',
  tags: [] as string[],
  enableRawLog: false
})

const groupForm = reactive({
  commandGroupId: 0,
  deviceIDs: [] as number[]
})

const bindingForm = reactive({
  items: [] as BindingItemForm[]
})

const newTag = ref('')
const formError = ref('')

watch(
  () => [props.task, props.modelValue] as const,
  ([task, visible]) => {
    if (!visible || !task) {
      return
    }
    hydrateForm(task)
  },
  { immediate: true }
)

const selectedGroupCommands = computed(() => {
  const current = props.commandGroups.find((group) => group.id === groupForm.commandGroupId)
  return current?.commands ?? []
})

function hydrateForm(task: TaskGroup) {
  form.name = task.name
  form.description = task.description
  form.tags = [...task.tags]
  form.enableRawLog = Boolean(task.enableRawLog)
  newTag.value = ''
  formError.value = ''

  if (task.mode === 'group') {
    const normalized = normalizeGroupTask(task.items)
    groupForm.commandGroupId = normalized.commandGroupId
    groupForm.deviceIDs = normalized.deviceIDs
    bindingForm.items = []
    return
  }

  groupForm.commandGroupId = 0
  groupForm.deviceIDs = []
  bindingForm.items = task.items.map((item) => ({
    deviceIDs: [...item.deviceIDs],
    commandsText: item.commands.join('\n')
  }))

  if (bindingForm.items.length === 0) {
    bindingForm.items = [{ deviceIDs: [], commandsText: '' }]
  }
}

function normalizeGroupTask(items: TaskItem[]) {
  const deviceSet = new Set<number>()
  let commandGroupId = 0

  items.forEach((item) => {
    if (!commandGroupId && item.commandGroupId) {
      commandGroupId = parseInt(item.commandGroupId, 10) || 0
    }
    item.deviceIDs.forEach((id) => deviceSet.add(id))
  })

  return {
    commandGroupId,
    deviceIDs: Array.from(deviceSet)
  }
}

function closeModal() {
  emit('update:modelValue', false)
}

function addTag() {
  const tag = newTag.value.trim()
  if (tag && !form.tags.includes(tag)) {
    form.tags.push(tag)
  }
  newTag.value = ''
}

function removeTag(index: number) {
  form.tags.splice(index, 1)
}

function toggleGroupDevice(deviceID: number) {
  if (groupForm.deviceIDs.includes(deviceID)) {
    groupForm.deviceIDs.splice(groupForm.deviceIDs.indexOf(deviceID), 1)
    return
  }
  groupForm.deviceIDs.push(deviceID)
}

function addBindingItem() {
  bindingForm.items.push({ deviceIDs: [], commandsText: '' })
}

function removeBindingItem(index: number) {
  if (bindingForm.items.length === 1) {
    bindingForm.items[0] = { deviceIDs: [], commandsText: '' }
    return
  }
  bindingForm.items.splice(index, 1)
}

function toggleBindingDevice(index: number, deviceID: number) {
  const item = bindingForm.items[index]
  if (!item) return

  if (item.deviceIDs.includes(deviceID)) {
    item.deviceIDs.splice(item.deviceIDs.indexOf(deviceID), 1)
    return
  }

  item.deviceIDs.push(deviceID)
}

function submit() {
  if (!props.task) return

  const name = form.name.trim()
  if (!name) {
    formError.value = '任务名称不能为空'
    return
  }

  const tags = form.tags
    .map((tag) => tag.trim())
    .filter((tag, index, array) => tag !== '' && array.indexOf(tag) === index)

  let items: TaskItem[] = []
  if (props.task.mode === 'group') {
    if (!groupForm.commandGroupId) {
      formError.value = '请选择命令组'
      return
    }
    if (groupForm.deviceIDs.length === 0) {
      formError.value = '请至少选择一台设备'
      return
    }

    items = [{
      commandGroupId: String(groupForm.commandGroupId),
      commands: [],
      deviceIDs: [...groupForm.deviceIDs]
    }]
  } else {
    items = bindingForm.items
      .map((item) => ({
        commandGroupId: '',
        commands: item.commandsText
          .split('\n')
          .map((line) => line.trim())
          .filter((line) => line !== ''),
        deviceIDs: [...item.deviceIDs]
      }))
      .filter((item) => item.deviceIDs.length > 0 && item.commands.length > 0)

    if (items.length === 0) {
      formError.value = '请至少保留一个包含设备和命令的任务项'
      return
    }
  }

  formError.value = ''
  emit('save', {
    id: props.task.id,
    name,
    description: form.description.trim(),
    deviceGroup: props.task.deviceGroup,
    commandGroup: props.task.commandGroup,
    maxWorkers: props.task.maxWorkers,
    timeout: props.task.timeout,
    mode: props.task.mode,
    items,
    tags,
    enableRawLog: form.enableRawLog,
    status: props.task.status,
    createdAt: props.task.createdAt,
    updatedAt: props.task.updatedAt
  })
}

function modeLabel(mode: string) {
  return mode === 'group' ? '模式A' : mode === 'binding' ? '模式B' : mode
}

function statusLabel(status: string) {
  const mapping: Record<string, string> = {
    pending: '待执行',
    running: '执行中',
    completed: '已完成',
    failed: '失败'
  }
  return mapping[status] ?? status
}
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.bg-terminal-bg {
  background-color: var(--color-terminal-bg);
}

.text-terminal-text {
  color: var(--color-terminal-text);
}
</style>
