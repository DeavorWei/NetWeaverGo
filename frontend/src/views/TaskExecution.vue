 <template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <p class="text-sm text-text-muted">管理和执行已创建的任务绑定组合</p>
      <div class="flex gap-3">
        <button
          v-if="executionView.active && isRunning"
          @click="stopExecution"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-error/10 border border-error/30 text-error hover:bg-error hover:text-white"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="6" width="12" height="12" rx="1"/></svg>
          停止任务
        </button>
        <button
          @click="goToTaskCreate"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-bg-card border border-border text-text-muted hover:text-text-primary hover:border-accent/50"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          创建新任务
        </button>
        <button
          @click="loadTasks"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-bg-card border border-border text-text-muted hover:text-text-primary hover:border-accent/50"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          刷新
        </button>
      </div>
    </div>

    <!-- 搜索和筛选 -->
    <div class="flex items-center gap-4 flex-shrink-0">
      <div class="relative flex-1 max-w-md">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="搜索任务..."
          class="w-full pl-10 pr-4 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
        />
      </div>
      <select
        v-model="filterStatus"
        class="px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 transition-all"
      >
        <option value="">全部状态</option>
        <option value="pending">待执行</option>
        <option value="running">执行中</option>
        <option value="completed">已完成</option>
        <option value="failed">失败</option>
      </select>
      <select
        v-model="filterMode"
        class="px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 transition-all"
      >
        <option value="">全部模式</option>
        <option value="group">模式A（命令组→设备组）</option>
        <option value="binding">模式B（IP绑定→独立命令）</option>
      </select>
    </div>

    <!-- 执行视图（正在运行时显示） -->
    <template v-if="executionView.active">
      <!-- 进度条 -->
      <div class="flex-shrink-0 space-y-1.5">
        <div class="flex items-center justify-between text-xs text-text-muted">
          <span>{{ executionView.taskName }} - 总体进度</span>
          <span class="font-mono">{{ progressPercent }}%</span>
        </div>
        <div class="h-2 bg-bg-card rounded-full overflow-hidden border border-border">
          <div
            class="h-full rounded-full transition-all duration-500 ease-out"
            :class="progressPercent === 100 ? 'bg-success' : 'bg-accent'"
            :style="{ width: progressPercent + '%' }"
          ></div>
        </div>
      </div>

            <!-- 设备卡片网格 - 使用虚拟滚动优化 -->
      <div class="flex-1 overflow-auto scrollbar-custom min-h-0 relative" ref="devicesContainer">
        <div v-if="execDevices.length === 0 && (isRunning || awaitingSnapshot)" class="flex flex-col items-center justify-center h-48 text-text-muted gap-3">
          <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
          <p class="text-sm">正在初始化任务...</p>
        </div>
        <div v-else>
          <!-- 性能提示 -->
          <div v-if="execDevices.length > 50" class="text-xs text-text-muted mb-2 px-1">
            共 {{ execDevices.length }} 台设备，显示前 {{ visibleDeviceCount }} 台活跃设备
            <button v-if="execDevices.length > visibleDeviceCount" @click="showAllDevices = !showAllDevices" class="ml-2 text-accent hover:underline">
              {{ showAllDevices ? '收起' : '显示全部' }}
            </button>
          </div>
          
          <!-- 虚拟化设备网格 -->
          <div class="grid grid-cols-3 gap-4">
            <div
              v-for="dev in visibleDevices"
              :key="dev.ip"
              class="bg-bg-card border rounded-xl overflow-hidden shadow-card transition-all duration-300"
              :class="statusBorder(dev.status)"
            >
              <div class="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-panel">
                <span class="font-mono text-sm font-semibold text-text-primary">{{ dev.ip }}</span>
                <span class="flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full border" :class="statusBadge(dev.status)">
                  <span class="w-1.5 h-1.5 rounded-full" :class="statusDot(dev.status)"></span>
                  {{ statusLabel(dev.status) }}
                </span>
              </div>
              <!-- 优化后的终端区域 - 使用虚拟滚动 -->
              <VirtualLogTerminal
                :logs="dev.logs"
                :total-count="dev.logCount"
                :truncated="dev.truncated"
                :device-ip="dev.ip"
              />
              <div
                v-if="suspendSessions[dev.ip]"
                class="border-t border-warning/40 bg-warning/5 px-3 py-2"
              >
                <div class="flex items-center justify-between mb-2">
                  <div class="text-xs text-warning font-medium">异常干预（当前设备已挂起）</div>
                  <span
                    class="w-5 h-5 rounded-full border border-warning/40 text-warning text-[11px] flex items-center justify-center cursor-help"
                    title="继续下一条：忽略本次异常继续执行；放弃此指令：跳过当前报错命令进入下一条；切断连接：终止该设备后续执行。"
                  >?</span>
                </div>
                <div class="grid grid-cols-3 gap-2">
                  <button @click="resolveSuspend(dev.ip, 'C')" class="py-1.5 text-xs font-medium rounded border border-success/40 text-success bg-success/10 hover:bg-success hover:text-white transition-all duration-200">继续下一条</button>
                  <button @click="resolveSuspend(dev.ip, 'S')" class="py-1.5 text-xs font-medium rounded border border-accent/40 text-accent bg-accent/10 hover:bg-accent hover:text-white transition-all duration-200">放弃此指令</button>
                  <button @click="resolveSuspend(dev.ip, 'A')" class="py-1.5 text-xs font-medium rounded border border-error/40 text-error bg-error/10 hover:bg-error hover:text-white transition-all duration-200">切断连接</button>
                </div>
              </div>
            </div>
          </div>
          
          <!-- 更多设备提示 -->
          <div v-if="!showAllDevices && execDevices.length > visibleDeviceCount" class="text-center py-4 text-text-muted text-sm">
            还有 {{ execDevices.length - visibleDeviceCount }} 台设备已完成或等待中
            <button @click="showAllDevices = true" class="ml-2 text-accent hover:underline">显示全部</button>
          </div>
        </div>
      </div>

      <!-- 返回列表按钮 -->
      <div v-if="!isRunning" class="flex-shrink-0 text-center py-2">
        <button @click="closeExecutionView" class="text-sm text-accent hover:text-accent-glow transition-colors">
          ← 返回任务列表
        </button>
      </div>
    </template>

    <!-- 任务列表视图 -->
    <template v-else>
      <div class="flex-1 overflow-auto scrollbar-custom min-h-0">
        <div v-if="loading" class="flex items-center justify-center h-48">
          <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
        </div>
        <div v-else-if="filteredTasks.length === 0" class="flex flex-col items-center justify-center h-48 text-text-muted gap-3">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-12 h-12 opacity-30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="9" y1="9" x2="15" y2="15"/><line x1="15" y1="9" x2="9" y2="15"/></svg>
          <p class="text-sm">暂无任务，请前往「任务创建」页面创建</p>
        </div>
        <div v-else class="grid grid-cols-2 gap-4">
          <div
            v-for="task in filteredTasks"
            :key="task.id"
            class="bg-bg-card border border-border rounded-xl overflow-hidden shadow-card hover:border-accent/30 transition-all duration-300 group/card"
          >
            <!-- 卡片头部 -->
            <div class="flex items-start justify-between px-4 py-3 border-b border-border bg-bg-panel">
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <h3 class="text-sm font-semibold text-text-primary truncate">{{ task.name }}</h3>
                  <span
                    class="flex-shrink-0 text-xs px-2 py-0.5 rounded-full border font-medium"
                    :class="task.mode === 'group'
                      ? 'bg-info/10 border-info/30 text-info'
                      : 'bg-warning/10 border-warning/30 text-warning'"
                  >{{ task.mode === 'group' ? '模式A' : '模式B' }}</span>
                  <span
                    class="flex-shrink-0 flex items-center gap-1 text-xs px-2 py-0.5 rounded-full border font-medium"
                    :class="taskStatusBadge(task.status)"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" :class="taskStatusDot(task.status)"></span>
                    {{ taskStatusLabel(task.status) }}
                  </span>
                </div>
                <p class="text-xs text-text-muted line-clamp-1 mt-1">{{ task.description || '暂无描述' }}</p>
              </div>
              <!-- 操作按钮 -->
              <div class="flex items-center gap-1 ml-2">
                <button
                  @click="openTaskDetail(task)"
                  class="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10 transition-colors"
                  title="查看详情"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                </button>
                <button
                  @click="openTaskEdit(task)"
                  :disabled="task.status === 'running'"
                  class="p-1.5 rounded-md text-text-muted transition-colors"
                  :class="task.status !== 'running' ? 'hover:text-warning hover:bg-warning/10' : 'opacity-40 cursor-not-allowed'"
                  :title="task.status !== 'running' ? '编辑任务' : '任务执行中不可编辑'"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                </button>
                <button
                  @click="showExecutionHistory(task)"
                  class="p-1.5 rounded-md text-text-muted hover:text-info hover:bg-info/10 transition-colors"
                  title="查看执行历史"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 3v5h5"/><path d="M3.05 13A9 9 0 1 0 6 5.3L3 8"/><path d="M12 7v5l4 2"/></svg>
                </button>
                <button
                  @click="executeTask(task)"
                  :disabled="isRunning"
                  class="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10 transition-colors"
                  :class="isRunning ? 'opacity-50 cursor-not-allowed' : ''"
                  title="执行"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                </button>
                <button
                  @click="confirmDelete(task)"
                  class="p-1.5 rounded-md text-text-muted hover:text-error hover:bg-error/10 transition-colors"
                  title="删除"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                </button>
              </div>
            </div>

            <!-- 绑定详情 -->
            <div class="px-4 py-3">
              <template v-if="task.mode === 'group'">
                <div v-for="(item, idx) in task.items" :key="idx" class="flex items-center gap-2 text-xs">
                  <span class="px-2 py-0.5 rounded bg-accent/10 border border-accent/20 text-accent font-mono truncate max-w-[200px]">
                    命令组: {{ item.commandGroupId.substring(0, 8) }}...
                  </span>
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 text-text-muted flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
                  <span class="text-text-secondary">{{ item.deviceIDs.length }} 台设备</span>
                </div>
              </template>
              <template v-else>
                <div class="space-y-1">
                  <div v-for="(item, idx) in task.items.slice(0, 3)" :key="idx" class="flex items-center gap-2 text-xs">
                    <span class="font-mono text-text-secondary">{{ item.deviceIDs[0] }}</span>
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 text-text-muted flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
                    <span class="text-text-muted truncate">{{ item.commands.length }} 条命令</span>
                  </div>
                  <div v-if="task.items.length > 3" class="text-xs text-text-muted">
                    +{{ task.items.length - 3 }} 台设备...
                  </div>
                </div>
              </template>
            </div>

            <!-- 标签和时间 -->
            <div class="px-4 py-2 border-t border-border bg-bg-secondary/30 text-xs text-text-muted flex items-center justify-between">
              <div class="flex items-center gap-1.5 overflow-hidden">
                <span v-for="tag in task.tags.slice(0, 3)" :key="tag"
                  class="px-1.5 py-0.5 rounded bg-accent/10 border border-accent/20 text-accent truncate"
                >{{ tag }}</span>
              </div>
              <span class="flex-shrink-0 ml-2">{{ formatDate(task.updatedAt) }}</span>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- 删除确认弹窗 -->
    <Transition name="modal">
      <div v-if="deleteModal.show" class="fixed inset-0 z-50 flex items-center justify-center">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="deleteModal.show = false"></div>
        <div class="relative bg-bg-card border border-error/30 rounded-xl shadow-2xl max-w-sm w-full mx-4 overflow-hidden animate-slide-in">
          <div class="px-5 py-4 space-y-3">
            <h3 class="text-sm font-semibold text-text-primary">确认删除</h3>
            <p class="text-xs text-text-muted">确定要删除任务「{{ deleteModal.taskName }}」吗？此操作不可撤销。</p>
          </div>
          <div class="flex justify-end gap-3 px-5 py-3 border-t border-border">
            <button @click="deleteModal.show = false" class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-panel border border-border text-text-secondary hover:text-text-primary transition-all">取消</button>
            <button @click="doDelete" class="px-4 py-2 rounded-lg text-sm font-semibold bg-error hover:bg-error/80 text-white transition-all">删除</button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Toast 通知 -->
    <Transition name="toast">
      <div v-if="showToast" class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50">
        <div class="flex items-center gap-2 px-5 py-3 rounded-xl shadow-2xl border"
          :class="toastType === 'success' ? 'bg-success/10 border-success/30 text-success' : 'bg-error/10 border-error/30 text-error'"
        >
          <span class="text-sm font-medium">{{ toastMessage }}</span>
        </div>
      </div>
    </Transition>

    <!-- 执行历史抽屉 -->
    <ExecutionHistoryDrawer
      v-model="historyDrawer.show"
      :task-group-id="historyDrawer.taskGroupId"
      :task-group-name="historyDrawer.taskGroupName"
    />

    <TaskDetailModal
      v-model="detailModal.show"
      :loading="detailModal.loading"
      :detail="detailModal.detail"
      @edit="editFromDetail"
    />

    <TaskEditModal
      v-model="editModal.show"
      :task="editModal.task"
      :loading="editModal.loading"
      :saving="editModal.saving"
      :all-devices="editReferences.devices"
      :command-groups="editReferences.commandGroups"
      @save="saveTaskEdit"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  CommandGroupAPI,
  DeviceAPI,
  TaskGroupAPI
} from '../services/api'
import type {
  CommandGroup,
  DeviceAsset,
  DeviceViewState,
  TaskGroup,
  TaskGroupDetailViewModel
} from '../services/api'
import { useEngineStore } from '../stores/engineStore'
import VirtualLogTerminal from '../components/task/VirtualLogTerminal.vue'
import ExecutionHistoryDrawer from '../components/task/ExecutionHistoryDrawer.vue'
import TaskDetailModal from '../components/task/TaskDetailModal.vue'
import TaskEditModal from '../components/task/TaskEditModal.vue'

