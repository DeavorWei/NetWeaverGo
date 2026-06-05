<template>
  <div>
    <Transition name="modal">
    <div
      v-if="modelValue"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="absolute inset-0 bg-black/60 backdrop-blur-sm"
        @click="closeModal"
      ></div>
      <div
        class="relative w-full max-w-6xl mx-4 max-h-[90vh] overflow-hidden rounded-2xl border border-border bg-bg-card shadow-2xl animate-slide-in flex flex-col"
      >
        <div
          class="flex items-start justify-between gap-4 px-6 py-5 border-b border-border bg-bg-panel"
        >
          <div>
            <h3 class="text-base font-semibold text-text-primary">编辑任务</h3>
            <p class="text-xs text-text-muted mt-1">
              {{
                isTopologyTaskValue
                  ? "拓扑任务支持编辑基础配置、设备范围、命令覆盖与解析预览"
                  : "任务执行中不可编辑，模式不可切换"
              }}
            </p>
          </div>
          <button
            @click="closeModal"
            class="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-secondary transition-colors"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-5 h-5"
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

        <div class="px-6 py-4 border-b border-border bg-bg-secondary/30 flex items-center gap-2 flex-wrap">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            @click="activeTab = tab.key"
            class="px-3 py-1.5 rounded-lg text-sm font-medium transition-all"
            :class="activeTab === tab.key ? 'bg-accent text-white' : 'bg-bg-card border border-border text-text-muted hover:text-text-primary'"
          >
            {{ tab.label }}
          </button>
        </div>

        <div class="flex-1 overflow-y-auto scrollbar-custom p-6">
          <div v-if="loading" class="h-64 flex items-center justify-center">
            <div
              class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"
            ></div>
          </div>
          <div
            v-else-if="!task"
            class="h-64 flex items-center justify-center text-sm text-text-muted"
          >
            暂无可编辑任务
          </div>
          <template v-else>
            <section v-show="activeTab === 'basic'" class="space-y-5">
              <div class="grid grid-cols-1 lg:grid-cols-[1fr,400px] gap-5">
                <!-- 左侧主列：核心基本信息 -->
                <div class="space-y-5">
                  <div class="bg-bg-panel border border-border rounded-xl p-5 space-y-4">
                    <h4 class="text-sm font-semibold text-text-primary flex items-center gap-2 mb-2">基本信息</h4>
                    
                    <div>
                      <label class="block text-xs font-medium text-text-secondary mb-1.5">任务名称</label>
                      <input
                        v-model="form.name"
                        type="text"
                        class="w-full px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50"
                        placeholder="输入任务名称"
                      />
                    </div>

                    <div>
                      <label class="block text-xs font-medium text-text-secondary mb-1.5">任务描述</label>
                      <textarea
                        v-model="form.description"
                        rows="2"
                        class="w-full px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 resize-none"
                        placeholder="输入任务描述"
                      ></textarea>
                    </div>

                    <div>
                      <label class="block text-xs font-medium text-text-secondary mb-1.5">标签</label>
                      <div class="flex flex-wrap gap-2 mb-2">
                        <span
                          v-for="(tag, index) in form.tags"
                          :key="`${tag}-${index}`"
                          class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs bg-accent/10 border border-accent/20 text-accent"
                        >
                          {{ tag }}
                          <button @click="removeTag(index)" class="hover:text-error transition-colors ml-1">
                            <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
                          </button>
                        </span>
                      </div>
                      <div class="flex gap-2">
                        <input
                          v-model="newTag"
                          type="text"
                          class="flex-1 px-3 py-1.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50"
                          placeholder="添加新标签后按回车"
                          @keydown.enter.prevent="addTag"
                        />
                        <button
                          @click="addTag"
                          class="px-3 py-1.5 rounded-lg text-xs font-medium bg-bg-secondary border border-border text-text-primary hover:bg-bg-hover"
                        >
                          添加
                        </button>
                      </div>
                    </div>
                  </div>

                  <!-- 任务专属配置卡片 (拓扑) -->
                  <div v-if="isTopologyTaskValue" class="bg-bg-panel border border-border rounded-xl p-5 space-y-4">
                    <h4 class="text-sm font-semibold text-text-primary flex items-center gap-2 mb-2">拓扑采集设定</h4>
                    <div class="grid grid-cols-2 gap-4">
                      <label class="block">
                        <span class="block text-xs font-medium text-text-secondary mb-1.5">目标厂商</span>
                        <select
                          v-model="topologyForm.vendor"
                          class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50"
                        >
                          <option value="">自动识别</option>
                          <option v-for="vendor in topologyVendorOptions" :key="vendor" :value="vendor">
                            {{ vendor }}
                          </option>
                        </select>
                      </label>
                      <label class="flex flex-col justify-center gap-2 rounded-lg border border-border bg-bg-card px-3 py-2">
                        <div class="flex items-center justify-between">
                          <span class="text-xs font-medium text-text-primary">自动构图</span>
                          <input v-model="topologyForm.autoBuildTopology" type="checkbox" class="h-3.5 w-3.5" />
                        </div>
                        <p class="text-[10px] text-text-muted">采集完成后自动执行解析与构图</p>
                      </label>
                    </div>
                  </div>

                  <!-- 任务专属配置卡片 (备份) -->
                  <div v-if="isBackupTaskValue" class="bg-bg-panel border border-border rounded-xl p-5 space-y-4">
                    <h4 class="text-sm font-semibold text-text-primary flex items-center gap-2 mb-2">备份路径与规则</h4>
                    <div class="grid grid-cols-2 gap-4">
                      <label class="block">
                        <span class="block text-xs font-medium text-text-secondary mb-1.5">根保存路径 (相对于执行器)</span>
                        <input v-model="backupForm.saveRootPath" type="text" class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50" />
                      </label>
                      <label class="block">
                        <span class="block text-xs font-medium text-text-secondary mb-1.5">SFTP下载超时 (秒)</span>
                        <input v-model.number="backupForm.sftpTimeoutSec" type="number" min="0" placeholder="0 表示自动" class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50" />
                      </label>
                      <label class="block">
                        <span class="block text-xs font-medium text-text-secondary mb-1.5">目录名模板</span>
                        <input v-model="backupForm.dirNamePattern" type="text" class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50" />
                      </label>
                      <label class="block">
                        <span class="block text-xs font-medium text-text-secondary mb-1.5">文件名模板</span>
                        <input v-model="backupForm.fileNamePattern" type="text" class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50" />
                      </label>
                    </div>
                  </div>
                </div>

                <!-- 右侧侧边栏：执行控制与概览 -->
                <div class="space-y-5">
                  <!-- 执行控制卡片 -->
                  <div class="bg-bg-panel border border-border rounded-xl p-5 space-y-4">
                    <h4 class="text-sm font-semibold text-text-primary flex items-center gap-2 mb-2">执行参数</h4>
                    <div class="grid grid-cols-2 gap-4">
                      <label class="block">
                        <span class="block text-xs font-medium text-text-secondary mb-1.5">最大并发设备数</span>
                        <input v-model.number="executionForm.maxWorkers" type="number" min="1" class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50" />
                      </label>
                      <label class="block">
                        <span class="block text-xs font-medium text-text-secondary mb-1.5">全局超时 (秒)</span>
                        <input v-model.number="executionForm.timeout" type="number" min="1" class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50" />
                      </label>
                    </div>
                    
                    <label class="flex items-start justify-between gap-3 p-3 rounded-lg border border-border bg-bg-card mt-2">
                      <div>
                        <div class="text-xs font-medium text-text-primary">保留原始日志 (Raw Log)</div>
                        <p class="text-[10px] text-text-muted mt-0.5">额外保存完整 SSH 字节流用于排障</p>
                      </div>
                      <input v-model="form.enableRawLog" type="checkbox" class="mt-0.5 h-3.5 w-3.5" />
                    </label>
                  </div>

                  <!-- 任务概览信息 -->
                  <div class="bg-bg-card border border-border rounded-xl p-4 space-y-3">
                    <h4 class="text-sm font-semibold text-text-primary mb-3">当前任务概况</h4>
                    <div class="flex items-center justify-between">
                      <span class="text-xs text-text-muted">运行模式</span>
                      <span class="text-xs font-semibold text-text-primary bg-bg-panel px-2 py-0.5 rounded border border-border">{{ modeLabel(task.mode) }}</span>
                    </div>
                    <div class="flex items-center justify-between">
                      <span class="text-xs text-text-muted">对象类型</span>
                      <span class="text-xs font-semibold text-text-primary bg-bg-panel px-2 py-0.5 rounded border border-border">{{ isTopologyTaskValue ? "拓扑任务" : isBackupTaskValue ? "配置备份" : "普通执行" }}</span>
                    </div>
                    <div class="flex items-center justify-between">
                      <span class="text-xs text-text-muted">设备库存总数</span>
                      <span class="text-xs font-mono text-text-primary">{{ allDevices.length }}</span>
                    </div>
                    <div class="flex items-center justify-between">
                      <span class="text-xs text-text-muted">可选命令组</span>
                      <span class="text-xs font-mono text-text-primary">{{ commandGroups.length }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section v-show="activeTab === 'config'" class="space-y-5 h-full">
              <!-- 普通群组模式 -->
              <div v-if="task.mode === 'group' && !isTopologyTaskValue && !isBackupTaskValue" class="grid grid-cols-1 lg:grid-cols-2 gap-5 h-full min-h-[400px]">
                <!-- 左侧：命令组选择 -->
                <div class="bg-bg-panel border border-border rounded-xl flex flex-col overflow-hidden">
                  <div class="px-5 py-3.5 border-b border-border/60 bg-bg-secondary/40 flex items-center justify-between">
                    <h4 class="text-sm font-semibold text-text-primary">选择执行指令</h4>
                  </div>
                  <div class="p-5 flex-1 flex flex-col gap-4">
                    <select v-model="groupForm.commandGroupId" class="w-full px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50">
                      <option value="">请选择命令组</option>
                      <option v-for="group in commandGroups" :key="group.id" :value="group.id">{{ group.name }} ({{ group.commands.length }} 条命令)</option>
                    </select>
                    
                    <div class="flex-1 rounded-lg border border-border bg-bg-card flex flex-col min-h-[200px] overflow-hidden">
                      <div class="px-3 py-2 border-b border-border bg-bg-secondary/20 text-xs font-medium text-text-secondary">
                        指令预览
                      </div>
                      <div class="flex-1 overflow-y-auto p-3 scrollbar-custom">
                        <div v-for="(command, index) in selectedGroupCommands" :key="`${groupForm.commandGroupId}-${index}`" class="font-mono text-sm text-text-primary py-1.5 border-b border-border/40 last:border-0 hover:bg-bg-secondary/30 px-2 rounded-md transition-colors">
                          <span class="text-text-muted mr-2 opacity-60">{{ index + 1 }}.</span>{{ command }}
                        </div>
                        <div v-if="selectedGroupCommands.length === 0" class="h-full flex items-center justify-center text-sm text-text-muted">
                          当前命令组暂无命令
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 右侧：设备选择 -->
                <div class="bg-bg-panel border border-border rounded-xl flex flex-col overflow-hidden">
                  <div class="px-5 py-3.5 border-b border-border/60 bg-bg-secondary/40 flex items-center justify-between">
                    <h4 class="text-sm font-semibold text-text-primary">选择执行目标</h4>
                    <span class="text-xs px-2.5 py-1 rounded-full bg-accent/10 text-accent font-medium">已选 {{ groupForm.deviceIDs.length }} 台</span>
                  </div>
                  <div class="p-5 flex-1 flex flex-col gap-4">
                    <button @click="openDeviceSelector" class="w-full py-3 rounded-lg text-sm font-medium bg-accent/10 border border-accent/30 text-accent hover:bg-accent hover:text-white transition-colors flex items-center justify-center gap-2">
                      <el-icon><Search /></el-icon> 点击选择设备
                    </button>
                    <div class="flex-1 rounded-lg border border-border bg-bg-card p-4 overflow-y-auto min-h-[200px] flex flex-col">
                      <div class="text-xs font-medium text-text-secondary mb-2 shrink-0">已选设备预览:</div>
                      <div v-if="groupForm.deviceIDs.length > 0" class="font-mono text-sm text-text-primary leading-relaxed break-all">
                        {{ selectedDevicesPreview }}
                      </div>
                      <div v-else class="flex-1 flex items-center justify-center text-sm text-text-muted">
                        尚未选择目标设备
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 拓扑群组模式 -->
              <div v-else-if="isTopologyTaskValue" class="space-y-5">
                <!-- 顶部警告与提示 -->
                <div class="rounded-lg border border-accent/20 bg-accent/5 px-4 py-3 flex items-start gap-3">
                  <el-icon class="text-accent mt-0.5"><InfoFilled /></el-icon>
                  <div class="text-sm text-text-secondary leading-relaxed">
                    拓扑采集采用“固定字段 + 任务级覆盖”模式。您可以在下方预览各目标设备的默认解析厂商。<br/>
                    如需针对此任务定制特定字段的采集命令，请点击右侧的覆盖配置按钮。
                  </div>
                </div>

                <div class="grid grid-cols-1 lg:grid-cols-[1fr,400px] gap-5">
                  <!-- 左侧：目标设备与解析预览 -->
                  <div class="bg-bg-panel border border-border rounded-xl flex flex-col overflow-hidden h-[500px]">
                    <div class="px-5 py-3.5 border-b border-border/60 bg-bg-secondary/40 flex items-center justify-between">
                      <h4 class="text-sm font-semibold text-text-primary flex items-center gap-2">
                        执行目标与解析预览
                      </h4>
                      <div class="flex items-center gap-3">
                        <span class="text-xs font-medium text-text-muted">默认解析: {{ topologyPreview?.defaultResolution?.resolvedVendor || topologyForm.vendor || "huawei" }}</span>
                        <span class="text-xs px-2.5 py-1 rounded-full bg-accent/10 text-accent font-medium">已选 {{ groupForm.deviceIDs.length }} 台</span>
                      </div>
                    </div>
                    <div class="p-5 flex-1 flex flex-col gap-4 overflow-hidden">
                      <button @click="openDeviceSelector" class="w-full shrink-0 py-2.5 rounded-lg text-sm font-medium bg-accent/10 border border-accent/30 text-accent hover:bg-accent hover:text-white transition-colors flex items-center justify-center gap-2">
                        <el-icon><Search /></el-icon> 点击选择设备
                      </button>
                      
                      <div class="flex-1 rounded-lg border border-border bg-bg-card flex flex-col overflow-hidden relative">
                        <div class="absolute right-2 top-2 z-10">
                          <button @click="loadTopologyPreview" type="button" class="px-3 py-1.5 rounded-md text-xs font-medium bg-bg-panel border border-border hover:bg-bg-secondary transition-colors flex items-center gap-1.5 disabled:opacity-50" :disabled="topologyPreviewLoading">
                            <el-icon :class="{'animate-spin': topologyPreviewLoading}"><RefreshLeft /></el-icon> 刷新预览
                          </button>
                        </div>
                        <div class="px-4 py-3 border-b border-border bg-bg-secondary/20 flex items-center justify-between">
                          <span class="text-xs font-medium text-text-secondary">设备解析策略一览</span>
                        </div>
                        <div v-if="selectedTopologyDevices.length === 0" class="flex-1 flex items-center justify-center text-sm text-text-muted">
                          尚未选择目标设备
                        </div>
                        <div v-else class="flex-1 overflow-y-auto p-3 scrollbar-custom space-y-2">
                          <div v-for="device in selectedTopologyDevices" :key="`preview-${device.id}`" class="rounded-lg border border-border/60 bg-bg-panel p-3 hover:border-accent/40 transition-colors">
                            <div class="flex items-center justify-between mb-1.5">
                              <span class="font-mono text-sm font-medium text-text-primary">{{ device.ip }}</span>
                              <span class="text-[10px] px-1.5 py-0.5 rounded bg-bg-secondary text-text-muted border border-border">{{ device.vendor || "未知厂商" }}</span>
                            </div>
                            <div class="flex items-center justify-between text-xs">
                              <span class="text-text-muted flex items-center gap-1">
                                <span class="w-1.5 h-1.5 rounded-full" :class="findDevicePreview(device.id)?.resolution?.resolvedVendor ? 'bg-success' : 'bg-warning'"></span>
                                最终解析: <span class="font-medium text-text-primary">{{ findDevicePreview(device.id)?.resolution?.resolvedVendor || topologyPreview?.defaultResolution?.resolvedVendor || topologyForm.vendor || "huawei" }}</span>
                              </span>
                              <span class="text-text-muted opacity-70">
                                来源: {{ findDevicePreview(device.id)?.resolution?.vendorSource || topologyPreview?.defaultResolution?.vendorSource || "fallback_default" }}
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- 右侧：任务级覆盖 -->
                  <div class="space-y-4">
                    <div class="bg-bg-panel border border-border rounded-xl overflow-hidden">
                      <div class="px-5 py-3.5 border-b border-border/60 bg-bg-secondary/40">
                        <h4 class="text-sm font-semibold text-text-primary flex items-center gap-2">
                          <el-icon class="text-accent"><Cpu /></el-icon> 命令覆盖配置
                        </h4>
                      </div>
                      <div class="p-5 space-y-4">
                        <p class="text-xs text-text-muted leading-relaxed">
                          对当前采集任务中特定的拓扑字段（如 ARP, MAC, LLDP 等）配置独立的采集命令或超时参数。
                        </p>
                        
                        <div class="bg-bg-card border border-border rounded-lg p-4 flex flex-col items-center justify-center text-center space-y-3">
                          <div class="w-12 h-12 rounded-full bg-accent/10 flex items-center justify-center text-accent mb-1">
                            <span class="text-xl font-bold">{{ topologyOverrides.length }}</span>
                          </div>
                          <div class="text-sm text-text-primary font-medium">当前已配置定制项</div>
                          
                          <el-button type="primary" class="w-full !rounded-lg mt-2" @click="openTopologyOverrideDialog">
                            管理覆盖配置
                          </el-button>
                        </div>
                        
                        <div v-if="topologyOverrides.length > 0" class="pt-2">
                          <el-button link type="danger" size="small" class="w-full" @click="clearAllTopologyOverrides">
                            清除全部覆盖项
                          </el-button>
                        </div>
                      </div>
                    </div>
                    
                    <div v-if="topologyPreviewDirty" class="rounded-lg border border-warning/30 bg-warning/10 px-4 py-3 flex items-start gap-2">
                      <el-icon class="text-warning mt-0.5"><InfoFilled /></el-icon>
                      <div class="text-xs text-warning">检测到覆盖配置有变更，请点击“刷新预览”更新解析策略。</div>
                    </div>
                    <div v-if="topologyInvalidCount > 0" class="rounded-lg border border-error/30 bg-error/10 px-4 py-3 text-xs text-error">
                      存在 {{ topologyInvalidCount }} 条已启用但内容为空的定制命令，保存前必须修正。
                    </div>
                    <div v-if="topologyPreviewError" class="rounded-lg border border-error/30 bg-error/10 px-4 py-3 text-xs text-error">
                      {{ topologyPreviewError }}
                    </div>
                  </div>
                </div>
              </div>

              <!-- 配置备份群组模式 -->
              <div v-else-if="isBackupTaskValue" class="grid grid-cols-1 lg:grid-cols-2 gap-5 h-full min-h-[400px]">
                <div class="bg-bg-panel border border-border rounded-xl flex flex-col overflow-hidden">
                  <div class="px-5 py-3.5 border-b border-border/60 bg-bg-secondary/40 flex items-center justify-between">
                    <h4 class="text-sm font-semibold text-text-primary">查询指令</h4>
                  </div>
                  <div class="p-5 flex-1 flex flex-col gap-4">
                    <div class="rounded-lg border border-accent/20 bg-accent/5 px-4 py-3 flex items-start gap-3 mb-2">
                      <el-icon class="text-accent mt-0.5"><InfoFilled /></el-icon>
                      <div class="text-sm text-text-secondary leading-relaxed">
                        配置备份任务将执行下方命令并将结果保存。
                      </div>
                    </div>
                    <div>
                      <label class="block text-xs font-medium text-text-secondary mb-1.5">配置查询命令</label>
                      <input v-model="backupForm.startupCommand" type="text" class="w-full px-4 py-3 rounded-lg bg-terminal-bg text-terminal-text border border-border font-mono text-sm focus:outline-none focus:border-accent/50" />
                    </div>
                  </div>
                </div>

                <div class="bg-bg-panel border border-border rounded-xl flex flex-col overflow-hidden">
                  <div class="px-5 py-3.5 border-b border-border/60 bg-bg-secondary/40 flex items-center justify-between">
                    <h4 class="text-sm font-semibold text-text-primary">选择执行目标</h4>
                    <span class="text-xs px-2.5 py-1 rounded-full bg-accent/10 text-accent font-medium">已选 {{ groupForm.deviceIDs.length }} 台</span>
                  </div>
                  <div class="p-5 flex-1 flex flex-col gap-4">
                    <button @click="openDeviceSelector" class="w-full py-3 rounded-lg text-sm font-medium bg-accent/10 border border-accent/30 text-accent hover:bg-accent hover:text-white transition-colors flex items-center justify-center gap-2">
                      <el-icon><Search /></el-icon> 点击选择设备
                    </button>
                    <div class="flex-1 rounded-lg border border-border bg-bg-card p-4 overflow-y-auto min-h-[200px] flex flex-col">
                      <div class="text-xs font-medium text-text-secondary mb-2 shrink-0">已选设备预览:</div>
                      <div v-if="groupForm.deviceIDs.length > 0" class="font-mono text-sm text-text-primary leading-relaxed break-all">
                        {{ selectedDevicesPreview }}
                      </div>
                      <div v-else class="flex-1 flex items-center justify-center text-sm text-text-muted">
                        尚未选择目标设备
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 分别绑定模式 -->
              <div v-else class="space-y-5">
                <div class="flex items-center justify-between px-2">
                  <div>
                    <h4 class="text-sm font-semibold text-text-primary">任务项独立配置</h4>
                    <p class="text-xs text-text-muted mt-1">分别绑定模式下，允许为不同设备组合配置独立的执行命令集。</p>
                  </div>
                  <button @click="addBindingItem" class="px-4 py-2 rounded-lg text-sm font-medium bg-accent text-white hover:bg-accent-glow flex items-center gap-1.5 shadow-sm shadow-accent/20">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
                    新增组合项
                  </button>
                </div>

                <div v-for="(item, index) in bindingForm.items" :key="`binding-item-${index}`" class="rounded-xl border border-border bg-bg-panel shadow-sm overflow-hidden flex flex-col">
                  <div class="px-5 py-3 border-b border-border/60 bg-bg-secondary/30 flex items-center justify-between">
                    <div class="flex items-center gap-3">
                      <span class="flex items-center justify-center w-6 h-6 rounded-full bg-accent/10 text-accent font-bold text-xs">{{ index + 1 }}</span>
                      <h5 class="text-sm font-semibold text-text-primary">组合配置项</h5>
                    </div>
                    <button @click="removeBindingItem(index)" class="px-2.5 py-1.5 rounded-md text-xs font-medium text-error hover:bg-error/10 transition-colors">
                      移除
                    </button>
                  </div>

                  <div class="p-5 grid grid-cols-1 lg:grid-cols-[1fr,1.5fr] gap-6">
                    <div class="flex flex-col gap-3">
                      <div class="flex items-center justify-between">
                        <label class="text-xs font-medium text-text-secondary">目标设备概览</label>
                        <span class="text-[10px] px-2 py-0.5 rounded-full bg-bg-secondary text-text-primary border border-border">已选 {{ item.deviceIDs.length }} 台</span>
                      </div>
                      <div class="flex-1 rounded-lg border border-border bg-bg-card p-2 max-h-[300px] overflow-y-auto scrollbar-custom grid grid-cols-1 gap-1.5">
                        <label v-for="device in allDevices" :key="`${index}-${device.id}`" class="flex items-start gap-3 rounded-md border border-border/50 bg-bg-panel px-3 py-2.5 hover:border-accent/40 hover:bg-accent/5 transition-colors cursor-pointer group">
                          <input type="checkbox" :checked="item.deviceIDs.includes(device.id)" @change="toggleBindingDevice(index, device.id)" class="mt-0.5" />
                          <div class="min-w-0 flex-1">
                            <div class="font-mono text-sm text-text-primary group-hover:text-accent transition-colors">{{ device.ip }}</div>
                            <div class="text-[10px] text-text-muted mt-0.5">分组: {{ device.group || "未分组" }}</div>
                          </div>
                        </label>
                      </div>
                    </div>

                    <div class="flex flex-col gap-3">
                      <label class="text-xs font-medium text-text-secondary">执行命令内容</label>
                      <textarea v-model="item.commandsText" class="w-full flex-1 min-h-[300px] p-4 rounded-lg bg-terminal-bg text-terminal-text border border-border font-mono text-sm resize-none focus:outline-none focus:border-accent/50 leading-relaxed" placeholder="每行输入一条命令"></textarea>
                    </div>
                  </div>
                </div>
                
                <div v-if="bindingForm.items.length === 0" class="rounded-xl border border-dashed border-border/80 bg-bg-panel p-10 flex flex-col items-center justify-center text-center gap-3">
                  <div class="w-12 h-12 rounded-full bg-bg-secondary flex items-center justify-center text-text-muted">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" /><polyline points="3.27 6.96 12 12.01 20.73 6.96" /><line x1="12" y1="22.08" x2="12" y2="12" /></svg>
                  </div>
                  <div class="text-sm font-medium text-text-primary">暂无组合项</div>
                  <p class="text-xs text-text-muted max-w-sm">您尚未创建任何设备-命令组合项，点击右上角新增组合项开始配置。</p>
                </div>
              </div>
            </section>

            <div
              v-if="formError"
              class="mt-5 rounded-lg border border-error/30 bg-error/10 px-4 py-3 text-sm text-error"
            >
              {{ formError }}
            </div>
          </template>
        </div>

        <div
          class="flex justify-end gap-3 px-6 py-4 border-t border-border bg-bg-panel"
        >
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
            {{ saving ? "保存中..." : "保存任务" }}
          </button>
        </div>
      </div>
    </div>
  </Transition>

    <!-- 设备选择弹窗 -->
    <DeviceSelectorModal
      v-model:visible="showDeviceSelector"
      :devices="allDevices"
      :selected-i-ps="selectedTopologyDevices.map(d => d.ip)"
      title="选择目标设备"
      @confirm="handleDeviceConfirm"
    />

    <!-- 拓扑命令任务级覆盖配置弹窗 -->
    <el-dialog
      v-model="showTopologyOverrideDialog"
      title="定制任务级拓扑命令与字段覆盖"
      width="1000px"
      append-to-body
      destroy-on-close
      class="topology-override-dialog"
      :close-on-click-modal="false"
    >
      <div class="h-[600px] flex flex-col gap-4 -mt-2">
        <div class="flex items-center justify-between gap-3 bg-accent/5 border border-accent/15 rounded-xl px-4 py-3 text-xs text-text-secondary flex-shrink-0">
          <span class="flex items-center gap-1.5">
            <el-icon class="text-accent"><InfoFilled /></el-icon>
            任务级覆盖的命令优先级高于全局。如果您需要退回使用厂商默认配置，请点击右侧的“恢复继承”按钮。
          </span>
          <div class="flex items-center gap-2">
            <el-button
              type="primary"
              size="small"
              :loading="topologyPreviewLoading"
              @click="loadTopologyPreview"
              class="!rounded-md"
            >
              刷新解析预览
            </el-button>
            <span v-if="topologyPreviewDirty" class="text-warning font-semibold animate-pulse">
              (检测到未刷新的覆盖项)
            </span>
          </div>
        </div>

        <!-- 两栏配置主体 -->
        <div class="flex-1 flex gap-4 min-h-0">
          <!-- 左栏: 字段导航 -->
          <div class="w-72 flex-shrink-0 flex flex-col border border-border bg-bg-panel rounded-xl overflow-hidden">
            <!-- 指标小看板 -->
            <div class="p-3 border-b border-border grid grid-cols-2 gap-2 bg-bg-secondary/40">
              <div class="bg-bg-card border border-border/80 rounded-lg p-2 text-center">
                <span class="block text-sm font-bold text-text-primary">{{ topologyPreviewCommands.length }}</span>
                <span class="text-[9px] text-text-muted uppercase tracking-wider">总字段数</span>
              </div>
              <div class="bg-bg-card border border-border/80 rounded-lg p-2 text-center">
                <span class="block text-sm font-bold text-accent">{{ topologyOverrides.length }}</span>
                <span class="text-[9px] text-text-muted uppercase tracking-wider">已覆盖项</span>
              </div>
            </div>

            <!-- 过滤搜索 -->
            <div class="p-3 border-b border-border bg-bg-secondary/20">
              <el-input
                v-model="overrideSearchQuery"
                placeholder="搜索字段或 Key..."
                clearable
                size="small"
              >
                <template #prefix>
                  <el-icon class="text-text-muted"><Search /></el-icon>
                </template>
              </el-input>
            </div>

            <!-- 列表 -->
            <div class="flex-1 overflow-y-auto p-2 space-y-1.5 bg-bg-secondary/5">
              <div
                v-for="command in filteredPreviewCommands"
                :key="command.fieldKey"
                @click="selectedOverrideFieldKey = command.fieldKey"
                class="p-2.5 border rounded-lg cursor-pointer transition-all duration-150 select-none flex flex-col gap-1"
                :class="[
                  selectedOverrideFieldKey === command.fieldKey
                    ? 'bg-accent/10 border-accent shadow-sm'
                    : 'bg-bg-card border-border/60 hover:bg-bg-hover hover:border-border'
                ]"
              >
                <div class="flex items-center justify-between gap-2">
                  <span 
                    class="font-bold text-xs text-text-primary line-clamp-1 transition-colors"
                    :class="{'text-accent': selectedOverrideFieldKey === command.fieldKey}"
                  >
                    {{ command.displayName || command.fieldKey }}
                  </span>
                  <span 
                    class="w-1.5 h-1.5 rounded-full flex-shrink-0"
                    :class="[topologyEnabledValue(command.fieldKey, command.enabled) ? 'bg-success shadow-[0_0_4px_var(--color-success)]' : 'bg-text-muted/30']"
                  ></span>
                </div>
                <span class="text-[10px] text-text-muted font-mono truncate">{{ command.fieldKey }}</span>
                
                <div class="flex items-center gap-1.5 mt-0.5">
                  <el-tag v-if="command.required" type="warning" size="small" class="!px-1 !h-4 !line-height-4 !rounded">
                    必填
                  </el-tag>
                  <el-tag v-if="findTopologyOverride(command.fieldKey)" type="danger" size="small" class="!px-1 !h-4 !line-height-4 !rounded">
                    已覆盖
                  </el-tag>
                </div>
              </div>
              <div v-if="filteredPreviewCommands.length === 0" class="text-center py-6 text-xs text-text-muted">
                未找到匹配字段
              </div>
            </div>
          </div>

          <!-- 右栏: 详细编辑区 -->
          <div class="flex-1 flex flex-col border border-border bg-bg-card rounded-xl overflow-hidden">
            <div v-if="currentOverrideField" class="flex-1 flex flex-col min-h-0">
              
              <!-- 头部信息 -->
              <div class="p-4 border-b border-border bg-bg-secondary/40 flex items-start justify-between gap-4 flex-shrink-0">
                <div class="space-y-1 min-w-0">
                  <div class="flex items-center gap-2 flex-wrap">
                    <h4 class="text-sm font-bold text-text-primary">{{ currentOverrideField.displayName || currentOverrideField.fieldKey }}</h4>
                    <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-bg-panel border border-border text-text-secondary truncate max-w-[150px]">{{ currentOverrideField.fieldKey }}</span>
                  </div>
                  <p class="text-[11px] text-text-muted leading-relaxed max-w-xl truncate">
                    {{ currentOverrideField.description || "暂无描述说明。" }}
                  </p>
                </div>
                <div class="flex items-center gap-1.5 flex-shrink-0">
                  <el-tag v-if="currentOverrideField.required" type="warning" effect="dark" size="small" class="!rounded-md">
                    必填
                  </el-tag>
                  <el-button
                    v-if="findTopologyOverride(currentOverrideField.fieldKey)"
                    type="info"
                    plain
                    size="small"
                    :icon="RefreshLeft"
                    @click="resetTopologyOverride(currentOverrideField.fieldKey)"
                    class="!rounded-md"
                  >
                    恢复继承
                  </el-button>
                </div>
              </div>

              <!-- 主编辑体 -->
              <div class="flex-1 overflow-y-auto p-4 flex flex-col gap-4">
                <!-- 命令输入 -->
                <div class="flex-1 flex flex-col gap-1.5 min-h-[10rem]">
                  <div class="flex items-center justify-between shrink-0">
                    <label class="text-xs font-semibold text-text-muted uppercase tracking-wider flex items-center gap-1">
                      <el-icon><Cpu /></el-icon>
                      定制采集命令
                    </label>
                    <span class="text-[10px] text-text-muted font-mono bg-bg-panel px-1.5 py-0.2 rounded border border-border/60">
                      绑定: {{ currentOverrideField.parserBinding || "无" }}
                    </span>
                  </div>
                  <div class="border border-border/80 focus-within:border-accent focus-within:shadow-[0_0_0_3px_var(--color-accent-bg)] rounded-lg bg-bg-panel overflow-hidden transition-all flex flex-col flex-1">
                    <textarea
                      :value="topologyCommandValue(currentOverrideField.fieldKey, currentOverrideField.command)"
                      @input="onTopologyCommandInput(currentOverrideField.fieldKey, $event)"
                      placeholder="输入该任务专属的采集命令。留空表示继续继承厂商默认配置"
                      class="premium-textarea h-full"
                    ></textarea>
                  </div>
                </div>

                <!-- 超时与开关 Grid -->
                <div class="bg-bg-secondary/20 border border-border/60 rounded-lg p-4 space-y-4 shrink-0">
                  <div class="grid grid-cols-2 gap-4">
                    <!-- 超时秒数 -->
                    <div class="space-y-1">
                      <label class="text-xs font-medium text-text-primary block">采集超时 (秒)</label>
                      <el-input-number
                        :model-value="topologyTimeoutValue(currentOverrideField.fieldKey, currentOverrideField.timeoutSec)"
                        @change="(val: any) => onTopologyTimeoutChange(currentOverrideField!.fieldKey, val)"
                        :min="1"
                        :max="3600"
                        class="w-full"
                        controls-position="right"
                        size="small"
                      />
                    </div>

                    <!-- 启用开关 -->
                    <div class="space-y-1">
                      <label class="text-xs font-medium text-text-primary block">启用状态</label>
                      <div class="flex items-center gap-2.5 py-0.5 px-2 bg-bg-panel border border-border/60 rounded h-[30px]">
                        <el-switch
                          :model-value="topologyEnabledValue(currentOverrideField.fieldKey, currentOverrideField.enabled)"
                          @change="(val: any) => onTopologyEnabledChangeDirect(currentOverrideField!.fieldKey, val)"
                          size="small"
                        />
                        <span class="text-[11px] text-text-muted truncate">
                          {{ topologyEnabledValue(currentOverrideField.fieldKey, currentOverrideField.enabled) ? '任务中执行此字段采集' : '跳过此字段' }}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 只读基础继承属性 -->
                <div class="bg-bg-panel/40 border border-border/50 rounded-lg p-3 space-y-2 text-[11px] shrink-0">
                  <div class="flex justify-between">
                    <span class="text-text-muted">本任务解析厂商：</span>
                    <span class="font-medium text-text-primary">{{ currentOverrideField.resolvedVendor || "-" }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-text-muted">指令当前继承来源：</span>
                    <span class="font-medium text-text-primary">
                      <el-tag size="small" :type="sourceType(currentOverrideField.commandSource)" effect="dark" class="!rounded !px-1.5">
                        {{ sourceLabel(currentOverrideField.commandSource) }}
                      </el-tag>
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <!-- 缺省空白 -->
            <div v-else class="flex-1 flex flex-col items-center justify-center p-6 text-center bg-bg-secondary/10">
              <el-empty description="请在左侧选择需要覆盖配置字段" size="small" />
            </div>
          </div>
        </div>
      </div>
      
      <template #footer>
        <div class="flex justify-end gap-2 border-t border-border/60 pt-3">
          <el-button @click="showTopologyOverrideDialog = false" type="primary" class="!rounded-md">
            确定并关闭
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { Cpu, Search, RefreshLeft, InfoFilled } from "@element-plus/icons-vue";
import { TopologyCommandAPI } from "../../services/api";
import type {
  CommandGroup,
  DeviceAsset,
  TaskGroup,
  TaskItem,
  TopologyCommandPreviewView,
  TopologyTaskFieldOverride,
} from "../../services/api";
import DeviceSelectorModal from "./DeviceSelectorModal.vue";

type TaskGroupWithTopologyOverrides = TaskGroup & {
  topologyFieldOverrides?: TopologyTaskFieldOverride[];
};

type BindingItemForm = {
  deviceIDs: number[];
  commandsText: string;
};

const props = defineProps<{
  modelValue: boolean;
  task: TaskGroup | null;
  allDevices: DeviceAsset[];
  commandGroups: CommandGroup[];
  loading: boolean;
  saving: boolean;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void;
  (e: "save", payload: TaskGroup): void;
}>();

const tabs = [
  { key: 'basic', label: '基础与执行参数' },
  { key: 'config', label: '目标与指令配置' }
] as const;

const activeTab = ref<typeof tabs[number]['key']>('basic');

watch(
  () => props.modelValue,
  (value) => {
    if (value) {
      activeTab.value = 'basic';
    }
  }
);

const form = reactive({
  name: "",
  description: "",
  tags: [] as string[],
  enableRawLog: false,
});

const executionForm = reactive({
  maxWorkers: 10,
  timeout: 60,
});

const topologyForm = reactive({
  vendor: "",
  autoBuildTopology: true,
});

const groupForm = reactive({
  commandGroupId: 0,
  deviceIDs: [] as number[],
});

const backupForm = reactive({
  startupCommand: "display startup",
  saveRootPath: "",
  dirNamePattern: "%Y-%M-%D",
  fileNamePattern: "%H_startup_%h%m%s.cfg",
  sftpTimeoutSec: 0,
});

const bindingForm = reactive({
  items: [] as BindingItemForm[],
});

const newTag = ref("");
const formError = ref("");
const topologyOverrides = ref<TopologyTaskFieldOverride[]>([]);
const topologyPreview = ref<TopologyCommandPreviewView | null>(null);
const topologyPreviewLoading = ref(false);
const topologyPreviewError = ref("");
const topologyPreviewDirty = ref(false);

// 新增：任务级覆盖 Dialog 相关响应式状态
const showTopologyOverrideDialog = ref(false);
const selectedOverrideFieldKey = ref("");
const overrideSearchQuery = ref("");

// 计算属性：当前在弹窗中选中的预览字段对象
const currentOverrideField = computed(() => {
  return topologyPreviewCommands.value.find(item => item.fieldKey === selectedOverrideFieldKey.value);
});

// 计算属性：弹窗内模糊过滤后的预览字段列表
const filteredPreviewCommands = computed(() => {
  const query = overrideSearchQuery.value.trim().toLowerCase();
  if (!query) return topologyPreviewCommands.value;
  return topologyPreviewCommands.value.filter((item) => {
    const keyMatch = String(item.fieldKey || "").toLowerCase().includes(query);
    const nameMatch = String(item.displayName || "").toLowerCase().includes(query);
    const descMatch = String(item.description || "").toLowerCase().includes(query);
    return keyMatch || nameMatch || descMatch;
  });
});

// 打开覆盖弹窗，默认选中第一个有效字段
function openTopologyOverrideDialog() {
  showTopologyOverrideDialog.value = true;
  if (topologyPreviewCommands.value.length > 0) {
    if (!topologyPreviewCommands.value.some(item => item.fieldKey === selectedOverrideFieldKey.value)) {
      selectedOverrideFieldKey.value = topologyPreviewCommands.value[0]?.fieldKey || "";
    }
  } else {
    selectedOverrideFieldKey.value = "";
  }
}

// 一键清除全部任务级覆盖
function clearAllTopologyOverrides() {
  topologyOverrides.value = [];
  markTopologyPreviewDirty();
  void loadTopologyPreview();
}

// 适配 Element Plus el-input-number 修改超时时间
function onTopologyTimeoutChange(fieldKey: string, val: number | null) {
  const value = val || 0;
  const override = ensureTopologyOverride(fieldKey);
  override.timeoutSec = value > 0 ? value : 0;
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

// 适配 Element Plus el-switch 修改启用状态
function onTopologyEnabledChangeDirect(fieldKey: string, val: boolean) {
  const override = ensureTopologyOverride(fieldKey);
  override.enabled = val;
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

// 设备选择弹窗状态
const showDeviceSelector = ref(false);

// 打开设备选择弹窗
const openDeviceSelector = () => {
  showDeviceSelector.value = true;
};

// 确认设备选择
const handleDeviceConfirm = (devices: DeviceAsset[]) => {
  groupForm.deviceIDs = devices.map(d => d.id);
};

watch(
  () => [props.task, props.modelValue] as const,
  ([task, visible]) => {
    if (!visible || !task) {
      return;
    }
    hydrateForm(task);
    if (task.taskType === "topology") {
      void loadTopologyPreview();
    }
  },
  { immediate: true },
);

const selectedGroupCommands = computed(() => {
  const current = props.commandGroups.find(
    (group) => group.id === groupForm.commandGroupId,
  );
  return current?.commands ?? [];
});

const isTopologyTaskValue = computed(() => props.task?.taskType === "topology");
const isBackupTaskValue = computed(() => props.task?.taskType === "backup");

const topologyVendorOptions = computed(() => {
  const values = new Set<string>(["huawei", "h3c", "cisco"]);
  for (const device of props.allDevices as Array<any>) {
    const vendor = String(device?.vendor || "")
      .trim()
      .toLowerCase();
    if (vendor) {
      values.add(vendor);
    }
  }
  if (topologyForm.vendor) {
    values.add(topologyForm.vendor.trim().toLowerCase());
  }
  return Array.from(values).filter(Boolean).sort();
});

const selectedTopologyDevices = computed(() =>
  props.allDevices.filter((device) => groupForm.deviceIDs.includes(device.id)),
);

// 已选设备预览文本
const selectedDevicesPreview = computed(() =>
  selectedTopologyDevices.value.map(d => d.ip).join(', '),
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

watch(
  () => [
    props.modelValue,
    props.task?.taskType,
    topologyForm.vendor,
    [...groupForm.deviceIDs].sort((a, b) => a - b).join(","),
  ],
  ([visible, taskType]) => {
    if (!visible || taskType !== "topology") {
      return;
    }
    void loadTopologyPreview();
  },
);

function hydrateForm(task: TaskGroup) {
  const topologyTask = task as TaskGroupWithTopologyOverrides;
  form.name = task.name;
  form.description = task.description;
  form.tags = [...task.tags];
  form.enableRawLog = Boolean(task.enableRawLog);
  executionForm.maxWorkers = Number(task.maxWorkers || 10);
  executionForm.timeout = Number(task.timeout || 60);
  topologyForm.vendor = task.topologyVendor || "";
  topologyForm.autoBuildTopology = Boolean(task.autoBuildTopology);
  topologyOverrides.value = cloneTopologyOverrides(
    topologyTask.topologyFieldOverrides,
  );
  topologyPreview.value = null;
  topologyPreviewError.value = "";
  topologyPreviewDirty.value = false;
  newTag.value = "";
  formError.value = "";

  backupForm.startupCommand = task.backupStartupCommand || "display startup";
  backupForm.saveRootPath = task.backupSaveRootPath || "";
  backupForm.dirNamePattern = task.backupDirNamePattern || "%Y-%M-%D";
  backupForm.fileNamePattern = task.backupFileNamePattern || "%H_startup_%h%m%s.cfg";
  backupForm.sftpTimeoutSec = task.backupSftpTimeoutSec || 0;

  if (task.mode === "group") {
    const normalized = normalizeGroupTask(task.items);
    groupForm.commandGroupId = normalized.commandGroupId;
    groupForm.deviceIDs = normalized.deviceIDs;
    bindingForm.items = [];
    return;
  }

  groupForm.commandGroupId = 0;
  groupForm.deviceIDs = [];
  bindingForm.items = task.items.map((item) => ({
    deviceIDs: [...item.deviceIDs],
    commandsText: item.commands.join("\n"),
  }));

  if (bindingForm.items.length === 0) {
    bindingForm.items = [{ deviceIDs: [], commandsText: "" }];
  }
}

function normalizeGroupTask(items: TaskItem[]) {
  const deviceSet = new Set<number>();
  let commandGroupId = 0;

  items.forEach((item) => {
    if (!commandGroupId && item.commandGroupId) {
      commandGroupId = parseInt(item.commandGroupId, 10) || 0;
    }
    item.deviceIDs.forEach((id) => deviceSet.add(id));
  });

  return {
    commandGroupId,
    deviceIDs: Array.from(deviceSet),
  };
}

function closeModal() {
  emit("update:modelValue", false);
}

function addTag() {
  const tag = newTag.value.trim();
  if (tag && !form.tags.includes(tag)) {
    form.tags.push(tag);
  }
  newTag.value = "";
}

function removeTag(index: number) {
  form.tags.splice(index, 1);
}

function addBindingItem() {
  bindingForm.items.push({ deviceIDs: [], commandsText: "" });
}

function removeBindingItem(index: number) {
  if (bindingForm.items.length === 1) {
    bindingForm.items[0] = { deviceIDs: [], commandsText: "" };
    return;
  }
  bindingForm.items.splice(index, 1);
}

function toggleBindingDevice(index: number, deviceID: number) {
  const item = bindingForm.items[index];
  if (!item) return;

  if (item.deviceIDs.includes(deviceID)) {
    item.deviceIDs.splice(item.deviceIDs.indexOf(deviceID), 1);
    return;
  }

  item.deviceIDs.push(deviceID);
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



function resetTopologyOverride(fieldKey: string) {
  topologyOverrides.value = topologyOverrides.value.filter(
    (item: TopologyTaskFieldOverride) => item.fieldKey !== fieldKey,
  );
  markTopologyPreviewDirty();
  void loadTopologyPreview();
}

function findDevicePreview(deviceID: number) {
  return topologyPreview.value?.devices?.find(
    (item: { deviceId: number }) => item.deviceId === deviceID,
  );
}

async function loadTopologyPreview() {
  if (!props.modelValue || !isTopologyTaskValue.value) {
    return;
  }
  if (groupForm.deviceIDs.length === 0) {
    topologyPreview.value = null;
    topologyPreviewError.value = "";
    topologyPreviewDirty.value = false;
    return;
  }
  topologyPreviewLoading.value = true;
  topologyPreviewError.value = "";
  try {
    const nextPreview = await TopologyCommandAPI.previewTopologyCommands(
      topologyForm.vendor,
      [...groupForm.deviceIDs],
      cloneTopologyOverrides(topologyOverrides.value),
    );
    topologyPreview.value = nextPreview;
    topologyOverrides.value = cloneTopologyOverrides(
      nextPreview?.taskOverrides || [],
    );
    topologyPreviewDirty.value = false;
  } catch (err: any) {
    topologyPreviewError.value = `命令预览加载失败: ${err?.message || err}`;
  } finally {
    topologyPreviewLoading.value = false;
  }
}

function submit() {
  if (!props.task) return;

  const name = form.name.trim();
  if (!name) {
    formError.value = "任务名称不能为空";
    return;
  }

  const tags = form.tags
    .map((tag) => tag.trim())
    .filter((tag, index, array) => tag !== "" && array.indexOf(tag) === index);

  let items: TaskItem[] = [];
  if (executionForm.maxWorkers <= 0) {
    formError.value = "最大并发必须大于 0";
    return;
  }
  if (executionForm.timeout <= 0) {
    formError.value = "超时时间必须大于 0";
    return;
  }

  if (props.task.mode === "group") {
    if (groupForm.deviceIDs.length === 0) {
      formError.value = "请至少选择一台设备";
      return;
    }

    if (isTopologyTaskValue.value) {
      if (topologyInvalidCount.value > 0) {
        formError.value = "存在已启用但命令为空的拓扑覆盖项，请先修正";
        return;
      }
      if (topologyPreviewDirty.value) {
        formError.value = "拓扑命令存在未刷新的变更，请先刷新命令预览";
        return;
      }
      items = [
        {
          commandGroupId: props.task.items?.[0]?.commandGroupId || "",
          commands: props.task.items?.[0]?.commands
            ? [...props.task.items[0].commands]
            : [],
          deviceIDs: [...groupForm.deviceIDs],
        },
      ];
    } else if (isBackupTaskValue.value) {
      if (!backupForm.startupCommand.trim()) {
        formError.value = "配置查询命令不能为空";
        return;
      }
      if (!backupForm.saveRootPath.trim()) {
        formError.value = "根保存路径不能为空";
        return;
      }
      items = [
        {
          commandGroupId: "",
          commands: [],
          deviceIDs: [...groupForm.deviceIDs],
        },
      ];
    } else {
      if (!groupForm.commandGroupId) {
        formError.value = "请选择命令组";
        return;
      }

      items = [
        {
          commandGroupId: String(groupForm.commandGroupId),
          commands: [],
          deviceIDs: [...groupForm.deviceIDs],
        },
      ];
    }
  } else {
    items = bindingForm.items
      .map((item) => ({
        commandGroupId: "",
        commands: item.commandsText
          .split("\n")
          .map((line) => line.trim())
          .filter((line) => line !== ""),
        deviceIDs: [...item.deviceIDs],
      }))
      .filter((item) => item.deviceIDs.length > 0 && item.commands.length > 0);

    if (items.length === 0) {
      formError.value = "请至少保留一个包含设备和命令的任务项";
      return;
    }
  }

  formError.value = "";
  const taskWithTopology = props.task as TaskGroupWithTopologyOverrides;
  const payload: TaskGroupWithTopologyOverrides = {
    id: props.task.id,
    name,
    description: form.description.trim(),
    deviceGroup: props.task.deviceGroup,
    commandGroup: props.task.commandGroup,
    maxWorkers: executionForm.maxWorkers,
    timeout: executionForm.timeout,
    taskType: props.task.taskType,
    topologyVendor: isTopologyTaskValue.value
      ? topologyForm.vendor
      : props.task.topologyVendor,
    topologyFieldOverrides: isTopologyTaskValue.value
      ? cloneTopologyOverrides(topologyOverrides.value)
      : taskWithTopology.topologyFieldOverrides
        ? [...taskWithTopology.topologyFieldOverrides]
        : [],
    autoBuildTopology: isTopologyTaskValue.value
      ? topologyForm.autoBuildTopology
      : props.task.autoBuildTopology,
    mode: props.task.mode,
    items,
    tags,
    enableRawLog: form.enableRawLog,
    backupSaveRootPath: backupForm.saveRootPath.trim(),
    backupDirNamePattern: backupForm.dirNamePattern.trim(),
    backupFileNamePattern: backupForm.fileNamePattern.trim(),
    backupStartupCommand: backupForm.startupCommand.trim(),
    backupSftpTimeoutSec: backupForm.sftpTimeoutSec || 0,
    status: "",
    createdAt: props.task.createdAt,
    updatedAt: props.task.updatedAt,
  };
  emit("save", payload);
}

function sourceLabel(source: string) {
  switch (source) {
    case "vendor_config":
      return "厂商配置";
    case "profile_seed":
      return "画像种子";
    case "field_default":
      return "字段默认";
    case "task_override":
      return "任务覆盖";
    default:
      return source || "未知";
  }
}

function sourceType(source: string) {
  switch (source) {
    case "vendor_config": return "success";
    case "profile_seed": return "info";
    case "task_override": return "danger";
    default: return "info";
  }
}

function modeLabel(mode: string) {
  return mode === "group" ? "模式A" : mode === "binding" ? "模式B" : mode;
}
</script>

<style scoped>
/* modal 过渡动画已移至全局 _animations.css */
/* 终端颜色类已移至全局 index.css */
</style>
