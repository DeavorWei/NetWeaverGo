<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <div class="flex items-center gap-4">
        <p class="text-sm text-text-muted">管理和执行已创建的任务绑定组合</p>
      </div>

      <!-- 操作按钮区域 -->
      <div class="flex gap-3">
        <button
          v-if="executionView.active && isRunning"
          @click="stopExecution"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-error/10 border border-error/30 text-error hover:bg-error hover:text-white"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <rect x="6" y="6" width="12" height="12" rx="1" />
          </svg>
          停止任务
        </button>
        <button
          @click="goToTaskCreate"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-bg-card border border-border text-text-muted hover:text-text-primary hover:border-accent/50"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          创建新任务
        </button>
        <!-- 执行详情视图时显示返回按钮，否则显示刷新按钮 -->
        <button
          v-if="shouldShowExecutionView"
          @click="closeExecutionView"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-bg-card border border-border text-text-muted hover:text-text-primary hover:border-accent/50"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <polyline points="15 18 9 12 15 6" />
          </svg>
          返回任务列表
        </button>
        <button
          v-else
          @click="refreshTaskList"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-bg-card border border-border text-text-muted hover:text-text-primary hover:border-accent/50"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <polyline points="23 4 23 10 17 10" />
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
          </svg>
          刷新
        </button>
      </div>
    </div>

    <!-- ==================== 任务执行内容 ==================== -->
    <div class="flex-1 min-h-0 flex flex-col gap-4">
      <!-- 搜索和筛选 -->
      <div class="flex items-center gap-4 flex-shrink-0">
        <div class="relative flex-1 max-w-md">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-text-muted"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
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
          <option value="partial">部分成功</option>
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

      <div
        class="flex items-center justify-between gap-3 flex-shrink-0 px-3 py-2 rounded-lg border border-border bg-bg-card/70 text-xs text-text-muted"
      >
        <span
          >状态：tasks={{ tasks.length }} / filtered={{
            filteredTasks.length
          }}
          / active={{ shouldShowExecutionView ? "true" : "false" }} / running={{
            isRunning ? "true" : "false"
          }}
          / awaiting={{ awaitingSnapshot ? "true" : "false" }}</span
        >
        <span class="font-mono"
          >run={{
            executionView.runId || taskexecStore.currentRunId || "-"
          }}</span
        >
      </div>

      <!-- 执行视图（正在运行时显示） -->
      <template v-if="shouldShowExecutionView">
        <div class="flex-1 flex flex-col gap-4">
          <div
            class="bg-bg-card border border-border rounded-xl p-4 flex items-start justify-between gap-4"
          >
            <div class="space-y-2">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="text-sm font-semibold text-text-primary">
                  {{ executionView.taskName || "任务执行" }}
                </span>
                <span
                  class="px-2.5 py-1 rounded-full text-xs border flex items-center gap-1.5"
                  :class="taskStatusBadge(executionRunStatus)"
                >
                  <span
                    class="w-1.5 h-1.5 rounded-full"
                    :class="taskStatusDot(executionRunStatus)"
                  ></span>
                  {{ taskStatusLabel(executionRunStatus) }}
                </span>
                <span
                  v-if="executionView.taskType === 'topology'"
                  class="px-2 py-0.5 rounded text-xs bg-accent/10 border border-accent/20 text-accent"
                >
                  拓扑采集
                </span>
              </div>
              <p class="text-sm text-text-primary">
                {{ executionStatusSummary }}
              </p>
              <p
                v-if="executionStatusDetail"
                class="text-xs"
                :class="executionStatusDetailClass"
              >
                {{ executionStatusDetail }}
              </p>
            </div>
            <button
              v-if="
                executionView.taskType === 'topology' && isExecutionTerminal
              "
              @click="router.push('/topology')"
              class="px-3 py-2 rounded-lg text-sm font-medium border border-accent/30 text-accent hover:bg-accent/10 transition-colors"
            >
              查看拓扑图谱
            </button>
          </div>

          <!-- 进度条 -->
          <div class="flex-shrink-0 space-y-1.5">
            <div
              class="flex items-center justify-between text-xs text-text-muted"
            >
              <span>{{ executionView.taskName || "任务执行" }} - 总体进度</span>
              <span class="font-mono">{{ progressPercent }}%</span>
            </div>
            <div
              class="h-2 bg-bg-card rounded-full overflow-hidden border border-border"
            >
              <div
                class="h-full rounded-full transition-all duration-500 ease-out"
                :class="progressPercent === 100 ? 'bg-success' : 'bg-accent'"
                :style="{ width: progressPercent + '%' }"
              ></div>
            </div>
          </div>

          <!-- Stage 进度展示 (新运行时支持) -->
          <div v-if="executionStages.length > 0" class="flex-shrink-0">
            <StageProgress :stages="executionStages" :units="executionUnits" />
          </div>

          <!-- 拓扑采集计划证据 -->
          <div
            v-if="executionView.taskType === 'topology'"
            class="flex-shrink-0 bg-bg-card border border-border rounded-xl p-4 space-y-3"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <h4 class="text-sm font-semibold text-text-primary">
                  拓扑采集计划证据
                </h4>
                <p class="text-xs text-text-muted mt-1">
                  展示字段启停、命令来源与厂商来源，支持运行后复盘。
                </p>
              </div>
              <div class="text-xs text-text-muted text-right">
                <div>设备计划: {{ topologyCollectionPlanRows.length }}</div>
                <div>
                  启用字段: {{ topologyPlanEnabledCount }} / 禁用字段:
                  {{ topologyPlanDisabledCount }}
                </div>
              </div>
            </div>

            <div v-if="topologyPlanLoading" class="text-xs text-text-muted">
              正在加载采集计划快照...
            </div>
            <div
              v-else-if="topologyPlanError"
              class="text-xs text-error bg-error/10 border border-error/20 rounded-lg px-3 py-2"
            >
              加载采集计划失败：{{ topologyPlanError }}
            </div>
            <div
              v-else-if="topologyCollectionPlanRows.length === 0"
              class="text-xs text-text-muted"
            >
              暂无采集计划快照，待设备采集阶段产物生成后自动展示。
            </div>
            <div
              v-else
              class="space-y-3 max-h-64 overflow-auto scrollbar-custom pr-1"
            >
              <div
                v-for="plan in topologyCollectionPlanRows"
                :key="`${plan.artifactKey || '-'}:${plan.deviceIp}:${String(plan.generatedAt || '-')}`"
                class="rounded-lg border border-border bg-bg-panel/40 px-3 py-2 space-y-2"
              >
                <div class="flex items-center justify-between gap-3 text-xs">
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="font-mono text-text-primary">{{
                      plan.deviceIp
                    }}</span>
                    <span
                      class="px-2 py-0.5 rounded border bg-accent/10 border-accent/20 text-accent"
                    >
                      厂商: {{ plan.resolvedVendor || "-" }}
                    </span>
                    <span class="text-text-muted">
                      来源: {{ vendorSourceLabel(plan.vendorSource) }}
                    </span>
                  </div>
                  <span class="text-text-muted">
                    {{ formatDate(String(plan.generatedAt || "")) }}
                  </span>
                </div>

                <div class="text-xs text-text-muted">
                  字段: 启用 {{ enabledCommandCount(plan) }} / 禁用
                  {{ disabledCommandCount(plan) }}
                </div>

                <div class="flex flex-wrap gap-1.5">
                  <span
                    v-for="cmd in (plan.commands || []).slice(0, 6)"
                    :key="`${cmd.fieldKey}:${cmd.commandSource}`"
                    class="px-1.5 py-0.5 rounded border text-[11px]"
                    :class="
                      cmd.enabled
                        ? 'border-success/30 bg-success/10 text-success'
                        : 'border-border bg-bg-card text-text-muted'
                    "
                  >
                    {{ cmd.fieldKey }} ·
                    {{ commandSourceLabel(cmd.commandSource) }}
                  </span>
                  <span
                    v-if="(plan.commands || []).length > 6"
                    class="px-1.5 py-0.5 rounded border border-border text-[11px] text-text-muted"
                  >
                    +{{ (plan.commands || []).length - 6 }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- 设备卡片网格 -->
          <div
            class="flex-1 overflow-auto scrollbar-custom min-h-0 relative"
            ref="devicesContainer"
          >
            <div
              v-if="
                deviceCardUnits.length === 0 && (isRunning || awaitingSnapshot)
              "
              class="flex flex-col items-center justify-center h-48 text-text-muted gap-3"
            >
              <div
                class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"
              ></div>
              <p class="text-sm">正在初始化任务...</p>
            </div>
            <div v-else>
              <div
                v-if="deviceCardUnits.length > 50"
                class="text-xs text-text-muted mb-2 px-1"
              >
                共 {{ deviceCardUnits.length }} 台设备，显示前
                {{ visibleUnitCount }} 台活跃设备
                <button
                  v-if="deviceCardUnits.length > visibleUnitCount"
                  @click="showAllDevices = !showAllDevices"
                  class="ml-2 text-accent hover:underline"
                >
                  {{ showAllDevices ? "收起" : "显示全部" }}
                </button>
              </div>

              <div class="grid grid-cols-3 gap-4">
                <div
                  v-for="unit in visibleUnits"
                  :key="unit.id"
                  class="bg-bg-card border rounded-xl overflow-hidden shadow-card transition-all duration-300"
                  :class="statusBorder(unit.status)"
                >
                  <div
                    class="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-panel"
                  >
                    <span
                      class="font-mono text-sm font-semibold text-text-primary"
                      >{{ unit.targetKey }}</span
                    >
                    <span
                      class="flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full border"
                      :class="statusBadge(unit.status)"
                    >
                      <span
                        class="w-1.5 h-1.5 rounded-full"
                        :class="statusDot(unit.status)"
                      ></span>
                      {{ statusLabel(unit.status) }}
                    </span>
                  </div>
                  <div
                    class="px-4 py-2 border-b border-border bg-bg-card/50 space-y-1"
                  >
                    <div
                      class="flex items-center justify-between gap-3 text-xs"
                    >
                      <span class="text-text-muted">步骤进度</span>
                      <span class="font-mono text-text-primary"
                        >{{ unit.doneSteps }}/{{ unit.totalSteps }}</span
                      >
                    </div>
                    <div
                      v-if="unit.errorMessage"
                      class="text-xs text-error break-all"
                    >
                      {{ unit.errorMessage }}
                    </div>
                  </div>
                  <VirtualLogTerminal
                    :logs="unit.logs || []"
                    :total-count="unit.logCount || 0"
                    :truncated="unit.truncated || false"
                    :device-ip="unit.targetKey"
                  />
                </div>
              </div>

              <div
                v-if="
                  !showAllDevices && deviceCardUnits.length > visibleUnitCount
                "
                class="text-center py-4 text-text-muted text-sm"
              >
                还有
                {{ deviceCardUnits.length - visibleUnitCount }}
                台设备已完成或等待中
                <button
                  @click="showAllDevices = true"
                  class="ml-2 text-accent hover:underline"
                >
                  显示全部
                </button>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- 任务列表视图 -->
      <template v-else>
        <div class="flex-1 overflow-auto scrollbar-custom min-h-0 space-y-4">
          <div v-if="loading" class="flex items-center justify-center h-48">
            <div
              class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"
            ></div>
          </div>
          <div
            v-else-if="loadError"
            class="flex flex-col items-center justify-center h-48 text-text-muted gap-3"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-12 h-12 opacity-30"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <circle cx="12" cy="12" r="9" />
              <line x1="12" y1="8" x2="12" y2="13" />
              <circle cx="12" cy="16" r="0.5" fill="currentColor" />
            </svg>
            <p class="text-sm text-error">任务列表加载失败</p>
            <p class="text-xs max-w-md text-center break-all">
              {{ loadError }}
            </p>
            <button
              @click="loadTasks('retry')"
              class="px-3 py-1.5 rounded-lg text-xs font-medium bg-bg-card border border-border text-text-primary hover:border-accent/50 transition-all"
            >
              重试加载
            </button>
          </div>
          <div
            v-else-if="filteredTasks.length === 0"
            class="flex flex-col items-center justify-center h-48 text-text-muted gap-3"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-12 h-12 opacity-30"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <line x1="9" y1="9" x2="15" y2="15" />
              <line x1="15" y1="9" x2="9" y2="15" />
            </svg>
            <p class="text-sm">暂无任务，请前往「任务创建」页面创建</p>
          </div>
          <div v-else class="grid grid-cols-2 gap-4">
            <div
              v-for="task in filteredTasks"
              :key="task.id"
              class="bg-bg-card border border-border rounded-xl overflow-hidden shadow-card hover:border-accent/30 transition-all duration-300 group/card"
            >
              <!-- 卡片头部 -->
              <div
                class="flex items-start justify-between px-4 py-3 border-b border-border bg-bg-panel"
              >
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <h3
                      class="text-sm font-semibold text-text-primary truncate"
                    >
                      {{ task.name }}
                    </h3>
                    <span
                      class="flex-shrink-0 text-xs px-2 py-0.5 rounded-full border font-medium"
                      :class="
                        isTopologyTask(task)
                          ? 'bg-accent/10 border-accent/30 text-accent'
                          : 'bg-bg-panel border-border text-text-muted'
                      "
                    >
                      {{ isTopologyTask(task) ? "拓扑采集" : "普通任务" }}
                    </span>
                    <span
                      class="flex-shrink-0 text-xs px-2 py-0.5 rounded-full border font-medium"
                      :class="
                        task.mode === 'group'
                          ? 'bg-info/10 border-info/30 text-info'
                          : 'bg-warning/10 border-warning/30 text-warning'
                      "
                      >{{ task.mode === "group" ? "模式A" : "模式B" }}</span
                    >
                    <span
                      class="flex-shrink-0 flex items-center gap-1 text-xs px-2 py-0.5 rounded-full border font-medium"
                      :class="
                        taskStatusBadge(task.latestRunStatus || task.status)
                      "
                    >
                      <span
                        class="w-1.5 h-1.5 rounded-full"
                        :class="
                          taskStatusDot(task.latestRunStatus || task.status)
                        "
                      ></span>
                      {{ taskStatusLabel(task.latestRunStatus || task.status) }}
                    </span>
                  </div>
                  <p class="text-xs text-text-muted line-clamp-1 mt-1">
                    {{ task.description || "暂无描述" }}
                  </p>
                </div>
                <!-- 操作按钮 -->
                <div class="flex items-center gap-1 ml-2">
                  <button
                    @click="openTaskDetail(task)"
                    class="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10 transition-colors"
                    title="查看详情"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-4 h-4"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                      <circle cx="12" cy="12" r="3" />
                    </svg>
                  </button>
                  <button
                    @click="openTaskEdit(task)"
                    :disabled="!task.canEdit"
                    class="p-1.5 rounded-md text-text-muted transition-colors"
                    :class="
                      task.canEdit
                        ? 'hover:text-warning hover:bg-warning/10'
                        : 'opacity-40 cursor-not-allowed'
                    "
                    :title="
                      !task.canEdit
                        ? '任务存在活跃运行，不可编辑'
                        : isTopologyTask(task)
                          ? '编辑拓扑任务（支持字段级覆盖）'
                          : '编辑任务'
                    "
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-4 h-4"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path
                        d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"
                      />
                      <path
                        d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"
                      />
                    </svg>
                  </button>
                  <button
                    @click="showExecutionHistory(task)"
                    class="p-1.5 rounded-md text-text-muted hover:text-info hover:bg-info/10 transition-colors"
                    title="查看执行历史"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-4 h-4"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path d="M3 3v5h5" />
                      <path d="M3.05 13A9 9 0 1 0 6 5.3L3 8" />
                      <path d="M12 7v5l4 2" />
                    </svg>
                  </button>
                  <button
                    @click="executeTask(task)"
                    :disabled="isRunning || awaitingSnapshot"
                    class="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10 transition-colors"
                    :class="
                      isRunning || awaitingSnapshot
                        ? 'opacity-50 cursor-not-allowed'
                        : ''
                    "
                    title="执行"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-4 h-4"
                      viewBox="0 0 24 24"
                      fill="currentColor"
                    >
                      <polygon points="5 3 19 12 5 21 5 3" />
                    </svg>
                  </button>
                  <button
                    @click="confirmDelete(task)"
                    class="p-1.5 rounded-md text-text-muted hover:text-error hover:bg-error/10 transition-colors"
                    title="删除"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-4 h-4"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <polyline points="3 6 5 6 21 6" />
                      <path
                        d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
                      />
                    </svg>
                  </button>
                </div>
              </div>

              <!-- 绑定详情 -->
              <div class="px-4 py-3">
                <template v-if="isTopologyTask(task)">
                  <div class="flex items-center gap-2 text-xs">
                    <span
                      class="px-2 py-0.5 rounded bg-accent/10 border border-accent/20 text-accent font-mono"
                    >
                      厂商: {{ topologyVendorLabel(task) }}
                    </span>
                    <span class="text-text-secondary">
                      {{ topologyDeviceCount(task) }} 台设备
                    </span>
                  </div>
                </template>
                <template v-else-if="task.mode === 'group'">
                  <div
                    v-for="(item, idx) in task.items || []"
                    :key="idx"
                    class="flex items-center gap-2 text-xs"
                  >
                    <span
                      class="px-2 py-0.5 rounded bg-accent/10 border border-accent/20 text-accent font-mono truncate max-w-[200px]"
                    >
                      命令组:
                      {{
                        String(item.commandGroupId || "-").substring(0, 8)
                      }}...
                    </span>
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3.5 h-3.5 text-text-muted flex-shrink-0"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <line x1="5" y1="12" x2="19" y2="12" />
                      <polyline points="12 5 19 12 12 19" />
                    </svg>
                    <span class="text-text-secondary"
                      >{{
                        Array.isArray(item.deviceIDs)
                          ? item.deviceIDs.length
                          : 0
                      }}
                      台设备</span
                    >
                  </div>
                </template>
                <template v-else>
                  <div class="space-y-1">
                    <div
                      v-for="(item, idx) in (task.items || []).slice(0, 3)"
                      :key="idx"
                      class="flex items-center gap-2 text-xs"
                    >
                      <span class="font-mono text-text-secondary">{{
                        Array.isArray(item.deviceIDs) &&
                        item.deviceIDs.length > 0
                          ? item.deviceIDs[0]
                          : "-"
                      }}</span>
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        class="w-3 h-3 text-text-muted flex-shrink-0"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <line x1="5" y1="12" x2="19" y2="12" />
                        <polyline points="12 5 19 12 12 19" />
                      </svg>
                      <span class="text-text-muted truncate"
                        >{{
                          Array.isArray(item.commands)
                            ? item.commands.length
                            : 0
                        }}
                        条命令</span
                      >
                    </div>
                    <div
                      v-if="(task.items || []).length > 3"
                      class="text-xs text-text-muted"
                    >
                      +{{ (task.items || []).length - 3 }} 台设备...
                    </div>
                  </div>
                </template>
              </div>

              <!-- 标签和时间 -->
              <div
                class="px-4 py-2 border-t border-border bg-bg-secondary/30 text-xs text-text-muted flex items-center justify-between"
              >
                <div class="flex items-center gap-1.5 overflow-hidden">
                  <span
                    v-for="tag in (task.tags || []).slice(0, 3)"
                    :key="tag"
                    class="px-1.5 py-0.5 rounded bg-accent/10 border border-accent/20 text-accent truncate"
                    >{{ tag }}</span
                  >
                </div>
                <span class="flex-shrink-0 ml-2">{{
                  formatDate(task.updatedAt)
                }}</span>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- 删除确认弹窗 -->
    <Transition name="modal">
      <div
        v-if="deleteModal.show"
        class="fixed inset-0 z-50 flex items-center justify-center"
      >
        <div
          class="absolute inset-0 bg-black/60 backdrop-blur-sm"
          @click="deleteModal.show = false"
        ></div>
        <div
          class="relative bg-bg-card border border-error/30 rounded-xl shadow-2xl max-w-sm w-full mx-4 overflow-hidden animate-slide-in"
        >
          <div class="px-5 py-4 space-y-3">
            <h3 class="text-sm font-semibold text-text-primary">确认删除</h3>
            <p class="text-xs text-text-muted">
              确定要删除任务「{{ deleteModal.taskName }}」吗？此操作不可撤销。
            </p>
          </div>
          <div class="flex justify-end gap-3 px-5 py-3 border-t border-border">
            <button
              @click="deleteModal.show = false"
              class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-panel border border-border text-text-secondary hover:text-text-primary transition-all"
            >
              取消
            </button>
            <button
              @click="doDelete"
              class="px-4 py-2 rounded-lg text-sm font-semibold bg-error hover:bg-error/80 text-white transition-all"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Toast 通知 -->
    <Transition name="toast">
      <div
        v-if="showToast"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50"
      >
        <div
          class="flex items-center gap-2 px-5 py-3 rounded-xl shadow-2xl border"
          :class="
            toastType === 'success'
              ? 'bg-success/10 border-success/30 text-success'
              : 'bg-error/10 border-error/30 text-error'
          "
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
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import {
  CommandGroupAPI,
  DeviceAPI,
  TaskExecutionAPI,
  TaskGroupAPI,
} from "../services/api";
import type {
  CommandGroup,
  DeviceAsset,
  TaskGroup,
  TaskGroupDetailViewModel,
  TaskGroupListView,
  TopologyCollectionPlanArtifact,
} from "../services/api";
import { useTaskexecStore } from "../stores/taskexecStore";
import VirtualLogTerminal from "../components/task/VirtualLogTerminal.vue";
import ExecutionHistoryDrawer from "../components/task/ExecutionHistoryDrawer.vue";
import TaskDetailModal from "../components/task/TaskDetailModal.vue";
import TaskEditModal from "../components/task/TaskEditModal.vue";
import StageProgress from "../components/task/StageProgress.vue";
import type { StageSnapshot, UnitSnapshot } from "../types/taskexec";

const router = useRouter();
const route = useRoute();
const taskexecStore = useTaskexecStore();

// ================== 任务执行状态 ==================
const loading = ref(false);
const loadError = ref("");
const tasks = ref<TaskGroupListView[]>([]);
const searchQuery = ref("");
const filterStatus = ref("");
const filterMode = ref("");

// 执行视图状态 (阶段3: 统一执行框架 - 使用runId驱动)
const executionView = ref({
  active: false,
  taskId: 0 as number,
  runId: "" as string, // 统一运行时runId
  taskName: "",
  taskType: "normal" as "normal" | "topology",
});
const awaitingSnapshot = ref(false);
let snapshotTimeoutTimer: ReturnType<typeof setTimeout> | null = null;
const SNAPSHOT_TIMEOUT = 10000;
let snapshotPollTimer: ReturnType<typeof setInterval> | null = null;
const SNAPSHOT_POLL_INTERVAL = 1000;

// 删除弹窗
const deleteModal = ref({ show: false, taskId: 0 as number, taskName: "" });

// 执行历史抽屉
const historyDrawer = ref({
  show: false,
  taskGroupId: "",
  taskGroupName: "",
});

// 任务详情弹窗
const detailModal = ref({
  show: false,
  loading: false,
  detail: null as TaskGroupDetailViewModel | null,
});

// 任务编辑弹窗
const editModal = ref({
  show: false,
  loading: false,
  saving: false,
  task: null as TaskGroup | null,
});

const editReferences = ref({
  devices: [] as DeviceAsset[],
  commandGroups: [] as CommandGroup[],
});

const topologyCollectionPlanRows = ref<TopologyCollectionPlanArtifact[]>([]);
const topologyPlanLoading = ref(false);
const topologyPlanError = ref("");
const topologyPlanLastRevision = ref(-1);

// 虚拟滚动优化状态
const showAllDevices = ref(false);
const VISIBLE_DEVICE_LIMIT = 30;

// Toast
const showToast = ref(false);
const toastMessage = ref("");
const toastType = ref<"success" | "error">("success");
let toastTimer: ReturnType<typeof setTimeout> | null = null;

function triggerToast(msg: string, type: "success" | "error" = "success") {
  toastMessage.value = msg;
  toastType.value = type;
  showToast.value = true;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    showToast.value = false;
  }, 3000);
}

