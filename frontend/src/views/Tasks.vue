<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <p class="text-sm text-text-muted">
        选择设备和命令组，创建任务绑定到任务执行页
      </p>
      <div class="flex gap-3">
        <button
          @click="goToTaskExecution"
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
            <polygon points="5 3 19 12 5 21 5 3" />
          </svg>
          前往任务执行
        </button>
        <button
          @click="openCreateModal"
          :disabled="!canCreate"
          class="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold transition-all duration-200 shadow-card"
          :class="
            !canCreate
              ? 'bg-bg-card border border-border text-text-muted cursor-not-allowed'
              : 'bg-accent hover:bg-accent-glow text-white border border-accent/30 hover:shadow-glow'
          "
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
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          创建任务
        </button>
      </div>
    </div>

    <!-- 步骤选择区域 -->
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
      <div class="flex-1 overflow-y-auto scrollbar-custom pr-1">
        <!-- 任务类型 -->
        <div
          class="bg-bg-card border border-border rounded-xl overflow-hidden mb-3"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <span class="text-sm font-medium text-text-primary">任务类型</span>
          </div>
          <div class="p-3 flex gap-2">
            <button
              @click="selectedTaskType = 'normal'"
              class="px-3 py-1.5 rounded-lg text-sm border transition-all"
              :class="
                selectedTaskType === 'normal'
                  ? 'bg-accent text-white border-accent/40'
                  : 'bg-bg-panel border-border text-text-secondary hover:text-text-primary'
              "
            >
              普通任务
            </button>
            <button
              @click="selectedTaskType = 'topology'"
              class="px-3 py-1.5 rounded-lg text-sm border transition-all"
              :class="
                selectedTaskType === 'topology'
                  ? 'bg-accent text-white border-accent/40'
                  : 'bg-bg-panel border-border text-text-secondary hover:text-text-primary'
              "
            >
              拓扑采集任务
            </button>
            <button
              @click="selectedTaskType = 'backup'"
              class="px-3 py-1.5 rounded-lg text-sm border transition-all"
              :class="
                selectedTaskType === 'backup'
                  ? 'bg-accent text-white border-accent/40'
                  : 'bg-bg-panel border-border text-text-secondary hover:text-text-primary'
              "
            >
              配置备份任务
            </button>
          </div>
        </div>

        <!-- 步骤1: 选择目标设备 -->
        <div
          class="bg-bg-card border border-border rounded-xl overflow-hidden"
          :style="{ height: devicePanelHeight + 'px', minHeight: '200px' }"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <div
              class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent"
            >
              1
            </div>
            <span class="text-sm font-medium text-text-primary"
              >选择目标设备</span
            >
            <span
              v-if="selectedDevices.length > 0"
              class="ml-2 text-xs text-accent font-mono"
              >已选 {{ selectedDevices.length }} 台</span
            >
            <button
              @click="goToDevices"
              class="ml-auto text-xs text-accent hover:text-accent-glow transition-colors"
            >
              管理设备资产
            </button>
          </div>
          <div class="p-3 h-[calc(100%-45px)] overflow-y-auto scrollbar-custom">
            <button
              @click="showDeviceSelector = true"
              class="w-full px-4 py-2.5 rounded-lg text-sm font-medium bg-accent/10 border border-accent/30 text-accent hover:bg-accent hover:text-white transition-colors"
            >
              点击选择设备
            </button>
            <div v-if="selectedDevices.length > 0" class="mt-3 text-xs text-text-muted">
              已选设备预览:
              <span class="font-mono text-text-primary">
                {{ selectedDevices.map(d => d.ip).slice(0, 5).join(', ') }}{{ selectedDevices.length > 5 ? '...' : '' }}
              </span>
            </div>
          </div>
        </div>

        <!-- 可拖拽分隔条 -->
        <div
          class="h-2 flex items-center justify-center cursor-row-resize group py-1"
          @mousedown="startResize"
        >
          <div
            class="w-16 h-1.5 rounded-full bg-border group-hover:bg-accent/50 transition-colors"
          ></div>
        </div>

        <!-- 步骤2: 普通任务命令组 -->
        <div
          v-if="selectedTaskType === 'normal'"
          class="bg-bg-card border border-border rounded-xl overflow-hidden mb-4"
          :style="{ minHeight: commandPanelMinHeight + 'px' }"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <div
              class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent"
            >
              2
            </div>
            <span class="text-sm font-medium text-text-primary"
              >选择命令组</span
            >
            <span
              v-if="selectedCommandGroup"
              class="ml-2 text-xs text-accent font-mono"
              >{{ selectedCommandGroup.name }}</span
            >
          </div>
          <div class="p-3">
            <CommandGroupSelector
              v-model="selectedCommandGroupId"
              @selectionChange="onCommandGroupChange"
            />
          </div>
        </div>

        <!-- 步骤2: 备份采集参数 -->
        <div
          v-else-if="selectedTaskType === 'backup'"
          class="bg-bg-card border border-border rounded-xl overflow-hidden mb-4"
          :style="{ minHeight: commandPanelMinHeight + 'px' }"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <div
              class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent"
            >
              2
            </div>
            <span class="text-sm font-medium text-text-primary"
              >备份采集说明</span
            >
          </div>
          <div class="p-4 space-y-4">
            <p class="text-xs text-text-muted leading-relaxed">
              此任务类型将自动连接目标设备下载启动配置（通常为 startup-config 或 saved-configuration）。<br/><br/>
              默认参数（如保存目录、生成文件名规则等）将在创建任务时自动生成，您也可以在任务列表中点击编辑进行详情配置。
            </p>
            <!-- SFTP超时配置 -->
            <div class="rounded-lg border border-border bg-bg-panel p-3">
              <label class="flex items-start justify-between gap-4">
                <div class="flex-1">
                  <div class="text-xs font-medium text-text-secondary">
                    SFTP下载超时(秒)
                  </div>
                  <p class="text-xs text-text-muted mt-1">
                    SFTP下载大文件时的独立超时时间，设置为0时自动使用命令超时的2倍。
                  </p>
                </div>
                <input
                  v-model.number="backupSftpTimeoutSec"
                  type="number"
                  min="0"
                  class="w-24 px-3 py-1.5 rounded-lg bg-bg-card border border-border text-xs text-text-primary text-center focus:outline-none focus:border-accent/50"
                  placeholder="自动"
                />
              </label>
            </div>
          </div>
        </div>

        <!-- 步骤2: 拓扑参数 -->
        <div
          v-else
          class="bg-bg-card border border-border rounded-xl overflow-hidden mb-4"
          :style="{ minHeight: commandPanelMinHeight + 'px' }"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <div
              class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent"
            >
              2
            </div>
            <span class="text-sm font-medium text-text-primary"
              >拓扑采集参数</span
            >
          </div>
          <div class="p-3 space-y-3">
            <div>
              <label
                class="block text-xs font-medium text-text-secondary mb-1.5"
                >目标厂商</label
              >
              <select
                v-model="topologyVendor"
                class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
              >
                <option value="">自动识别</option>
                <option
                  v-for="vendor in supportedVendors"
                  :key="vendor"
                  :value="vendor"
                >
                  {{ vendor }}
                </option>
              </select>
            </div>
            <div class="rounded-lg border border-border bg-bg-panel p-3">
              <label class="flex items-start justify-between gap-4">
                <div>
                  <div class="text-xs font-medium text-text-secondary">
                    自动构建拓扑
                  </div>
                  <p class="text-xs text-text-muted mt-1">
                    采集完成后自动触发拓扑构建。
                  </p>
                </div>
                <input
                  v-model="autoBuildTopology"
                  type="checkbox"
                  class="mt-1 h-4 w-4"
                />
              </label>
            </div>

            <div
              class="rounded-lg border border-border bg-bg-panel p-3 space-y-3"
            >
              <div class="flex items-center justify-between gap-2">
                <div>
                  <div class="text-xs font-medium text-text-secondary">
                    字段级命令覆盖
                  </div>
                  <p class="text-xs text-text-muted mt-1">
                    在任务维度覆盖默认命令，执行前将按覆盖结果重新生成采集计划。
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    @click="loadTopologyPreview"
                    :disabled="topologyPreviewLoading"
                    class="px-2.5 py-1.5 rounded-lg text-xs border transition-all"
                    :class="
                      topologyPreviewLoading
                        ? 'bg-bg-card border-border text-text-muted cursor-not-allowed'
                        : 'bg-bg-card border-accent/30 text-accent hover:bg-accent hover:text-white'
                    "
                  >
                    {{ topologyPreviewLoading ? "刷新中..." : "刷新预览" }}
                  </button>
                  <button
                    @click="goToTopologyCommandConfig"
                    class="px-2.5 py-1.5 rounded-lg text-xs border border-border text-text-secondary hover:text-text-primary hover:border-accent/40 transition-all"
                  >
                    配置中心
                  </button>
                </div>
              </div>

              <div
                v-if="topologyPreviewDirty"
                class="text-xs px-2.5 py-2 rounded-lg border border-warning/30 bg-warning/10 text-warning"
              >
                检测到未刷新的拓扑命令变更，请先刷新预览后再创建任务。
              </div>

              <div
                v-if="topologyPreviewError"
                class="text-xs px-2.5 py-2 rounded-lg border border-error/30 bg-error/10 text-error"
              >
                {{ topologyPreviewError }}
              </div>

              <div
                v-if="topologyPreviewLoading"
                class="text-xs px-2.5 py-2 rounded-lg border border-border bg-bg-card text-text-muted"
              >
                正在加载拓扑命令预览...
              </div>

              <div
                v-else-if="topologyPreviewCommands.length === 0"
                class="text-xs px-2.5 py-2 rounded-lg border border-border bg-bg-card text-text-muted"
              >
                暂无预览命令。请选择设备后再刷新预览。
              </div>

              <div
                v-else
                class="space-y-2 max-h-[340px] overflow-y-auto scrollbar-custom pr-1"
              >
                <div
                  v-for="cmd in topologyPreviewCommands"
                  :key="cmd.fieldKey"
                  class="rounded-lg border border-border bg-bg-card p-2.5 space-y-2"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div>
                      <div class="flex items-center gap-2 flex-wrap">
                        <span class="text-xs font-medium text-text-primary">{{
                          cmd.displayName
                        }}</span>
                        <span
                          class="text-[11px] px-1.5 py-0.5 rounded bg-bg-panel border border-border text-text-muted font-mono"
                        >
                          {{ cmd.fieldKey }}
                        </span>
                        <span
                          class="text-[11px] px-1.5 py-0.5 rounded border"
                          :class="
                            cmd.required
                              ? 'border-warning/30 bg-warning/10 text-warning'
                              : 'border-border bg-bg-panel text-text-muted'
                          "
                        >
                          {{ cmd.required ? "关键字段" : "可选字段" }}
                        </span>
                        <span
                          class="text-[11px] px-1.5 py-0.5 rounded border border-accent/30 bg-accent/10 text-accent"
                        >
                          {{ cmd.commandSource || "unknown" }}
                        </span>
                      </div>
                      <p class="text-xs text-text-muted mt-1">
                        {{ cmd.description || "无描述" }}
                      </p>
                    </div>
                    <label
                      class="inline-flex items-center gap-1 text-xs text-text-secondary"
                    >
                      <input
                        type="checkbox"
                        class="h-3.5 w-3.5"
                        :checked="
                          topologyEnabledValue(cmd.fieldKey, cmd.enabled)
                        "
                        @change="onTopologyEnabledChange(cmd.fieldKey, $event)"
                      />
                      启用
                    </label>
                  </div>

                  <div
                    class="grid grid-cols-1 lg:grid-cols-[1fr_120px_auto] gap-2 items-start"
                  >
                    <textarea
                      rows="2"
                      class="w-full px-2.5 py-1.5 rounded-lg bg-bg-panel border border-border text-xs text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/40"
                      :value="topologyCommandValue(cmd.fieldKey, cmd.command)"
                      @input="onTopologyCommandInput(cmd.fieldKey, $event)"
                    ></textarea>
                    <input
                      type="number"
                      min="1"
                      class="w-full px-2.5 py-1.5 rounded-lg bg-bg-panel border border-border text-xs text-text-primary focus:outline-none focus:border-accent/40"
                      :value="
                        topologyTimeoutValue(cmd.fieldKey, cmd.timeoutSec)
                      "
                      @input="onTopologyTimeoutInput(cmd.fieldKey, $event)"
                    />
                    <button
                      @click="resetTopologyOverride(cmd.fieldKey)"
                      class="px-2.5 py-1.5 rounded-lg text-xs border border-border text-text-secondary hover:text-text-primary hover:border-accent/40 transition-all"
                    >
                      恢复继承
                    </button>
                  </div>
                </div>
              </div>

              <div class="flex items-center justify-between text-xs">
                <span class="text-text-muted"
                  >覆盖项 {{ topologyOverrides.length }} 条</span
                >
                <span v-if="topologyInvalidCount > 0" class="text-error">
                  存在 {{ topologyInvalidCount }} 条已启用但命令为空的覆盖项
                </span>
                <span v-else class="text-text-muted">覆盖项校验通过</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 底部提示 -->
    <div class="flex-shrink-0 text-center py-2">
      <p class="text-sm text-text-muted">
        选择设备并确认拓扑预览后，点击「创建任务」将绑定组合发送到任务执行页
      </p>
    </div>

    <!-- 创建任务弹窗 -->
    <Transition name="modal">
      <div
        v-if="createModal.show"
        class="fixed inset-0 z-50 flex items-center justify-center"
      >
        <div
          class="absolute inset-0 bg-black/60 backdrop-blur-sm"
          @click="createModal.show = false"
        ></div>
        <div
          class="relative bg-bg-card border border-border rounded-xl shadow-2xl max-w-lg w-full mx-4 overflow-hidden animate-slide-in"
        >
          <!-- 弹窗头部 -->
          <div
            class="flex items-center justify-between px-5 py-4 border-b border-border bg-bg-panel"
          >
            <h3 class="text-sm font-semibold text-text-primary">创建任务</h3>
            <button
              @click="createModal.show = false"
              class="text-text-muted hover:text-text-primary transition-colors"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>

          <!-- 弹窗内容 -->
          <div
            class="px-5 py-4 space-y-4 max-h-[60vh] overflow-auto scrollbar-custom"
          >
            <!-- 任务名称 -->
            <div>
              <label
                class="block text-xs font-medium text-text-secondary mb-1.5"
                >任务名称</label
              >
              <input
                v-model="createModal.name"
                type="text"
                class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
                placeholder="输入任务名称"
              />
            </div>

            <!-- 任务描述 -->
            <div>
              <label
                class="block text-xs font-medium text-text-secondary mb-1.5"
                >描述（可选）</label
              >
              <textarea
                v-model="createModal.description"
                rows="2"
                class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all resize-none"
                placeholder="输入任务描述"
              ></textarea>
            </div>

            <!-- 标签管理 -->
            <div>
              <label
                class="block text-xs font-medium text-text-secondary mb-1.5"
                >标签</label
              >
              <div class="flex flex-wrap gap-2 mb-2">
                <span
                  v-for="(tag, idx) in createModal.tags"
                  :key="idx"
                  class="flex items-center gap-1 text-xs px-2.5 py-1 rounded-full bg-accent/10 border border-accent/30 text-accent"
                >
                  {{ tag }}
                  <button
                    @click="createModal.tags.splice(idx, 1)"
                    class="hover:text-error transition-colors"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3 h-3"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <line x1="18" y1="6" x2="6" y2="18" />
                      <line x1="6" y1="6" x2="18" y2="18" />
                    </svg>
                  </button>
                </span>
              </div>
              <div class="flex gap-2">
                <input
                  v-model="createModal.newTag"
                  type="text"
                  class="flex-1 px-3 py-1.5 rounded-lg bg-bg-panel border border-border text-xs text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 transition-all"
                  placeholder="输入标签后按回车"
                  @keydown.enter.prevent="addTag"
                />
                <button
                  @click="addTag"
                  class="px-3 py-1.5 rounded-lg text-xs font-medium bg-accent/10 border border-accent/30 text-accent hover:bg-accent hover:text-white transition-all"
                >
                  添加
                </button>
              </div>
            </div>

            <!-- 绑定预览 -->
            <div
              class="bg-bg-panel border border-border rounded-lg p-3 space-y-2"
            >
              <h4 class="text-xs font-medium text-text-secondary mb-2">
                绑定预览
              </h4>
              <div class="flex items-center gap-2 text-xs">
                <span
                  class="px-2 py-0.5 rounded bg-accent/10 border border-accent/20 text-accent font-mono"
                >
                  {{
                    selectedTaskType === "normal"
                      ? selectedCommandGroup?.name || "未选择命令组"
                      : topologyVendor || "自动识别厂商"
                  }}
                </span>
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-3.5 h-3.5 text-text-muted"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <line x1="5" y1="12" x2="19" y2="12" />
                  <polyline points="12 5 19 12 12 19" />
                </svg>
                <span class="text-text-secondary"
                  >{{ selectedDevices.length }} 台设备</span
                >
              </div>
              <div class="flex flex-wrap gap-1 mt-1">
                <span
                  v-for="dev in selectedDevices.slice(0, 10)"
                  :key="dev.ip"
                  class="text-xs font-mono px-1.5 py-0.5 rounded bg-bg-card border border-border text-text-muted"
                  >{{ dev.ip }}</span
                >
                <span
                  v-if="selectedDevices.length > 10"
                  class="text-xs text-text-muted"
                  >+{{ selectedDevices.length - 10 }} 台</span
                >
              </div>
            </div>

            <div class="bg-bg-panel border border-border rounded-lg p-3">
              <label class="flex items-start justify-between gap-4">
                <div>
                  <div class="text-xs font-medium text-text-secondary">
                    原始日志
                  </div>
                  <p class="text-xs text-text-muted mt-1">
                    默认关闭。开启后会额外保存完整 SSH 字节流。
                  </p>
                </div>
                <input
                  v-model="createModal.enableRawLog"
                  type="checkbox"
                  class="mt-1 h-4 w-4"
                />
              </label>
            </div>
          </div>

          <!-- 弹窗底部 -->
          <div class="flex justify-end gap-3 px-5 py-4 border-t border-border">
            <button
              @click="createModal.show = false"
              class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-panel border border-border text-text-secondary hover:text-text-primary transition-all"
            >
              取消
            </button>
            <button
              @click="confirmCreate"
              :disabled="!createModal.name.trim()"
              class="px-5 py-2 rounded-lg text-sm font-semibold transition-all duration-200"
              :class="
                createModal.name.trim()
                  ? 'bg-accent hover:bg-accent-glow text-white border border-accent/30 hover:shadow-glow'
                  : 'bg-bg-card border border-border text-text-muted cursor-not-allowed'
              "
            >
              确认创建
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
          <svg
            v-if="toastType === 'success'"
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polyline points="20 6 9 17 4 12" />
          </svg>
          <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="10" />
            <line x1="15" y1="9" x2="9" y2="15" />
            <line x1="9" y1="9" x2="15" y2="15" />
          </svg>
          <span class="text-sm font-medium">{{ toastMessage }}</span>
        </div>
      </div>
    </Transition>

    <!-- 设备选择弹窗 -->
    <DeviceSelectorModal
      v-model:visible="showDeviceSelector"
      :devices="deviceList"
      :selected-i-ps="selectedDevices.map(d => d.ip)"
      title="选择目标设备"
      @confirm="onDeviceSelectionConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { useRouter } from "vue-router";