const router = useRouter()
const engineStore = useEngineStore()

// ================== 状态定义 ==================
const loading = ref(false)
const tasks = ref<TaskGroup[]>([])
const searchQuery = ref('')
const filterStatus = ref('')
const filterMode = ref('')

// 执行视图状态
const executionView = ref({
  active: false,
  taskId: '',
  taskName: ''
})
const awaitingSnapshot = ref(false)
let snapshotTimeoutTimer: ReturnType<typeof setTimeout> | null = null
const SNAPSHOT_TIMEOUT = 10000 // 10秒超时
let snapshotPollTimer: ReturnType<typeof setInterval> | null = null
const SNAPSHOT_POLL_INTERVAL = 1000 // 1秒兜底拉取

// 删除弹窗
const deleteModal = ref({ show: false, taskId: '', taskName: '' })

// 执行历史抽屉
const historyDrawer = ref({
  show: false,
  taskGroupId: '',
  taskGroupName: ''
})

// 任务详情弹窗
const detailModal = ref({
  show: false,
  loading: false,
  detail: null as TaskGroupDetailViewModel | null
})

// 任务编辑弹窗
const editModal = ref({
  show: false,
  loading: false,
  saving: false,
  task: null as TaskGroup | null
})

const editReferences = ref({
  devices: [] as DeviceAsset[],
  commandGroups: [] as CommandGroup[]
})