// ================== 计算属性 - 从 Store 获取 ==================
const executionSnapshot = computed(() => taskexecStore.currentSnapshot);
const isRunning = computed(() => taskexecStore.isRunning);
const shouldShowExecutionView = computed(() => {
  return (
    executionView.value.active &&
    (awaitingSnapshot.value || isRunning.value || !!executionSnapshot.value)
  );
});
const progressPercent = computed(() => {
  const snapshot = executionSnapshot.value;
  if (!snapshot) {
    return 0;
  }

  const runProgress = snapshot.progress ?? 0;
  if (Array.isArray(snapshot.stages) && snapshot.stages.length > 0) {
    const total = snapshot.stages.reduce(
      (sum, stage) => sum + (stage.progress || 0),
      0,
    );
    const stageProgress = Math.round(total / snapshot.stages.length);
    return Math.max(runProgress, stageProgress);
  }

  return runProgress;
});
// ================== 统一运行时 Stage/Unit 数据 (新增) ==================
const executionStages = computed<StageSnapshot[]>(() => {
  return (executionSnapshot.value as any)?.stages || [];
});

const executionUnits = computed<UnitSnapshot[]>(() => {
  return (executionSnapshot.value as any)?.units || [];
});

const executionRunStatus = computed(() => {
  const snapshot = executionSnapshot.value;
  if (snapshot?.status) {
    return snapshot.status;
  }
  if (awaitingSnapshot.value) {
    return "pending";
  }
  return isRunning.value ? "running" : "pending";
});