import {
  DeviceAPI,
  TaskGroupAPI,
  TopologyCommandAPI,
  TopologyCommandConfigAPI,
} from "../services/api";
import type {
  DeviceAsset,
  CommandGroup,
  TopologyCommandPreviewView,
  TopologyTaskFieldOverride,
} from "../services/api";
import DeviceSelectorModal from "../components/task/DeviceSelectorModal.vue";
import CommandGroupSelector from "../components/task/CommandGroupSelector.vue";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const router = useRouter();

// 设备列表和选择状态
const deviceList = ref<DeviceAsset[]>([]);
const selectedDevices = ref<DeviceAsset[]>([]);
const showDeviceSelector = ref(false);
const selectedTaskType = ref<"normal" | "topology" | "backup">("normal");
const selectedCommandGroupId = ref<number>(0);
const selectedCommandGroup = ref<CommandGroup | null>(null);
const supportedVendors = ref<string[]>([]);
const topologyVendor = ref("");
const autoBuildTopology = ref(true);
const backupSftpTimeoutSec = ref(0); // SFTP下载独立超时(秒)，0时使用命令超时的2倍

const topologyOverrides = ref<TopologyTaskFieldOverride[]>([]);
const topologyPreview = ref<TopologyCommandPreviewView | null>(null);
const topologyPreviewLoading = ref(false);
const topologyPreviewError = ref("");
const topologyPreviewDirty = ref(false);