// 虚拟滚动优化状态
const showAllDevices = ref(false)
const VISIBLE_DEVICE_LIMIT = 30 // 默认显示的设备数量限制

// Toast
const showToast = ref(false)
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
let toastTimer: ReturnType<typeof setTimeout> | null = null

function triggerToast(msg: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = msg
  toastType.value = type
  showToast.value = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { showToast.value = false }, 3000)
}

// ================== 计算属性 - 从 Store 获取 ==================
const executionSnapshot = computed(() => engineStore.executionSnapshot)
const isRunning = computed(() => engineStore.isRunning)
const progressPercent = computed(() => executionSnapshot.value?.progress ?? 0)
const execDevices = computed(() => executionSnapshot.value?.devices ?? [])
const suspendSessions = computed(() => engineStore.suspendSessions)

// ================== 虚拟滚动优化计算属性 ==================
// 可见设备数量限制
const visibleDeviceCount = computed(() => 
  showAllDevices.value ? execDevices.value.length : VISIBLE_DEVICE_LIMIT
)

// 可见设备列表（优先显示运行中和错误的设备）
const visibleDevices = computed(() => {
  const devices = execDevices.value as DeviceViewState[]
  
  // 如果显示全部，直接返回
  if (showAllDevices.value) {
    return devices
  }
  
  // 优先显示活跃设备（运行中、错误、等待）
  const activeStatuses = ['running', 'error', 'waiting']
  const active = devices.filter((d: DeviceViewState) => activeStatuses.includes(d.status))
  const inactive = devices.filter((d: DeviceViewState) => !activeStatuses.includes(d.status))
  
  // 优先返回活跃设备，如果活跃设备不足限制数量，补充已完成设备
  if (active.length >= VISIBLE_DEVICE_LIMIT) {
    return active.slice(0, VISIBLE_DEVICE_LIMIT)
  }
  
  return [...active, ...inactive].slice(0, VISIBLE_DEVICE_LIMIT)
})