function isDeviceExecutionUnit(unit: UnitSnapshot): boolean {
  return (
    normalizeString(unit.kind) === "device" &&
    normalizeString(unit.targetType) === "device_ip" &&
    normalizeString(unit.targetKey) !== ""
  );
}

function deviceUnitPriority(
  unit: UnitSnapshot,
  stageOrderMap: Map<string, number>,
): number {
  const statusPriority: Record<string, number> = {
    running: 700,
    failed: 600,
    partial: 550,
    cancelled: 500,
    pending: 300,
    completed: 200,
  };
  const stageOrder = stageOrderMap.get(unit.stageId) ?? 0;
  const progress = Number(unit.progress || 0);
  const doneSteps = Number(unit.doneSteps || 0);
  return (
    (statusPriority[unit.status] || 0) * 1000000 +
    stageOrder * 10000 +
    progress * 100 +
    doneSteps
  );
}

const deviceCardUnits = computed<UnitSnapshot[]>(() => {
  const stageOrderMap = new Map<string, number>(
    executionStages.value.map((stage) => [stage.id, stage.order]),
  );
  const groups = new Map<string, UnitSnapshot[]>();

  for (const unit of executionUnits.value) {
    if (!isDeviceExecutionUnit(unit)) {
      continue;
    }
    const key = normalizeString(unit.targetKey);
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key)!.push(unit);
  }

  const projected = Array.from(groups.values())
    .map((group) => {
      const sorted = [...group].sort((left, right) => {
        const priorityDiff =
          deviceUnitPriority(right, stageOrderMap) -
          deviceUnitPriority(left, stageOrderMap);
        if (priorityDiff !== 0) {
          return priorityDiff;
        }
        return normalizeString(left.id).localeCompare(
          normalizeString(right.id),
        );
      });
      return sorted.length > 0 ? sorted[0] : null;
    })
    .filter((unit): unit is UnitSnapshot => unit !== null);

  return projected.sort((left, right) => {
    const priorityDiff =
      deviceUnitPriority(right, stageOrderMap) -
      deviceUnitPriority(left, stageOrderMap);
    if (priorityDiff !== 0) {
      return priorityDiff;
    }
    return normalizeString(left.targetKey).localeCompare(
      normalizeString(right.targetKey),
    );
  });
});