// 面板高度控制
const devicePanelHeight = ref(280); // 设备选择面板默认高度
const commandPanelMinHeight = 300; // 命令组面板最小高度
const minHeight = 150; // 面板最小高度限制

// 拖拽调整高度相关
let isResizing = false;
let startY = 0;
let startHeight = 0;

const selectedDeviceIDsSignature = computed(() =>
  [...selectedDevices.value]
    .map((item) => item.id)
    .sort((a, b) => a - b)
    .join(","),
);

const topologyPreviewCommands = computed(
  () => topologyPreview.value?.defaultResolution?.commands || [],
);

const topologyInvalidCount = computed(
  () =>
    topologyOverrides.value.filter(
      (item: TopologyTaskFieldOverride) =>
        item.enabled === true && String(item.command || "").trim() === "",
    ).length,
);

function startResize(e: MouseEvent) {
  isResizing = true;
  startY = e.clientY;
  startHeight = devicePanelHeight.value;
  document.addEventListener("mousemove", onResize);
  document.addEventListener("mouseup", stopResize);
  document.body.style.cursor = "row-resize";
  document.body.style.userSelect = "none";
}

function onResize(e: MouseEvent) {
  if (!isResizing) return;
  const deltaY = e.clientY - startY;
  const newHeight = startHeight + deltaY;

  // 限制高度范围
  if (newHeight >= minHeight) {
    devicePanelHeight.value = newHeight;
  }
}