// ================== 生命周期 ==================
onMounted(() => {
  void syncExecutionView()
  void loadTasks()
})

onUnmounted(() => {
  clearSnapshotTimeout()
  stopSnapshotPolling()
  if (toastTimer) {
    clearTimeout(toastTimer)
    toastTimer = null
  }
})

watch(isRunning, (running, wasRunning) => {
  if (!running && wasRunning && executionView.value.active) {
    stopSnapshotPolling()
    void loadTasks()
  }
})

// ================== 过滤逻辑（任务列表） ==================
const filteredTasks = computed(() => {
  let result = tasks.value
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    result = result.filter(t =>
      t.name.toLowerCase().includes(q) ||
      t.description.toLowerCase().includes(q) ||
      t.tags.some(tag => tag.toLowerCase().includes(q))
    )
  }
  if (filterStatus.value) {
    result = result.filter(t => t.status === filterStatus.value)
  }
  if (filterMode.value) {
    result = result.filter(t => t.mode === filterMode.value)
  }
  return result
})


// 加载任务列表
async function loadTasks() {
  loading.value = true
  try {
    const result = await TaskGroupAPI.listTaskGroups()
    tasks.value = result || []
  } catch (err) {
    console.error('加载任务列表失败:', err)
    tasks.value = []
  } finally {
    loading.value = false
  }
}