const failedExecutionUnitCount = computed(() => {
  return deviceCardUnits.value.filter((unit) =>
    ["failed", "partial", "cancelled"].includes(unit.status),
  ).length;
});

const firstExecutionError = computed(() => {
  return (
    deviceCardUnits.value.find((unit) => normalizeString(unit.errorMessage))
      ?.errorMessage ??
    executionUnits.value.find((unit) => normalizeString(unit.errorMessage))
      ?.errorMessage ??
    ""
  );
});

const isExecutionTerminal = computed(() => {
  return ["completed", "partial", "failed", "cancelled", "aborted"].includes(
    executionRunStatus.value,
  );
});

const executionStatusSummary = computed(() => {
  const isTopology = executionView.value.taskType === "topology";
  switch (executionRunStatus.value) {
    case "pending":
      return isTopology
        ? "拓扑采集任务正在初始化，等待首个快照返回。"
        : "任务正在初始化，等待首个快照返回。";
    case "running":
      return isTopology
        ? "拓扑采集正在执行，页面将实时刷新采集、解析与构建进度。"
        : "任务正在执行，页面将实时刷新设备日志与阶段进度。";
    case "completed":
      return isTopology
        ? "拓扑采集已完成，所有阶段执行成功。"
        : "任务执行完成，所有设备均已成功结束。";
    case "partial":
      return isTopology
        ? "拓扑采集部分完成，存在失败设备或失败阶段。"
        : "任务部分完成，存在失败设备或未完全成功的单元。";
    case "failed":
      return isTopology
        ? "拓扑采集失败，未能完成必要执行阶段。"
        : "任务执行失败。";
    case "aborted":
      return isTopology
        ? "拓扑采集已中止，关键阶段失败导致后续阶段跳过。"
        : "任务已中止。";
    case "cancelled":
      return isTopology ? "拓扑采集已取消。" : "任务已取消。";
    default:
      return isTopology ? "拓扑采集状态未知。" : "任务状态未知。";
  }
});