function stopResize() {
  isResizing = false;
  document.removeEventListener("mousemove", onResize);
  document.removeEventListener("mouseup", stopResize);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
}

// 组件卸载时清理事件监听
onUnmounted(() => {
  document.removeEventListener("mousemove", onResize);
  document.removeEventListener("mouseup", stopResize);
});

const canCreate = computed(() => {
  if (selectedTaskType.value === "topology") {
    return selectedDevices.value.length > 0 && topologyInvalidCount.value === 0;
  }
  if (selectedTaskType.value === "backup") {
    return selectedDevices.value.length > 0;
  }
  return selectedDevices.value.length > 0 && selectedCommandGroupId.value > 0;
});

// Toast 通知
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

// 创建任务弹窗
const createModal = ref({
  show: false,
  name: "",
  description: "",
  tags: [] as string[],
  newTag: "",
  enableRawLog: false,
});

function generateDefaultName() {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  const h = String(now.getHours()).padStart(2, "0");
  const mi = String(now.getMinutes()).padStart(2, "0");
  const s = String(now.getSeconds()).padStart(2, "0");
  return `Task_${y}${m}${d}_${h}${mi}${s}`;
}

function openCreateModal() {
  if (!canCreate.value) return;
  if (selectedTaskType.value === "topology" && topologyPreviewDirty.value) {
    triggerToast("拓扑命令存在未刷新的变更，请先刷新命令预览", "error");
    return;
  }
  createModal.value = {
    show: true,
    name: generateDefaultName(),
    description: "",
    tags: [],
    newTag: "",
    enableRawLog: false,
  };
}

