<template>
  <div class="h-full flex flex-col space-y-4 animate-slide-in">
    <!-- 顶部标题栏与全局动作 -->
    <div class="flex items-center justify-between bg-bg-panel border border-border px-6 py-4 rounded-xl shadow-card">
      <div>
        <div class="flex items-center gap-3">
          <h1 class="text-xl font-bold text-text-primary tracking-tight">硬件清单与批次预警 (CEAS)</h1>
          <el-tag type="primary" effect="plain" size="small" class="font-mono">P3 Complete</el-tag>
        </div>
        <p class="text-xs text-text-muted mt-1">
          电子标签层级化解析 · 框/板/卡/端口拓扑树 · 华为 BOM 批次安全隐患联动预警
        </p>
      </div>
      <div class="flex items-center gap-3">
        <el-button type="primary" :icon="VideoPlay" @click="openCollectModal">
          新建采集任务
        </el-button>
        <el-button :icon="Refresh" @click="handleRefreshAll" :loading="loading">
          刷新
        </el-button>
      </div>
    </div>

    <!-- 核心标签页切换 -->
    <div class="flex-1 min-h-0 bg-bg-panel border border-border rounded-xl shadow-card flex flex-col overflow-hidden">
      <el-tabs v-model="activeTab" class="h-full flex flex-col custom-tabs" @tab-change="handleTabChange">
        <!-- 标签页 1: 硬件拓扑树 -->
        <el-tab-pane label="硬件拓扑树" name="tree" class="h-full">
          <div class="h-full flex divide-x divide-border">
            <!-- 左侧：设备列表 -->
            <div class="w-80 flex flex-col h-full bg-bg-panel/50">
              <div class="p-3 border-b border-border space-y-2">
                <el-input
                  v-model="deviceSearchQuery"
                  placeholder="搜索 IP / 厂商 / 款型..."
                  :prefix-icon="Search"
                  clearable
                  size="small"
                />
                <div class="flex items-center justify-between text-xs text-text-muted px-1">
                  <span>设备 ({{ filteredDevices.length }})</span>
                  <span>{{ collectedDeviceCount }} 台已采集</span>
                </div>
              </div>

              <!-- 设备列表滚动区 -->
              <div class="flex-1 overflow-y-auto divide-y divide-border/60 scrollbar-custom">
                <div
                  v-for="dev in filteredDevices"
                  :key="dev.ip"
                  class="p-3.5 cursor-pointer transition-colors duration-150 hover:bg-bg-hover"
                  :class="{ 'bg-accent/10 border-l-4 border-l-accent': selectedDeviceIP === dev.ip }"
                  @click="selectDevice(dev.ip)"
                >
                  <div class="flex items-center justify-between mb-1.5">
                    <span class="font-mono text-sm font-semibold text-text-primary">{{ dev.ip }}</span>
                    <el-tag
                      size="small"
                      :type="dev.hasCeasData ? 'success' : 'info'"
                      effect="light"
                      class="text-[11px]"
                    >
                      {{ dev.hasCeasData ? `${dev.nodeCount} 节点` : '未采集' }}
                    </el-tag>
                  </div>
                  <div class="flex items-center justify-between text-xs text-text-muted">
                    <div class="flex items-center gap-1.5">
                      <span class="capitalize font-medium">{{ dev.vendor || '通用' }}</span>
                      <span v-if="dev.model" class="text-text-secondary">· {{ dev.model }}</span>
                    </div>
                    <el-tag
                      v-if="dev.alertCount > 0"
                      type="danger"
                      size="small"
                      effect="dark"
                      class="font-mono text-[10px] px-1.5 py-0 h-4"
                    >
                      {{ dev.alertCount }} 预警
                    </el-tag>
                  </div>
                </div>

                <div v-if="filteredDevices.length === 0" class="p-8 text-center text-text-muted text-xs">
                  未匹配到相关设备
                </div>
              </div>
            </div>

            <!-- 右侧：硬件树拓扑与详情 -->
            <div class="flex-1 flex flex-col h-full min-w-0 bg-bg-panel">
              <template v-if="selectedDevice">
                <!-- 设备基本信息卡片条 -->
                <div class="p-4 border-b border-border bg-bg-panel flex items-center justify-between flex-wrap gap-3">
                  <div class="flex items-center gap-6">
                    <div>
                      <div class="text-xs text-text-muted">设备 IP</div>
                      <div class="font-mono font-bold text-accent text-base">{{ selectedDevice.ip }}</div>
                    </div>
                    <div>
                      <div class="text-xs text-text-muted">设备款型 / 系列</div>
                      <div class="text-sm font-medium text-text-primary">
                        {{ selectedDevice.model || '-' }}
                        <span v-if="selectedDevice.modelSeries" class="text-xs text-text-muted">({{ selectedDevice.modelSeries }})</span>
                      </div>
                    </div>
                    <div>
                      <div class="text-xs text-text-muted">主机序列号 (ESN)</div>
                      <div class="font-mono text-sm text-text-secondary flex items-center gap-1">
                        {{ currentTree?.chassisEsn || selectedDevice.esn || '-' }}
                        <el-button
                          v-if="currentTree?.chassisEsn || selectedDevice.esn"
                          link
                          type="primary"
                          :icon="DocumentCopy"
                          size="small"
                          @click="copyText(currentTree?.chassisEsn || selectedDevice.esn)"
                          title="复制序列号"
                        />
                      </div>
                    </div>
                    <div>
                      <div class="text-xs text-text-muted">硬件节点总数</div>
                      <div class="font-mono text-sm font-semibold text-text-primary">{{ currentTree?.totalNodes || 0 }}</div>
                    </div>
                    <div v-if="selectedDevice.alertCount > 0">
                      <div class="text-xs text-text-muted">BOM 预警数</div>
                      <div class="font-mono text-sm font-bold text-danger">{{ selectedDevice.alertCount }} 处隐患</div>
                    </div>
                  </div>

                  <div class="flex items-center gap-2">
                    <el-button size="small" :icon="Download" @click="exportCurrentDeviceCSV" :disabled="!currentTree?.totalNodes">
                      导出硬件清单 CSV
                    </el-button>
                    <el-button size="small" type="primary" plain :icon="VideoPlay" @click="triggerSingleCollect(selectedDevice.ip)">
                      重新采集
                    </el-button>
                  </div>
                </div>

                <!-- 树与详情分栏 -->
                <div v-if="currentTree && currentTree.roots && currentTree.roots.length > 0" class="flex-1 flex min-h-0 divide-x divide-border">
                  <!-- 树组件区域 -->
                  <div class="w-1/2 p-4 flex flex-col h-full overflow-hidden">
                    <div class="mb-3">
                      <el-input
                        v-model="treeFilterText"
                        placeholder="过滤节点 (名称 / BOM / 槽位 / 端口)..."
                        :prefix-icon="Search"
                        clearable
                        size="small"
                      />
                    </div>
                    <div class="flex-1 overflow-auto scrollbar-custom border border-border/70 rounded-lg p-2 bg-bg-panel/40">
                      <el-tree
                        ref="treeRef"
                        :data="currentTree.roots"
                        :props="{ label: 'name', children: 'children' }"
                        node-key="id"
                        highlight-current
                        default-expand-all
                        :filter-node-method="filterTreeNode"
                        @node-click="handleNodeClick"
                      >
                        <template #default="{ data }">
                          <div class="flex items-center justify-between w-full py-0.5 pr-2 gap-2 text-xs">
                            <div class="flex items-center gap-1.5 min-w-0">
                              <el-tag size="small" :type="getNodeTypeTag(data.type)" effect="plain" class="scale-90 origin-left">
                                {{ data.type }}
                              </el-tag>
                              <span class="font-medium text-text-primary truncate">{{ data.name }}</span>
                              <span v-if="data.slot" class="text-[11px] text-text-muted font-mono">#{{ data.slot }}</span>
                              <span v-if="data.path" class="text-[11px] text-accent font-mono">({{ data.path }})</span>
                            </div>
                            <div class="flex items-center gap-1 flex-shrink-0">
                              <el-tag v-if="isBOMHit(data.item)" type="danger" effect="dark" size="small" class="h-4 text-[10px] px-1 font-mono animate-pulse">
                                BOM 预警
                              </el-tag>
                              <span v-if="data.item" class="text-[11px] text-text-secondary font-mono">{{ data.item }}</span>
                            </div>
                          </div>
                        </template>
                      </el-tree>
                    </div>
                  </div>

                  <!-- 节点详情属性面板 -->
                  <div class="w-1/2 p-5 flex flex-col h-full overflow-y-auto scrollbar-custom">
                    <template v-if="selectedNode">
                      <div class="flex items-center justify-between mb-4">
                        <div class="flex items-center gap-2">
                          <el-tag :type="getNodeTypeTag(selectedNode.type)" effect="light">
                            {{ selectedNode.type }}
                          </el-tag>
                          <h2 class="text-base font-bold text-text-primary">{{ selectedNode.name }}</h2>
                        </div>
                        <el-tag v-if="selectedNode.level" size="small" type="info" effect="plain">
                          第 {{ selectedNode.level }} 级
                        </el-tag>
                      </div>

                      <!-- 命中预警横幅 -->
                      <div v-if="getNodeAlert(selectedNode.item)" class="mb-4 p-3 bg-danger/10 border border-danger/30 rounded-lg flex items-start gap-2.5">
                        <el-icon class="text-danger text-lg mt-0.5"><WarningFilled /></el-icon>
                        <div class="text-xs">
                          <div class="font-bold text-danger flex items-center gap-2">
                            <span>BOM 批次隐患命中</span>
                            <el-tag size="small" type="danger" effect="dark">
                              {{ getNodeAlert(selectedNode.item)?.severity?.toUpperCase() }}
                            </el-tag>
                            <span v-if="getNodeAlert(selectedNode.item)?.batchNo" class="font-mono text-text-muted">
                              {{ getNodeAlert(selectedNode.item)?.batchNo }}
                            </span>
                          </div>
                          <div class="text-text-primary mt-1">
                            {{ getNodeAlert(selectedNode.item)?.description || '该物料已被列入批次安全预警清单，请注意核实硬件质量情况。' }}
                          </div>
                        </div>
                      </div>

                      <!-- 核心属性表 -->
                      <div class="bg-bg-panel/60 border border-border rounded-lg overflow-hidden mb-4">
                        <table class="w-full text-xs">
                          <tbody class="divide-y divide-border">
                            <tr>
                              <td class="w-28 px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">BOM 编码</td>
                              <td class="px-3 py-2 font-mono font-bold text-accent">{{ selectedNode.item || '-' }}</td>
                            </tr>
                            <tr>
                              <td class="px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">条形码 / 电子标签号</td>
                              <td class="px-3 py-2 font-mono text-text-primary">{{ selectedNode.barCode || '-' }}</td>
                            </tr>
                            <tr>
                              <td class="px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">单板型号 (BoardType)</td>
                              <td class="px-3 py-2 text-text-primary">{{ selectedNode.boardType || '-' }}</td>
                            </tr>
                            <tr>
                              <td class="px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">物理位置 (Path)</td>
                              <td class="px-3 py-2 font-mono text-text-primary">{{ selectedNode.path || '-' }}</td>
                            </tr>
                            <tr>
                              <td class="px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">归属主槽位 (Slot)</td>
                              <td class="px-3 py-2 font-mono text-text-primary">{{ selectedNode.slot || '-' }}</td>
                            </tr>
                            <tr>
                              <td class="px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">生产日期</td>
                              <td class="px-3 py-2 text-text-secondary">{{ selectedNode.manufactured || '-' }}</td>
                            </tr>
                            <tr>
                              <td class="px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">厂商名称</td>
                              <td class="px-3 py-2 text-text-secondary">{{ selectedNode.vendorName || '-' }}</td>
                            </tr>
                            <tr>
                              <td class="px-3 py-2 text-text-muted bg-bg-panel/40 font-medium">物料详细描述</td>
                              <td class="px-3 py-2 text-text-secondary leading-relaxed">{{ selectedNode.description || '-' }}</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>

                      <!-- 扩展属性 -->
                      <div v-if="selectedNode.attrs && Object.keys(selectedNode.attrs).length > 0">
                        <div class="text-xs font-semibold text-text-muted uppercase tracking-wider mb-2">扩展属性 (Attrs)</div>
                        <div class="grid grid-cols-2 gap-2">
                          <div
                            v-for="(val, key) in selectedNode.attrs"
                            :key="key"
                            class="p-2 bg-bg-panel/40 border border-border/80 rounded flex flex-col"
                          >
                            <span class="text-[10px] text-text-muted truncate" :title="String(key)">{{ key }}</span>
                            <span class="text-xs font-mono text-text-primary truncate" :title="String(val)">{{ val }}</span>
                          </div>
                        </div>
                      </div>
                    </template>

                    <div v-else class="h-full flex flex-col items-center justify-center text-text-muted text-xs">
                      <el-icon class="text-3xl mb-2"><InfoFilled /></el-icon>
                      <span>在左侧硬件树中点击节点查看完整物料与属性明细</span>
                    </div>
                  </div>
                </div>

                <!-- 无硬件数据提示 -->
                <div v-else class="flex-1 flex flex-col items-center justify-center p-12 text-center">
                  <el-empty description="该设备暂无已采集的电子标签硬件数据">
                    <el-button type="primary" :icon="VideoPlay" @click="triggerSingleCollect(selectedDevice.ip)">
                      立即执行硬件清单采集
                    </el-button>
                  </el-empty>
                </div>
              </template>

              <!-- 未选择设备提示 -->
              <div v-else class="h-full flex flex-col items-center justify-center p-12 text-center text-text-muted">
                <el-icon class="text-4xl mb-3 text-text-muted/60"><Platform /></el-icon>
                <div class="text-sm font-medium">请在左侧列表中选择一台设备</div>
                <div class="text-xs mt-1">查看其机框、单板、子卡与端口等层级拓扑</div>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <!-- 标签页 2: BOM 批次预警矩阵 -->
        <el-tab-pane label="BOM 批次预警矩阵" name="alerts" class="h-full">
          <div class="h-full flex flex-col p-5 space-y-4 overflow-hidden">
            <!-- 统计卡片与筛选行 -->
            <div class="flex items-center justify-between flex-wrap gap-4">
              <div class="flex items-center gap-3">
                <el-input
                  v-model="alertSearchItem"
                  placeholder="搜索 BOM / 序列号 / 设备..."
                  :prefix-icon="Search"
                  clearable
                  size="small"
                  class="w-64"
                />
                <el-select v-model="alertSeverityFilter" placeholder="严重度筛选" clearable size="small" class="w-36">
                  <el-option label="全部等级" value="" />
                  <el-option label="Critical (严重)" value="critical" />
                  <el-option label="Danger (危险)" value="danger" />
                  <el-option label="Warning (警告)" value="warning" />
                </el-select>
              </div>

              <div class="flex items-center gap-3">
                <div class="flex items-center gap-2 text-xs font-medium">
                  <span class="text-text-muted">命中总计:</span>
                  <el-tag type="danger" effect="dark" size="small">{{ filteredAlerts.length }} 处</el-tag>
                </div>
                <el-button :icon="Download" size="small" @click="exportAlertsCSV">
                  导出预警报表 CSV
                </el-button>
              </div>
            </div>

            <!-- 预警表格 -->
            <div class="flex-1 min-h-0 border border-border rounded-xl overflow-hidden shadow-sm bg-bg-panel">
              <el-table
                :data="filteredAlerts"
                v-loading="loadingAlerts"
                height="100%"
                stripe
                size="small"
                class="w-full"
              >
                <el-table-column label="风险等级" width="110" align="center">
                  <template #default="{ row }">
                    <el-tag :type="getSeverityTagType(row.severity)" effect="dark" size="small" class="uppercase font-mono">
                      {{ row.severity }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="deviceIp" label="设备 IP" width="140">
                  <template #default="{ row }">
                    <span class="font-mono text-accent font-semibold cursor-pointer hover:underline" @click="jumpToDeviceTree(row.deviceIp)">
                      {{ row.deviceIp }}
                    </span>
                  </template>
                </el-table-column>
                <el-table-column prop="slot" label="槽位" width="80" align="center">
                  <template #default="{ row }">
                    <span class="font-mono">#{{ row.slot || '-' }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="path" label="物理位置 (Path)" width="130">
                  <template #default="{ row }">
                    <span class="font-mono text-text-secondary">{{ row.path || '-' }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="nodeName" label="节点名称" width="150" />
                <el-table-column prop="item" label="命中 BOM 编码" width="140">
                  <template #default="{ row }">
                    <span class="font-mono font-bold text-danger">{{ row.item }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="barCode" label="序列号 / 条形码" width="180">
                  <template #default="{ row }">
                    <span class="font-mono text-text-muted text-xs">{{ row.barCode || '-' }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="batchNo" label="批次编号" width="140">
                  <template #default="{ row }">
                    <span class="font-mono text-text-secondary">{{ row.batchNo || '-' }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="description" label="隐患说明 / 处置建议" min-width="240" show-overflow-tooltip />
              </el-table>
            </div>
          </div>
        </el-tab-pane>

        <!-- 标签页 3: BOM 观察清单维护 -->
        <el-tab-pane label="BOM 观察清单维护" name="watchlist" class="h-full">
          <div class="h-full flex flex-col p-5 space-y-4 overflow-hidden">
            <!-- 筛选与添加 -->
            <div class="flex items-center justify-between flex-wrap gap-4">
              <div class="flex items-center gap-3">
                <el-input
                  v-model="watchlistSearch"
                  placeholder="搜索 BOM 编码 / 部件 / 描述..."
                  :prefix-icon="Search"
                  clearable
                  size="small"
                  class="w-64"
                />
                <el-select v-model="watchlistCategoryFilter" placeholder="产品类别" clearable size="small" class="w-36">
                  <el-option label="全部类别" value="" />
                  <el-option label="AR 路由物料" value="ar_items" />
                  <el-option label="园区交换物料" value="s_items" />
                  <el-option label="数据中心物料" value="ce_items" />
                  <el-option label="核心路由物料" value="ne_items" />
                  <el-option label="自定义物料" value="user_custom" />
                </el-select>
              </div>

              <div class="flex items-center gap-3">
                <span class="text-xs text-text-muted">已收录 {{ filteredWatchlist.length }} 条规则</span>
                <el-button type="primary" :icon="Plus" size="small" @click="openAddWatchlistModal">
                  新增观察物料
                </el-button>
              </div>
            </div>

            <!-- 清单表格 -->
            <div class="flex-1 min-h-0 border border-border rounded-xl overflow-hidden shadow-sm bg-bg-panel">
              <el-table
                :data="filteredWatchlist"
                v-loading="loadingWatchlist"
                height="100%"
                stripe
                size="small"
                class="w-full"
              >
                <el-table-column prop="item" label="BOM 编码" width="140">
                  <template #default="{ row }">
                    <span class="font-mono font-bold text-accent">{{ row.item }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="category" label="分类来源" width="130">
                  <template #default="{ row }">
                    <el-tag size="small" effect="plain">{{ getCategoryLabel(row.category) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="model" label="款型 / 部件名称" width="160" show-overflow-tooltip />
                <el-table-column prop="severity" label="风险等级" width="110" align="center">
                  <template #default="{ row }">
                    <el-tag :type="getSeverityTagType(row.severity)" size="small" effect="dark" class="uppercase font-mono">
                      {{ row.severity }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="batchNo" label="批次 / 通告号" width="150" />
                <el-table-column prop="description" label="隐患预警说明" min-width="220" show-overflow-tooltip />
                <el-table-column label="启用状态" width="100" align="center">
                  <template #default="{ row }">
                    <el-switch
                      v-model="row.enabled"
                      size="small"
                      @change="handleToggleWatchlist(row)"
                    />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="120" align="center" fixed="right">
                  <template #default="{ row }">
                    <div class="flex items-center justify-center gap-1">
                      <el-button link type="primary" size="small" @click="openEditWatchlistModal(row)">
                        编辑
                      </el-button>
                      <el-button link type="danger" size="small" @click="handleDeleteWatchlist(row)">
                        删除
                      </el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 弹窗 1: 新建采集任务弹窗 -->
    <el-dialog
      v-model="collectModalVisible"
      title="新建 CEAS 电子标签硬件采集任务"
      width="560px"
      append-to-body
      destroy-on-close
    >
      <div class="space-y-4">
        <div class="text-xs text-text-muted leading-relaxed">
          将向选中的设备下发 <code class="font-mono bg-bg-panel px-1 py-0.5 rounded text-accent">display elabel</code>（必要时提权至 diagnose 视图）并抓取 ESN，结构化提取硬件树与匹配 BOM 批次安全隐患。
        </div>

        <div>
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs font-semibold text-text-primary">选择采集目标设备</span>
            <div class="flex gap-2">
              <el-button link type="primary" size="small" @click="selectAllDevices">全选</el-button>
              <el-button link type="primary" size="small" @click="selectUncollectedDevices">仅未采集</el-button>
              <el-button link type="info" size="small" @click="clearSelectedDevices">清空</el-button>
            </div>
          </div>
          <div class="max-h-56 overflow-y-auto border border-border rounded-lg p-3 divide-y divide-border/60 scrollbar-custom bg-bg-panel/40">
            <el-checkbox-group v-model="selectedCollectIPs" class="flex flex-col gap-2">
              <el-checkbox
                v-for="d in devices"
                :key="d.ip"
                :label="d.ip"
                class="!mr-0 w-full"
              >
                <div class="flex items-center justify-between w-full text-xs">
                  <span class="font-mono font-medium">{{ d.ip }}</span>
                  <span class="text-text-muted">({{ d.vendor || '通用' }} {{ d.model }})</span>
                </div>
              </el-checkbox>
            </el-checkbox-group>
          </div>
          <div class="text-right text-[11px] text-text-muted mt-1">
            已选择 {{ selectedCollectIPs.length }} 台设备
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="collectModalVisible = false">取消</el-button>
        <el-button type="primary" :loading="submittingCollect" :disabled="selectedCollectIPs.length === 0" @click="submitCollectTask">
          立即执行采集
        </el-button>
      </template>
    </el-dialog>

    <!-- 弹窗 2: BOM 观察清单编辑弹窗 -->
    <el-dialog
      v-model="watchlistModalVisible"
      :title="editingWatchlist.id ? '编辑 BOM 预警物料' : '新增 BOM 预警物料'"
      width="480px"
      append-to-body
      destroy-on-close
    >
      <el-form label-position="top" size="small" class="space-y-3">
        <el-form-item label="BOM 编码 *" required>
          <el-input v-model="editingWatchlist.item" placeholder="如 02311UHE" />
        </el-form-item>
        <el-form-item label="所属类别 / 产品线">
          <el-select v-model="editingWatchlist.category" class="w-full">
            <el-option label="自定义物料 (user_custom)" value="user_custom" />
            <el-option label="AR 路由物料 (ar_items)" value="ar_items" />
            <el-option label="园区交换物料 (s_items)" value="s_items" />
            <el-option label="数据中心物料 (ce_items)" value="ce_items" />
            <el-option label="核心路由物料 (ne_items)" value="ne_items" />
          </el-select>
        </el-form-item>
        <el-form-item label="对应部件 / 款型名称">
          <el-input v-model="editingWatchlist.model" placeholder="如 CE6800 主板" />
        </el-form-item>
        <el-form-item label="风险等级">
          <el-radio-group v-model="editingWatchlist.severity">
            <el-radio-button label="critical">Critical</el-radio-button>
            <el-radio-button label="danger">Danger</el-radio-button>
            <el-radio-button label="warning">Warning</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="批次通知号 (BatchNo)">
          <el-input v-model="editingWatchlist.batchNo" placeholder="如 PCN-2026-001" />
        </el-form-item>
        <el-form-item label="预警隐患描述 / 处置建议">
          <el-input
            v-model="editingWatchlist.description"
            type="textarea"
            :rows="3"
            placeholder="说明批次原因与处置建议..."
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="watchlistModalVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingWatchlist" @click="submitSaveWatchlist">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  VideoPlay,
  Refresh,
  Search,
  Download,
  DocumentCopy,
  WarningFilled,
  InfoFilled,
  Platform,
  Plus
} from '@element-plus/icons-vue';
import {
  HardwareInventoryAPI,
  type CEASDeviceOverviewVO,
  type HardwareTreeVO,
  type NodeVO,
  type BOMAlertVO,
  type BOMWatchlistItem
} from '@/services/hardwareInventoryApi';

const route = useRoute();
const router = useRouter();

// 全局状态
const activeTab = ref('tree');
const loading = ref(false);
const devices = ref<CEASDeviceOverviewVO[]>([]);
const selectedDeviceIP = ref('');
const currentTree = ref<HardwareTreeVO | null>(null);
const selectedNode = ref<NodeVO | null>(null);
const treeFilterText = ref('');
const treeRef = ref();

// 搜索与过滤
const deviceSearchQuery = ref('');
const alertSearchItem = ref('');
const alertSeverityFilter = ref('');
const watchlistSearch = ref('');
const watchlistCategoryFilter = ref('');

// 数据列表
const alerts = ref<BOMAlertVO[]>([]);
const watchlist = ref<BOMWatchlistItem[]>([]);
const loadingAlerts = ref(false);
const loadingWatchlist = ref(false);

// 弹窗状态
const collectModalVisible = ref(false);
const selectedCollectIPs = ref<string[]>([]);
const submittingCollect = ref(false);

const watchlistModalVisible = ref(false);
const savingWatchlist = ref(false);
const editingWatchlist = ref<Partial<BOMWatchlistItem>>({
  item: '',
  category: 'user_custom',
  model: '',
  severity: 'warning',
  batchNo: '',
  description: '',
  enabled: true
});

// 计算属性
const filteredDevices = computed(() => {
  const q = deviceSearchQuery.value.trim().toLowerCase();
  if (!q) return devices.value;
  return devices.value.filter(d =>
    d.ip.toLowerCase().includes(q) ||
    (d.vendor && d.vendor.toLowerCase().includes(q)) ||
    (d.model && d.model.toLowerCase().includes(q)) ||
    (d.groupName && d.groupName.toLowerCase().includes(q))
  );
});

const collectedDeviceCount = computed(() => {
  return devices.value.filter(d => d.hasCeasData).length;
});

const selectedDevice = computed(() => {
  return devices.value.find(d => d.ip === selectedDeviceIP.value) || null;
});

const filteredAlerts = computed(() => {
  let list = alerts.value;
  if (alertSeverityFilter.value) {
    list = list.filter(a => a.severity === alertSeverityFilter.value);
  }
  const q = alertSearchItem.value.trim().toLowerCase();
  if (q) {
    list = list.filter(a =>
      a.item.toLowerCase().includes(q) ||
      a.deviceIp.toLowerCase().includes(q) ||
      (a.barCode && a.barCode.toLowerCase().includes(q)) ||
      (a.description && a.description.toLowerCase().includes(q))
    );
  }
  return list;
});

const filteredWatchlist = computed(() => {
  let list = watchlist.value;
  if (watchlistCategoryFilter.value) {
    list = list.filter(w => w.category === watchlistCategoryFilter.value);
  }
  const q = watchlistSearch.value.trim().toLowerCase();
  if (q) {
    list = list.filter(w =>
      w.item.toLowerCase().includes(q) ||
      (w.model && w.model.toLowerCase().includes(q)) ||
      (w.description && w.description.toLowerCase().includes(q))
    );
  }
  return list;
});

// Watch 树过滤
watch(treeFilterText, (val) => {
  if (treeRef.value) {
    treeRef.value.filter(val);
  }
});

// 初始化
onMounted(async () => {
  await loadDevices();
  await loadWatchlist();
  await loadAlerts();

  // 若 URL 查询参数带有特定 IP，优先定位
  const targetIP = route.query.ip as string;
  if (targetIP) {
    selectDevice(targetIP);
  } else if (devices.value.length > 0) {
    // 默认选中第一台已有数据的设备
    const firstCollected = devices.value.find(d => d.hasCeasData);
    if (firstCollected) {
      selectDevice(firstCollected.ip);
    } else if (devices.value.length > 0 && devices.value[0]) {
      selectDevice(devices.value[0].ip);
    }
  }
});

// 方法实现
async function loadDevices() {
  loading.value = true;
  try {
    devices.value = await HardwareInventoryAPI.listCEASDevices();
  } catch (err: any) {
    ElMessage.error(`加载设备列表失败: ${err.message || err}`);
  } finally {
    loading.value = false;
  }
}

async function loadWatchlist() {
  loadingWatchlist.value = true;
  try {
    watchlist.value = await HardwareInventoryAPI.listBOMWatchlist();
  } catch (err: any) {
    ElMessage.error(`加载 BOM 观察清单失败: ${err.message || err}`);
  } finally {
    loadingWatchlist.value = false;
  }
}

async function loadAlerts() {
  loadingAlerts.value = true;
  try {
    alerts.value = await HardwareInventoryAPI.listBOMAlerts('', '');
  } catch (err: any) {
    ElMessage.error(`加载 BOM 预警失败: ${err.message || err}`);
  } finally {
    loadingAlerts.value = false;
  }
}

async function selectDevice(ip: string) {
  selectedDeviceIP.value = ip;
  selectedNode.value = null;
  treeFilterText.value = '';
  try {
    currentTree.value = await HardwareInventoryAPI.getHardwareTree(ip);
    if (currentTree.value && currentTree.value.roots && currentTree.value.roots.length > 0) {
      selectedNode.value = (currentTree.value.roots[0] || null) as NodeVO | null;
    }
  } catch (err: any) {
    ElMessage.error(`加载设备硬件树失败: ${err.message || err}`);
  }
}

function handleNodeClick(data: NodeVO) {
  selectedNode.value = data;
}

function filterTreeNode(value: string, data: NodeVO): boolean {
  if (!value) return true;
  const q = value.toLowerCase();
  return Boolean(
    (data.name && data.name.toLowerCase().includes(q)) ||
    (data.item && data.item.toLowerCase().includes(q)) ||
    (data.slot && data.slot.toLowerCase().includes(q)) ||
    (data.path && data.path.toLowerCase().includes(q)) ||
    (data.barCode && data.barCode.toLowerCase().includes(q))
  );
}

function isBOMHit(item?: string): boolean {
  if (!item) return false;
  const norm = item.trim().toUpperCase();
  return watchlist.value.some(w => w.enabled && w.item.trim().toUpperCase() === norm);
}

function getNodeAlert(item?: string): BOMWatchlistItem | undefined {
  if (!item) return undefined;
  const norm = item.trim().toUpperCase();
  return watchlist.value.find(w => w.enabled && w.item.trim().toUpperCase() === norm);
}

function getNodeTypeTag(type: string): '' | 'primary' | 'success' | 'warning' | 'info' | 'danger' {
  switch (type?.toLowerCase()) {
    case 'frame':
      return 'primary';
    case 'slot':
      return 'success';
    case 'card':
    case 'mainboard':
    case 'motherboard':
      return 'warning';
    case 'daughterboard':
    case 'subcard':
      return 'info';
    case 'power':
      return 'danger';
    case 'fanframe':
      return 'info';
    default:
      return 'info';
  }
}

function getSeverityTagType(sev: string): '' | 'primary' | 'success' | 'warning' | 'info' | 'danger' {
  switch (sev?.toLowerCase()) {
    case 'critical':
      return 'danger';
    case 'danger':
      return 'warning';
    case 'warning':
      return 'info';
    default:
      return 'info';
  }
}

function getCategoryLabel(cat: string): string {
  const map: Record<string, string> = {
    ar_items: 'AR 路由',
    s_items: '园区交换',
    ce_items: '数据中心',
    ne_items: '核心路由',
    user_custom: '自定义'
  };
  return map[cat] || cat || '默认';
}

function copyText(text?: string) {
  if (!text) return;
  navigator.clipboard.writeText(text);
  ElMessage.success('已复制到剪贴板');
}

async function handleRefreshAll() {
  await loadDevices();
  await loadWatchlist();
  await loadAlerts();
  if (selectedDeviceIP.value) {
    await selectDevice(selectedDeviceIP.value);
  }
  ElMessage.success('数据已刷新');
}

function handleTabChange(tabName: any) {
  if (tabName === 'alerts') {
    loadAlerts();
  } else if (tabName === 'watchlist') {
    loadWatchlist();
  }
}

function jumpToDeviceTree(ip: string) {
  activeTab.value = 'tree';
  selectDevice(ip);
}

// 导出 CSV
async function exportCurrentDeviceCSV() {
  if (!selectedDeviceIP.value) return;
  try {
    const csvContent = await HardwareInventoryAPI.exportHardwareInventoryCSV(selectedDeviceIP.value);
    downloadCSV(csvContent, `硬件清单_${selectedDeviceIP.value}.csv`);
  } catch (err: any) {
    ElMessage.error(`导出失败: ${err.message || err}`);
  }
}

async function exportAlertsCSV() {
  try {
    const csvContent = await HardwareInventoryAPI.exportBOMAlertsCSV('', alertSeverityFilter.value);
    downloadCSV(csvContent, `BOM批次预警矩阵_${new Date().toISOString().slice(0, 10)}.csv`);
  } catch (err: any) {
    ElMessage.error(`导出失败: ${err.message || err}`);
  }
}

function downloadCSV(content: string, filename: string) {
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.setAttribute('href', url);
  link.setAttribute('download', filename);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
  ElMessage.success('报表导出成功');
}

// 采集弹窗
function openCollectModal() {
  selectedCollectIPs.value = [];
  collectModalVisible.value = true;
}

function triggerSingleCollect(ip: string) {
  selectedCollectIPs.value = [ip];
  collectModalVisible.value = true;
}

function selectAllDevices() {
  selectedCollectIPs.value = devices.value.map(d => d.ip);
}

function selectUncollectedDevices() {
  selectedCollectIPs.value = devices.value.filter(d => !d.hasCeasData).map(d => d.ip);
}

function clearSelectedDevices() {
  selectedCollectIPs.value = [];
}

async function submitCollectTask() {
  if (selectedCollectIPs.value.length === 0) {
    ElMessage.warning('请选择至少一台设备');
    return;
  }
  submittingCollect.value = true;
  try {
    const runID = await HardwareInventoryAPI.triggerCEASCollect(selectedCollectIPs.value);
    collectModalVisible.value = false;
    ElMessageBox.confirm(
      `CEAS 硬件清单采集任务已提交成功 (RunID: ${runID})。是否立即前往任务执行监控页面？`,
      '任务已启动',
      {
        confirmButtonText: '查看任务执行',
        cancelButtonText: '留在此页',
        type: 'success'
      }
    ).then(() => {
      router.push(`/task-execution?runId=${runID}`);
    }).catch(() => {
      // 用户选择留在此页
    });
  } catch (err: any) {
    ElMessage.error(`任务提交失败: ${err.message || err}`);
  } finally {
    submittingCollect.value = false;
  }
}

// Watchlist CRUD
function openAddWatchlistModal() {
  editingWatchlist.value = {
    item: '',
    category: 'user_custom',
    model: '',
    severity: 'warning',
    batchNo: '',
    description: '',
    enabled: true
  };
  watchlistModalVisible.value = true;
}

function openEditWatchlistModal(item: BOMWatchlistItem) {
  editingWatchlist.value = { ...item };
  watchlistModalVisible.value = true;
}

async function submitSaveWatchlist() {
  if (!editingWatchlist.value.item?.trim()) {
    ElMessage.warning('BOM 编码不能为空');
    return;
  }
  savingWatchlist.value = true;
  try {
    await HardwareInventoryAPI.saveBOMWatchlistItem(editingWatchlist.value as BOMWatchlistItem);
    ElMessage.success('BOM 预警物料保存成功');
    watchlistModalVisible.value = false;
    await loadWatchlist();
    await loadAlerts();
    await loadDevices();
  } catch (err: any) {
    ElMessage.error(`保存失败: ${err.message || err}`);
  } finally {
    savingWatchlist.value = false;
  }
}

async function handleToggleWatchlist(row: BOMWatchlistItem) {
  try {
    await HardwareInventoryAPI.toggleBOMWatchlistEnabled(row.id, row.enabled);
    ElMessage.success(`已${row.enabled ? '启用' : '禁用'} BOM 编码 ${row.item}`);
    await loadAlerts();
    await loadDevices();
  } catch (err: any) {
    row.enabled = !row.enabled;
    ElMessage.error(`操作失败: ${err.message || err}`);
  }
}

async function handleDeleteWatchlist(row: BOMWatchlistItem) {
  try {
    await ElMessageBox.confirm(`确定删除 BOM 预警编码 "${row.item}" 吗？`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    });
    await HardwareInventoryAPI.deleteBOMWatchlistItem(row.id);
    ElMessage.success('删除成功');
    await loadWatchlist();
    await loadAlerts();
    await loadDevices();
  } catch (err: any) {
    if (err !== 'cancel') {
      ElMessage.error(`删除失败: ${err.message || err}`);
    }
  }
}
</script>

<style scoped>
:deep(.custom-tabs .el-tabs__header) {
  margin-bottom: 0;
  padding: 0 1.25rem;
  background-color: var(--color-bg-panel);
  border-bottom: 1px solid var(--color-border);
}

:deep(.custom-tabs .el-tabs__content) {
  flex: 1;
  min-height: 0;
  height: 100%;
}

:deep(.custom-tabs .el-tab-pane) {
  height: 100%;
}
</style>