async function syncExecutionView() {
  const active = await engineStore.syncExecutionState()
  const snapshot = engineStore.executionSnapshot
  if (active || snapshot) {
    executionView.value.active = true
    executionView.value.taskName = snapshot?.taskName || '任务执行'
  }
}

function startSnapshotPolling() {
  if (snapshotPollTimer) return
  snapshotPollTimer = setInterval(() => {
    if (!executionView.value.active) {
      stopSnapshotPolling()
      return
    }
    // 仅在初始化等待窗口或运行窗口启用兜底拉取，避免额外负载。
    if (!awaitingSnapshot.value && !isRunning.value) {
      stopSnapshotPolling()
      return
    }
    void syncExecutionView()
  }, SNAPSHOT_POLL_INTERVAL)
}

function stopSnapshotPolling() {
  if (snapshotPollTimer) {
    clearInterval(snapshotPollTimer)
    snapshotPollTimer = null
  }
}

async function handleSnapshotTimeout() {
  if (!awaitingSnapshot.value) return

  // 二次确认：先同步后端状态，避免快照晚到导致误判超时失败。
  const active = await engineStore.syncExecutionState()
  const snapshot = engineStore.executionSnapshot
  if (active || snapshot) {
    awaitingSnapshot.value = false
    clearSnapshotTimeout()
    executionView.value.active = true
    executionView.value.taskName = snapshot?.taskName || executionView.value.taskName || '任务执行'
    startSnapshotPolling()
    return
  }

  console.warn('[TaskExecution] 快照超时，重置UI状态')
  awaitingSnapshot.value = false
  triggerToast('任务执行超时，请检查设备连接配置', 'error')
  executionView.value.active = false
  stopSnapshotPolling()
}