const executionStatusDetail = computed(() => {
  if (
    executionRunStatus.value === "running" ||
    executionRunStatus.value === "pending"
  ) {
    return executionView.value.taskType === "topology"
      ? "执行链路：设备采集 → 数据解析 → 拓扑构建。任一设备失败都会在下方设备卡片中显示。"
      : "执行链路会按快照持续更新，下方设备卡片与阶段条目会实时反映最新状态。";
  }

  const failedCount = failedExecutionUnitCount.value;
  const firstError = normalizeString(firstExecutionError.value);
  switch (executionRunStatus.value) {
    case "completed":
      return executionView.value.taskType === "topology"
        ? "可以直接前往拓扑图谱页面查看本次采集结果。"
        : "可以返回任务列表查看执行结果。";
    case "partial":
      return failedCount > 0
        ? `共有 ${failedCount} 个执行单元未成功完成，请优先检查失败设备的错误信息。`
        : "部分阶段未完全成功，请检查阶段与设备详情。";
    case "failed":
    case "aborted":
      return firstError || "请检查失败设备的日志和错误原因。";
    case "cancelled":
      return "任务已停止，未完成的阶段不会继续执行。";
    default:
      return "";
  }
});

const executionStatusDetailClass = computed(() => {
  switch (executionRunStatus.value) {
    case "partial":
      return "text-warning";
    case "failed":
    case "aborted":
      return "text-error";
    case "completed":
      return "text-success";
    default:
      return "text-text-muted";
  }
});

const topologyPlanEnabledCount = computed(() =>
  topologyCollectionPlanRows.value.reduce(
    (sum, plan) => sum + enabledCommandCount(plan),
    0,
  ),
);

const topologyPlanDisabledCount = computed(() =>
  topologyCollectionPlanRows.value.reduce(
    (sum, plan) => sum + disabledCommandCount(plan),
    0,
  ),
);

// ================== 虚拟滚动优化计算属性 ==================
const visibleUnitCount = computed(() =>
  showAllDevices.value ? deviceCardUnits.value.length : VISIBLE_DEVICE_LIMIT,
);

const visibleUnits = computed(() => {
  const units = deviceCardUnits.value;

  if (showAllDevices.value) {
    return units;
  }

  const activeStatuses = ["running", "failed", "partial", "pending"];
  const active = units.filter((unit: UnitSnapshot) =>
    activeStatuses.includes(unit.status),
  );
  const inactive = units.filter(
    (unit: UnitSnapshot) => !activeStatuses.includes(unit.status),
  );

  if (active.length >= VISIBLE_DEVICE_LIMIT) {
    return active.slice(0, VISIBLE_DEVICE_LIMIT);
  }

  return [...active, ...inactive].slice(0, VISIBLE_DEVICE_LIMIT);
});

// ================== 生命周期 ==================
onMounted(() => {
  void syncExecutionView();
  void taskexecStore.loadRunHistory(50);
  void loadTasks("mounted");
});

onUnmounted(() => {
  clearSnapshotTimeout();
  stopSnapshotPolling();
  if (toastTimer) {
    clearTimeout(toastTimer);
    toastTimer = null;
  }
});

watch(isRunning, (running, wasRunning) => {
  if (!running && wasRunning && executionView.value.active) {
    stopSnapshotPolling();
    void taskexecStore.loadRunHistory(50);
    void loadTasks("run-finished");
  }
});

watch(
  () => route.query.refresh,
  (refreshToken, previousToken) => {
    if (!refreshToken || refreshToken === previousToken) {
      return;
    }
    console.debug(
      "[TaskExecution] 检测到路由刷新信号，重新加载任务列表",
      refreshToken,
    );
    void loadTasks("route-refresh");
  },
);

function normalizeString(value: unknown, fallback: string = ""): string {
  return typeof value === "string" ? value.trim() : fallback;
}

function normalizeStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeNumberArray(value: unknown): number[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item) && item > 0);
}

function normalizeTaskItem(item: any) {
  return {
    commandGroupId: normalizeString(item?.commandGroupId),
    commands: normalizeStringArray(item?.commands),
    deviceIDs: normalizeNumberArray(item?.deviceIDs),
  };
}