function addTag() {
  const tag = createModal.value.newTag.trim();
  if (tag && !createModal.value.tags.includes(tag)) {
    createModal.value.tags.push(tag);
  }
  createModal.value.newTag = "";
}

async function confirmCreate() {
  if (!createModal.value.name.trim() || !canCreate.value) return;
  if (selectedTaskType.value === "topology") {
    if (topologyInvalidCount.value > 0) {
      triggerToast("存在无效拓扑覆盖项，请修正后重试", "error");
      return;
    }
    if (topologyPreviewDirty.value) {
      triggerToast("拓扑命令存在未刷新的变更，请先刷新命令预览", "error");
      return;
    }
  }

  try {
    const taskItems =
      selectedTaskType.value === "topology" || selectedTaskType.value === "backup"
        ? [
            {
              commandGroupId: "",
              commands: [] as string[],
              deviceIDs: selectedDevices.value.map((d: DeviceAsset) => d.id),
            },
          ]
        : [
            {
              commandGroupId: String(selectedCommandGroupId.value),
              commands: [] as string[],
              deviceIDs: selectedDevices.value.map((d: DeviceAsset) => d.id),
            },
          ];

    const taskGroup: any = {
      id: 0,
      name: createModal.value.name.trim(),
      description: createModal.value.description.trim(),
      deviceGroup: "",
      commandGroup: selectedCommandGroup.value?.name || "",
      maxWorkers: 10,
      timeout: 60,
      mode: "group" as const,
      taskType: selectedTaskType.value,
      topologyVendor:
        selectedTaskType.value === "topology" ? topologyVendor.value : "",
      topologyFieldOverrides:
        selectedTaskType.value === "topology"
          ? cloneTopologyOverrides(topologyOverrides.value)
          : [],
      autoBuildTopology:
        selectedTaskType.value === "topology" ? autoBuildTopology.value : false,
      items: taskItems,
      tags: createModal.value.tags,
      enableRawLog: createModal.value.enableRawLog,
      backupSaveRootPath: "",
      backupDirNamePattern: selectedTaskType.value === 'backup' ? "%Y-%M-%D" : "",
      backupFileNamePattern: selectedTaskType.value === 'backup' ? "%H_startup_%h%m%s.cfg" : "",
      backupStartupCommand: selectedTaskType.value === 'backup' ? "display startup" : "",
      backupSftpTimeoutSec: selectedTaskType.value === 'backup' ? backupSftpTimeoutSec.value : 0,
      status: "",
      createdAt: new Date(),
      updatedAt: new Date(),
    };

    const createdTask = await TaskGroupAPI.createTaskGroup(taskGroup);
    logger.debug(`任务创建成功，id=${createdTask?.id}`, 'Tasks');
    createModal.value.show = false;
    triggerToast("任务创建成功，正在跳转任务执行页", "success");
    router.push({
      path: "/task-execution",
      query: { refresh: String(Date.now()) },
    });
  } catch (err: any) {
    logger.error('创建任务失败', 'Tasks', err);
    triggerToast(`创建失败: ${err?.message || err}`, "error");
  }
}