// 设置快照超时保护
function startSnapshotTimeout() {
  if (snapshotTimeoutTimer) {
    clearTimeout(snapshotTimeoutTimer)
  }
  snapshotTimeoutTimer = setTimeout(() => {
    void handleSnapshotTimeout()
  }, SNAPSHOT_TIMEOUT)
}

function clearSnapshotTimeout() {
  if (snapshotTimeoutTimer) {
    clearTimeout(snapshotTimeoutTimer)
    snapshotTimeoutTimer = null
  }
}

// 执行任务
async function executeTask(task: TaskGroup) {
  if (isRunning.value) return

  engineStore.reset()
  awaitingSnapshot.value = true
  
  // 启动超时保护
  startSnapshotTimeout()
  startSnapshotPolling()
  
  executionView.value = {
    active: true,
    taskId: task.id,
    taskName: task.name
  }

  try {
    await TaskGroupAPI.startTaskGroup(task.id)
  } catch (err: any) {
    console.error('执行任务失败:', err)
    triggerToast(`执行失败: ${err?.message || err}`, 'error')
    executionView.value.active = false
    clearSnapshotTimeout()
    stopSnapshotPolling()
  }
}

// 监听快照变化，收到快照后清除超时
watch(executionSnapshot, (snapshot) => {
  if (snapshot) {
    awaitingSnapshot.value = false
    clearSnapshotTimeout()
    executionView.value.active = true
    if (snapshot.taskName) {
      executionView.value.taskName = snapshot.taskName
    }
  }
})

// 删除任务
function confirmDelete(task: TaskGroup) {
  deleteModal.value = { show: true, taskId: task.id, taskName: task.name }
}

async function doDelete() {
  try {
    await TaskGroupAPI.deleteTaskGroup(deleteModal.value.taskId)
    deleteModal.value.show = false
    triggerToast('任务已删除', 'success')
    void loadTasks()
  } catch (err: any) {
    triggerToast(`删除失败: ${err?.message || err}`, 'error')
  }
}

// 关闭执行视图
function closeExecutionView() {
  executionView.value.active = false
  awaitingSnapshot.value = false
  clearSnapshotTimeout()
  stopSnapshotPolling()
  engineStore.reset()
  loadTasks()
}

async function stopExecution() {
  if (!confirm('确定要停止当前执行任务吗？')) {
    return
  }

  try {
    await engineStore.stopEngine()
    triggerToast('已发送停止信号')
  } catch (err: any) {
    triggerToast(`停止失败: ${err?.message || err}`, 'error')
  }
}

// Suspend 处理
function resolveSuspend(ip: string, action: 'C' | 'S' | 'A') {
  engineStore.resolveSuspend(ip, action)
}

// 导航
function goToTaskCreate() {
  router.push('/tasks')
}

// 查看执行历史
function showExecutionHistory(task: TaskGroup) {
  historyDrawer.value = {
    show: true,
    taskGroupId: task.id,
    taskGroupName: task.name
  }
}

async function ensureEditReferences() {
  if (editReferences.value.devices.length > 0 && editReferences.value.commandGroups.length > 0) {
    return
  }

  const [devices, commandGroups] = await Promise.all([
    DeviceAPI.listDevices(),
    CommandGroupAPI.listCommandGroups()
  ])

  editReferences.value = {
    devices: devices || [],
    commandGroups: commandGroups || []
  }
}

async function openTaskDetail(task: TaskGroup) {
  detailModal.value.show = true
  detailModal.value.loading = true
  detailModal.value.detail = null

  try {
    detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(task.id)
  } catch (err: any) {
    detailModal.value.show = false
    triggerToast(`加载任务详情失败: ${err?.message || err}`, 'error')
  } finally {
    detailModal.value.loading = false
  }
}

async function openTaskEdit(task: TaskGroup) {
  if (task.status === 'running') {
    triggerToast('任务执行中不可编辑', 'error')
    return
  }

  editModal.value.show = true
  editModal.value.loading = true
  editModal.value.task = null

  try {
    const [freshTask] = await Promise.all([
      TaskGroupAPI.getTaskGroup(task.id),
      ensureEditReferences()
    ])
    editModal.value.task = freshTask
  } catch (err: any) {
    editModal.value.show = false
    triggerToast(`加载编辑数据失败: ${err?.message || err}`, 'error')
  } finally {
    editModal.value.loading = false
  }
}