function normalizeTaskListEntry(task: any): TaskGroupListView {
  const normalized = {
    ...(task || {}),
    id: Number(task?.id || 0),
    name: normalizeString(task?.name, "未命名任务"),
    description: normalizeString(task?.description, "暂无描述"),
    deviceGroup: normalizeString(task?.deviceGroup),
    commandGroup: normalizeString(task?.commandGroup),
    taskType: normalizeString(task?.taskType, "normal"),
    topologyVendor: normalizeString(task?.topologyVendor),
    mode: normalizeString(task?.mode, "group"),
    status: normalizeString(task?.status, "pending"),
    latestRunId: normalizeString(task?.latestRunId),
    latestRunStatus: normalizeString(task?.latestRunStatus),
    latestRunStartedAt: normalizeString(task?.latestRunStartedAt),
    latestRunFinishedAt: normalizeString(task?.latestRunFinishedAt),
    activeRunCount: Number(task?.activeRunCount || 0),
    canEdit: task?.canEdit !== false,
    tags: normalizeStringArray(task?.tags),
    items: Array.isArray(task?.items)
      ? task.items.map((item: any) => normalizeTaskItem(item))
      : [],
    createdAt: normalizeString(task?.createdAt),
    updatedAt: normalizeString(task?.updatedAt || task?.createdAt),
    enableRawLog: Boolean(task?.enableRawLog),
  };
  return normalized as TaskGroupListView;
}

function matchesTaskQuery(task: TaskGroupListView, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) {
    return true;
  }
  const name = normalizeString(task.name).toLowerCase();
  const description = normalizeString(task.description).toLowerCase();
  const tags = normalizeStringArray(task.tags);
  return (
    name.includes(q) ||
    description.includes(q) ||
    tags.some((tag) => tag.toLowerCase().includes(q))
  );
}

// ================== 过滤逻辑（任务列表） ==================
const filteredTasks = computed(() => {
  let result = tasks.value.filter((task) => task && task.id > 0);
  if (searchQuery.value) {
    result = result.filter((task) => matchesTaskQuery(task, searchQuery.value));
  }
  if (filterStatus.value) {
    result = result.filter(
      (task) => normalizeString(task.status, "pending") === filterStatus.value,
    );
  }
  if (filterMode.value) {
    result = result.filter(
      (task) => normalizeString(task.mode, "group") === filterMode.value,
    );
  }
  return result;
});

function refreshTaskList() {
  void loadTasks("manual-refresh");
}

function resetExecutionViewState(reason: string) {
  console.debug(`[TaskExecution] 重置执行视图状态，reason=${reason}`, {
    active: executionView.value.active,
    runId: executionView.value.runId,
    currentRunId: taskexecStore.currentRunId,
    awaitingSnapshot: awaitingSnapshot.value,
  });
  executionView.value.active = false;
  executionView.value.runId = "";
  executionView.value.taskName = "";
  executionView.value.taskType = "normal";
  awaitingSnapshot.value = false;
  clearSnapshotTimeout();
  stopSnapshotPolling();
  taskexecStore.setCurrentRunId(null);
  topologyCollectionPlanRows.value = [];
  topologyPlanError.value = "";
  topologyPlanLoading.value = false;
  topologyPlanLastRevision.value = -1;
}

// 加载任务列表
async function loadTasks(reason: string = "manual") {
  loading.value = true;
  loadError.value = "";
  try {
    console.debug(`[TaskExecution] 开始加载任务列表，reason=${reason}`);
    const result = await TaskGroupAPI.listTaskGroups();
    if (!Array.isArray(result)) {
      throw new Error("任务列表接口返回非数组数据");
    }
    tasks.value = result
      .filter(Boolean)
      .map((item) => normalizeTaskListEntry(item));
    console.debug(
      `[TaskExecution] 任务列表加载完成，count=${tasks.value.length}, reason=${reason}`,
    );
  } catch (err: any) {
    const message = err?.message || String(err);
    console.error("[TaskExecution] 加载任务列表失败:", err);
    loadError.value = message;
    tasks.value = [];
  } finally {
    loading.value = false;
  }
}

async function syncExecutionView() {
  try {
    const running = await TaskExecutionAPI.listRunningTasks();
    console.debug(`[TaskExecution] 同步执行视图: running=${running.length}`);

    if (!running.length) {
      // 没有运行中的任务，检查当前任务是否已完成
      const currentRunId =
        executionView.value.runId || taskexecStore.currentRunId;

      if (currentRunId) {
        // 主动刷新当前 runId 的快照，确认是否终态
        const snapshot = await taskexecStore.refreshSnapshot(currentRunId);
        if (snapshot) {
          // 检查刷新后的快照是否终态
          const terminalStatuses = [
            "completed",
            "partial",
            "failed",
            "cancelled",
            "aborted",
          ];
          if (terminalStatuses.includes(snapshot.status)) {
            // 任务已完成（终态），重置执行视图返回任务列表
            // 用户离开页面后返回，应该看到任务列表而非已完成的执行详情
            console.debug(
              `[TaskExecution] 任务已完成，重置执行视图，status=${snapshot.status}`,
            );
            resetExecutionViewState("task-completed-on-mount");
            return;
          }
        }
      }

      // 没有当前 runId 或快照刷新失败或非终态，重置执行视图
      resetExecutionViewState("no-running-snapshots");
      return;
    }

    for (const snapshot of running) {
      taskexecStore.updateSnapshot(snapshot.runId, snapshot);
    }

    const currentRunId =
      taskexecStore.currentRunId &&
      running.some((item) => item.runId === taskexecStore.currentRunId)
        ? taskexecStore.currentRunId
        : running[0].runId;
    const snapshot =
      running.find((item) => item.runId === currentRunId) ?? running[0];

    taskexecStore.setCurrentRunId(snapshot.runId);
    executionView.value.active = true;
    executionView.value.runId = snapshot.runId;
    executionView.value.taskName = snapshot.taskName || "任务执行";
    executionView.value.taskType =
      snapshot.runKind === "topology" ? "topology" : "normal";
    console.debug("[TaskExecution] 已切换到执行视图", {
      runId: snapshot.runId,
      taskName: snapshot.taskName,
      runKind: snapshot.runKind,
    });
  } catch (err) {
    console.error("[TaskExecution] 同步执行视图失败:", err);
    resetExecutionViewState("sync-error");
  }
}

function startSnapshotPolling() {
  if (snapshotPollTimer) return;
  snapshotPollTimer = setInterval(() => {
    if (!executionView.value.active) {
      stopSnapshotPolling();
      return;
    }
    // 任务完成后停止轮询（终态不再变化）
    if (!awaitingSnapshot.value && isExecutionTerminal.value) {
      stopSnapshotPolling();
      return;
    }
    if (!awaitingSnapshot.value && !isRunning.value) {
      stopSnapshotPolling();
      return;
    }
    void syncExecutionView();
  }, SNAPSHOT_POLL_INTERVAL);
}

function stopSnapshotPolling() {
  if (snapshotPollTimer) {
    clearInterval(snapshotPollTimer);
    snapshotPollTimer = null;
  }
}