async function loadTopologyVendors() {
  try {
    supportedVendors.value =
      (await TopologyCommandAPI.getTaskTopologyVendors()) || [];
  } catch (err) {
    logger.error('加载拓扑厂商列表失败', 'Tasks', err);
    supportedVendors.value =
      (await TopologyCommandConfigAPI.getSupportedTopologyVendors()) || [];
  }
}

function cloneTopologyOverrides(
  overrides?: TopologyTaskFieldOverride[],
): TopologyTaskFieldOverride[] {
  return (overrides || []).map((item) => ({
    fieldKey: String(item.fieldKey || "").trim(),
    command: String(item.command || ""),
    timeoutSec: Number(item.timeoutSec || 0),
    enabled: typeof item.enabled === "boolean" ? item.enabled : undefined,
  }));
}

function findTopologyOverride(fieldKey: string) {
  return topologyOverrides.value.find(
    (item: TopologyTaskFieldOverride) => item.fieldKey === fieldKey,
  );
}

function findTopologyOverrideIndex(fieldKey: string) {
  return topologyOverrides.value.findIndex(
    (item: TopologyTaskFieldOverride) => item.fieldKey === fieldKey,
  );
}

function ensureTopologyOverride(fieldKey: string) {
  const normalizedFieldKey = fieldKey.trim();
  let item = findTopologyOverride(normalizedFieldKey);
  if (item) {
    return item;
  }
  item = {
    fieldKey: normalizedFieldKey,
    command: "",
    timeoutSec: 0,
  };
  topologyOverrides.value = [...topologyOverrides.value, item];
  return item;
}