async function editFromDetail() {
  const currentTask = detailModal.value.detail?.task
  if (!currentTask) return
  detailModal.value.show = false
  await openTaskEdit(currentTask)
}

async function saveTaskEdit(payload: TaskGroup) {
  if (!editModal.value.task) return

  editModal.value.saving = true
  try {
    const updated = await TaskGroupAPI.updateTaskGroup(editModal.value.task.id, payload)
    editModal.value.task = updated || payload
    editModal.value.show = false
    triggerToast('任务更新成功', 'success')
    await loadTasks()

    if (detailModal.value.show && detailModal.value.detail?.task.id === payload.id) {
      detailModal.value.loading = true
      detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(payload.id)
      detailModal.value.loading = false
    }
  } catch (err: any) {
    triggerToast(`保存失败: ${err?.message || err}`, 'error')
    await loadTasks()

    if (detailModal.value.show && detailModal.value.detail?.task.id === payload.id) {
      try {
        detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(payload.id)
      } catch (detailErr) {
        console.error('刷新任务详情失败:', detailErr)
      }
    }
  } finally {
    editModal.value.saving = false
  }
}

// ================== 状态样式 ==================
function statusBorder(s: string) {
  switch (s) {
    case 'running': return 'border-accent/50'
    case 'success': return 'border-success/50'
    case 'error':   return 'border-error/50'
    case 'aborted': return 'border-error/50'
    case 'waiting': return 'border-warning/40'
    default:        return 'border-border'
  }
}
function statusBadge(s: string) {
  switch (s) {
    case 'running': return 'bg-accent/10 border-accent/30 text-accent'
    case 'success': return 'bg-success/10 border-success/30 text-success'
    case 'error':   return 'bg-error/10 border-error/30 text-error'
    case 'aborted': return 'bg-error/10 border-error/30 text-error'
    case 'waiting': return 'bg-warning/10 border-warning/30 text-warning'
    default:        return 'bg-bg-panel border-border text-text-muted'
  }
}
function statusDot(s: string) {
  switch (s) {
    case 'running': return 'bg-accent animate-pulse'
    case 'success': return 'bg-success'
    case 'error':   return 'bg-error'
    case 'aborted': return 'bg-error'
    case 'waiting': return 'bg-warning animate-pulse'
    default:        return 'bg-text-muted'
  }
}
function statusLabel(s: string) {
  const map: Record<string, string> = { running: '执行中', success: '成功', error: '失败', aborted: '已终止', waiting: '等待', idle: '空闲' }
  return map[s] ?? s
}
// 任务状态样式
function taskStatusBadge(s: string) {
  switch (s) {
    case 'pending':   return 'bg-bg-panel border-border text-text-muted'
    case 'running':   return 'bg-accent/10 border-accent/30 text-accent'
    case 'completed': return 'bg-success/10 border-success/30 text-success'
    case 'failed':    return 'bg-error/10 border-error/30 text-error'
    default:          return 'bg-bg-panel border-border text-text-muted'
  }
}
function taskStatusDot(s: string) {
  switch (s) {
    case 'pending':   return 'bg-text-muted'
    case 'running':   return 'bg-accent animate-pulse'
    case 'completed': return 'bg-success'
    case 'failed':    return 'bg-error'
    default:          return 'bg-text-muted'
  }
}
function taskStatusLabel(s: string) {
  const map: Record<string, string> = { pending: '待执行', running: '执行中', completed: '已完成', failed: '失败' }
  return map[s] ?? s
}

function formatDate(dateStr: string) {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`
}
</script>

<style scoped>
.modal-enter-active, .modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from, .modal-leave-to {
  opacity: 0;
}
.toast-enter-active {
  transition: all 0.3s ease-out;
}
.toast-leave-active {
  transition: all 0.2s ease-in;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(-50%) translateY(20px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(10px);
}
</style>