async function handleSnapshotTimeout() {
  if (!awaitingSnapshot.value) return;

  const runId = executionView.value.runId || taskexecStore.currentRunId || "";
  const snapshot = runId ? await taskexecStore.refreshSnapshot(runId) : null;
  if (snapshot) {
    awaitingSnapshot.value = false;
    clearSnapshotTimeout();
    executionView.value.active = true;
    executionView.value.runId = snapshot.runId;
    executionView.value.taskName =
      snapshot?.taskName || executionView.value.taskName || "任务执行";
    startSnapshotPolling();
    return;
  }

  console.warn("[TaskExecution] 快照超时，重置UI状态");
  awaitingSnapshot.value = false;
  triggerToast("任务执行超时，请检查设备连接配置", "error");
  executionView.value.active = false;
  stopSnapshotPolling();
}

function startSnapshotTimeout() {
  if (snapshotTimeoutTimer) {
    clearTimeout(snapshotTimeoutTimer);
  }
  snapshotTimeoutTimer = setTimeout(() => {
    void handleSnapshotTimeout();
  }, SNAPSHOT_TIMEOUT);
}

function clearSnapshotTimeout() {
  if (snapshotTimeoutTimer) {
    clearTimeout(snapshotTimeoutTimer);
    snapshotTimeoutTimer = null;
  }
}

// 执行任务 (阶段3: 统一执行框架 - 统一使用runId驱动)
async function executeTask(task: TaskGroupListView) {
  if (isRunning.value || awaitingSnapshot.value) return;

  executionView.value = {
    active: true,
    taskId: task.id,
    runId: "",
    taskName: task.name,
    taskType: isTopologyTask(task) ? "topology" : "normal",
  };

  taskexecStore.clearEventLogs();
  taskexecStore.setCurrentRunId(null);
  awaitingSnapshot.value = true;
  topologyCollectionPlanRows.value = [];
  topologyPlanError.value = "";
  topologyPlanLastRevision.value = -1;

  startSnapshotTimeout();
  startSnapshotPolling();

  try {
    const runId = await TaskGroupAPI.startTaskGroup(task.id);
    executionView.value.runId = runId;
    taskexecStore.setCurrentRunId(runId);
    await taskexecStore.refreshSnapshot(runId);
    await taskexecStore.loadRunHistory(50);
    await loadTasks("task-started");
  } catch (err: any) {
    console.error("执行任务失败:", err);
    triggerToast(`执行失败: ${err?.message || err}`, "error");
    executionView.value.active = false;
    clearSnapshotTimeout();
    stopSnapshotPolling();
  }
}

async function loadTopologyCollectionPlans(runId: string) {
  const normalizedRunID = normalizeString(runId);
  if (executionView.value.taskType !== "topology" || normalizedRunID === "") {
    topologyCollectionPlanRows.value = [];
    topologyPlanError.value = "";
    topologyPlanLoading.value = false;
    return;
  }

  topologyPlanLoading.value = true;
  topologyPlanError.value = "";
  try {
    const plans =
      await TaskExecutionAPI.getTopologyCollectionPlans(normalizedRunID);
    topologyCollectionPlanRows.value = Array.isArray(plans)
      ? plans.filter(Boolean)
      : [];
  } catch (err: any) {
    topologyPlanError.value = err?.message || String(err);
    topologyCollectionPlanRows.value = [];
  } finally {
    topologyPlanLoading.value = false;
  }
}

watch(
  () => executionView.value.runId,
  (runId, previousRunId) => {
    if (runId === previousRunId) {
      return;
    }
    topologyPlanLastRevision.value = -1;
    if (executionView.value.taskType === "topology" && normalizeString(runId)) {
      void loadTopologyCollectionPlans(runId);
    }
  },
);

watch(
  () => executionView.value.taskType,
  (taskType) => {
    if (taskType !== "topology") {
      topologyCollectionPlanRows.value = [];
      topologyPlanError.value = "";
      topologyPlanLoading.value = false;
      topologyPlanLastRevision.value = -1;
      return;
    }
    if (normalizeString(executionView.value.runId)) {
      void loadTopologyCollectionPlans(executionView.value.runId);
    }
  },
);

// 监听快照变化
watch(executionSnapshot, (snapshot) => {
  if (snapshot) {
    awaitingSnapshot.value = false;
    clearSnapshotTimeout();
    executionView.value.active = true;
    executionView.value.runId = snapshot.runId;
    executionView.value.taskType =
      snapshot.runKind === "topology" ? "topology" : "normal";
    if (snapshot.taskName) {
      executionView.value.taskName = snapshot.taskName;
    }

    if (snapshot.runKind === "topology") {
      const revision = Number(snapshot.revision || 0);
      if (revision !== topologyPlanLastRevision.value) {
        topologyPlanLastRevision.value = revision;
        void loadTopologyCollectionPlans(snapshot.runId);
      }
    }
  }
});

// 删除任务
function confirmDelete(task: TaskGroupListView) {
  deleteModal.value = { show: true, taskId: task.id, taskName: task.name };
}

async function doDelete() {
  try {
    await TaskGroupAPI.deleteTaskGroup(deleteModal.value.taskId);
    deleteModal.value.show = false;
    triggerToast("任务已删除", "success");
    void loadTasks();
  } catch (err: any) {
    triggerToast(`删除失败: ${err?.message || err}`, "error");
  }
}

// 关闭执行视图：仅解绑当前 run，不删除快照缓存
function closeExecutionView() {
  if (isRunning.value || awaitingSnapshot.value) return;
  console.debug("[TaskExecution] 用户关闭执行视图");
  resetExecutionViewState("close-execution-view");
  void loadTasks("close-execution-view");
}

// 停止执行任务 (阶段3: 使用统一运行时的CancelTask)
async function stopExecution() {
  if (!confirm("确定要停止当前执行任务吗？")) {
    return;
  }

  if (executionView.value.runId) {
    try {
      await taskexecStore.cancelTask(executionView.value.runId);
      triggerToast("已发送停止信号");
      return;
    } catch (err: any) {
      triggerToast(`停止失败: ${err?.message || err}`, "error");
    }
  }
}

// 导航
function goToTaskCreate() {
  router.push("/tasks");
}

function isTopologyTask(task: TaskGroupListView | TaskGroup) {
  return ((task as any).taskType || "normal") === "topology";
}

function topologyVendorLabel(task: TaskGroupListView | TaskGroup) {
  const vendor = ((task as any).topologyVendor || "").trim();
  return vendor === "" ? "自动识别" : vendor;
}

function topologyDeviceCount(task: TaskGroupListView | TaskGroup) {
  const set = new Set<number>();
  for (const item of task.items || []) {
    for (const id of item.deviceIDs || []) {
      set.add(id);
    }
  }
  return set.size;
}

function enabledCommandCount(plan: TopologyCollectionPlanArtifact): number {
  if (!Array.isArray(plan?.commands)) {
    return 0;
  }
  return plan.commands.filter((cmd: any) => Boolean(cmd?.enabled)).length;
}