function compactTopologyOverride(fieldKey: string) {
  const index = findTopologyOverrideIndex(fieldKey);
  if (index < 0) {
    return;
  }
  const current = topologyOverrides.value[index];
  if (!current) {
    return;
  }
  const hasCommand = String(current.command || "") !== "";
  const hasTimeout = Number(current.timeoutSec || 0) > 0;
  const hasEnabled = typeof current.enabled === "boolean";
  if (hasCommand || hasTimeout || hasEnabled) {
    return;
  }
  topologyOverrides.value = topologyOverrides.value.filter(
    (item: TopologyTaskFieldOverride) => item.fieldKey !== fieldKey,
  );
}

function markTopologyPreviewDirty() {
  topologyPreviewDirty.value = true;
}

function topologyCommandValue(fieldKey: string, fallback: string) {
  const override = findTopologyOverride(fieldKey);
  if (override && override.command !== "") {
    return override.command;
  }
  return fallback || "";
}

function topologyTimeoutValue(fieldKey: string, fallback: number) {
  const override = findTopologyOverride(fieldKey);
  if (override && Number(override.timeoutSec || 0) > 0) {
    return Number(override.timeoutSec);
  }
  return Number(fallback || 0);
}

function topologyEnabledValue(fieldKey: string, fallback: boolean) {
  const override = findTopologyOverride(fieldKey);
  if (override && typeof override.enabled === "boolean") {
    return override.enabled;
  }
  return Boolean(fallback);
}

function onTopologyCommandInput(fieldKey: string, event: Event) {
  const target = event.target as HTMLTextAreaElement | null;
  const override = ensureTopologyOverride(fieldKey);
  override.command = target?.value || "";
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

function onTopologyTimeoutInput(fieldKey: string, event: Event) {
  const target = event.target as HTMLInputElement | null;
  const value = Number(target?.value || 0);
  const override = ensureTopologyOverride(fieldKey);
  override.timeoutSec = Number.isFinite(value) && value > 0 ? value : 0;
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

function onTopologyEnabledChange(fieldKey: string, event: Event) {
  const target = event.target as HTMLInputElement | null;
  const override = ensureTopologyOverride(fieldKey);
  override.enabled = Boolean(target?.checked);
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

async function resetTopologyOverride(fieldKey: string) {
  topologyOverrides.value = topologyOverrides.value.filter(
    (item: TopologyTaskFieldOverride) => item.fieldKey !== fieldKey,
  );
  markTopologyPreviewDirty();
  await loadTopologyPreview();
}

async function loadTopologyPreview() {
  if (selectedTaskType.value !== "topology") {
    return;
  }
  topologyPreviewLoading.value = true;
  topologyPreviewError.value = "";
  try {
    const nextPreview = await TopologyCommandAPI.previewTopologyCommands(
      topologyVendor.value,
      selectedDevices.value.map((item) => item.id),
      cloneTopologyOverrides(topologyOverrides.value),
    );
    topologyPreview.value = nextPreview;
    topologyOverrides.value = cloneTopologyOverrides(
      nextPreview?.taskOverrides || [],
    );
    topologyPreviewDirty.value = false;
  } catch (err: any) {
    logger.error('加载拓扑命令预览失败', 'Tasks', err);
    topologyPreviewError.value = `命令预览加载失败: ${err?.message || err}`;
  } finally {
    topologyPreviewLoading.value = false;
  }
}

// 设备选择确认
function onDeviceSelectionConfirm(devs: DeviceAsset[]) {
  selectedDevices.value = devs;
}

// 命令组选择变化
function onCommandGroupChange(group: CommandGroup | null) {
  selectedCommandGroup.value = group;
}

// 导航
function goToDevices() {
  router.push("/devices");
}

function goToTaskExecution() {
  router.push("/task-execution");
}

function goToTopologyCommandConfig() {
  router.push("/topology-command-config");
}

// 加载设备列表
async function loadDevices() {
  try {
    const result = await DeviceAPI.listDevices();
    // 后端已统一使用小写字段名，前端组件也已适配
    deviceList.value = result || [];
  } catch (err) {
    logger.error('加载设备列表失败', 'Tasks', err);
    deviceList.value = [];
  }
}

watch(
  () => [
    selectedTaskType.value,
    topologyVendor.value,
    selectedDeviceIDsSignature.value,
  ],
  async ([taskType]) => {
    if (taskType !== "topology") {
      return;
    }
    if (selectedDevices.value.length === 0) {
      topologyPreview.value = null;
      topologyPreviewDirty.value = false;
      topologyPreviewError.value = "";
      return;
    }
    await loadTopologyPreview();
  },
);

watch(selectedTaskType, async (value) => {
  if (value !== "topology") {
    topologyPreview.value = null;
    topologyPreviewError.value = "";
    topologyPreviewDirty.value = false;
    topologyOverrides.value = [];
    return;
  }
  if (selectedDevices.value.length === 0) {
    topologyPreview.value = null;
    topologyPreviewError.value = "";
    topologyPreviewDirty.value = false;
    return;
  }
  await loadTopologyPreview();
});

onMounted(() => {
  loadDevices();
  loadTopologyVendors();
});
</script>