function disabledCommandCount(plan: TopologyCollectionPlanArtifact): number {
  if (!Array.isArray(plan?.commands)) {
    return 0;
  }
  return plan.commands.filter((cmd: any) => !Boolean(cmd?.enabled)).length;
}

function commandSourceLabel(source: string): string {
  const map: Record<string, string> = {
    task_override: "任务覆盖",
    vendor_config: "厂商配置",
    profile_seed: "画像种子",
    builtin_seed: "内置种子",
    disabled: "禁用",
  };
  const key = normalizeString(source);
  return map[key] ?? (key || "未知来源");
}

function vendorSourceLabel(source: string): string {
  const map: Record<string, string> = {
    task: "任务显式",
    inventory: "资产信息",
    detect: "自动探测",
    fallback_default: "默认回退",
  };
  const key = normalizeString(source);
  return map[key] ?? (key || "未知来源");
}

// 查看执行历史
function showExecutionHistory(task: TaskGroupListView) {
  historyDrawer.value = {
    show: true,
    taskGroupId: String(task.id),
    taskGroupName: task.name,
  };
}

async function ensureEditReferences() {
  if (
    editReferences.value.devices.length > 0 &&
    editReferences.value.commandGroups.length > 0
  ) {
    return;
  }

  const [devices, commandGroups] = await Promise.all([
    DeviceAPI.listDevices(),
    CommandGroupAPI.listCommandGroups(),
  ]);

  editReferences.value = {
    devices: devices || [],
    commandGroups: commandGroups || [],
  };
}

async function openTaskDetail(task: TaskGroupListView) {
  detailModal.value.show = true;
  detailModal.value.loading = true;
  detailModal.value.detail = null;

  try {
    detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(task.id);
  } catch (err: any) {
    detailModal.value.show = false;
    triggerToast(`加载任务详情失败: ${err?.message || err}`, "error");
  } finally {
    detailModal.value.loading = false;
  }
}

async function openTaskEdit(task: TaskGroupListView) {
  if (!task.canEdit) {
    triggerToast("任务存在活跃运行，不可编辑", "error");
    return;
  }

  editModal.value.show = true;
  editModal.value.loading = true;
  editModal.value.task = null;

  try {
    const [freshTask] = await Promise.all([
      TaskGroupAPI.getTaskGroup(task.id),
      ensureEditReferences(),
    ]);
    editModal.value.task = {
      ...(freshTask as any),
      status: task.latestRunStatus || task.status || "pending",
    };
  } catch (err: any) {
    editModal.value.show = false;
    triggerToast(`加载编辑数据失败: ${err?.message || err}`, "error");
  } finally {
    editModal.value.loading = false;
  }
}

async function editFromDetail() {
  const currentTask = detailModal.value.detail?.task;
  if (!currentTask) return;
  detailModal.value.show = false;
  await openTaskEdit({
    ...currentTask,
    status: detailModal.value.detail?.latestRunStatus || "pending",
    latestRunId: detailModal.value.detail?.latestRunId || "",
    latestRunStatus: detailModal.value.detail?.latestRunStatus || "pending",
    latestRunStartedAt: "",
    latestRunFinishedAt: "",
    activeRunCount: detailModal.value.detail?.activeRunCount || 0,
    canEdit: detailModal.value.detail?.canEdit ?? true,
  });
}

async function saveTaskEdit(payload: TaskGroup) {
  if (!editModal.value.task) return;

  editModal.value.saving = true;
  try {
    const updated = await TaskGroupAPI.updateTaskGroup(
      editModal.value.task.id,
      payload,
    );
    editModal.value.task = updated || payload;
    editModal.value.show = false;
    triggerToast("任务更新成功", "success");
    await loadTasks();

    if (
      detailModal.value.show &&
      detailModal.value.detail?.task.id === payload.id
    ) {
      detailModal.value.loading = true;
      detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(
        payload.id,
      );
      detailModal.value.loading = false;
    }
  } catch (err: any) {
    triggerToast(`保存失败: ${err?.message || err}`, "error");
    await loadTasks();

    if (
      detailModal.value.show &&
      detailModal.value.detail?.task.id === payload.id
    ) {
      try {
        detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(
          payload.id,
        );
      } catch (detailErr) {
        console.error("刷新任务详情失败:", detailErr);
      }
    }
  } finally {
    editModal.value.saving = false;
  }
}

// ================== 状态样式 ==================
function statusBorder(s: string) {
  switch (s) {
    case "running":
      return "border-accent/50";
    case "completed":
      return "border-success/50";
    case "failed":
    case "partial":
    case "cancelled":
      return "border-error/50";
    case "pending":
      return "border-warning/40";
    default:
      return "border-border";
  }
}
function statusBadge(s: string) {
  switch (s) {
    case "running":
      return "bg-accent/10 border-accent/30 text-accent";
    case "completed":
      return "bg-success/10 border-success/30 text-success";
    case "failed":
    case "partial":
    case "cancelled":
      return "bg-error/10 border-error/30 text-error";
    case "pending":
      return "bg-warning/10 border-warning/30 text-warning";
    default:
      return "bg-bg-panel border-border text-text-muted";
  }
}
function statusDot(s: string) {
  switch (s) {
    case "running":
      return "bg-accent animate-pulse";
    case "completed":
      return "bg-success";
    case "failed":
    case "partial":
    case "cancelled":
      return "bg-error";
    case "pending":
      return "bg-warning animate-pulse";
    default:
      return "bg-text-muted";
  }
}
function statusLabel(s: string) {
  const map: Record<string, string> = {
    pending: "等待中",
    running: "执行中",
    completed: "成功",
    partial: "部分完成",
    failed: "失败",
    cancelled: "已终止",
  };
  return map[s] ?? s;
}
// 任务状态样式
function taskStatusBadge(s: string) {
  switch (s) {
    case "pending":
      return "bg-bg-panel border-border text-text-muted";
    case "running":
      return "bg-accent/10 border-accent/30 text-accent";
    case "completed":
      return "bg-success/10 border-success/30 text-success";
    case "partial":
      return "bg-warning/10 border-warning/30 text-warning";
    case "failed":
    case "aborted":
      return "bg-error/10 border-error/30 text-error";
    case "cancelled":
      return "bg-bg-panel border-border text-text-muted";
    default:
      return "bg-bg-panel border-border text-text-muted";
  }
}
function taskStatusDot(s: string) {
  switch (s) {
    case "pending":
      return "bg-text-muted";
    case "running":
      return "bg-accent animate-pulse";
    case "completed":
      return "bg-success";
    case "partial":
      return "bg-warning";
    case "failed":
    case "aborted":
      return "bg-error";
    case "cancelled":
      return "bg-text-muted";
    default:
      return "bg-text-muted";
  }
}
function taskStatusLabel(s: string) {
  const map: Record<string, string> = {
    pending: "待执行",
    running: "执行中",
    completed: "已完成",
    partial: "部分成功",
    failed: "失败",
    cancelled: "已取消",
    aborted: "已中止",
  };
  return map[s] ?? s;
}

function formatDate(dateStr: string) {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
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
