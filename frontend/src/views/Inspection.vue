<template>
  <div class="h-full flex flex-col space-y-4 animate-slide-in">
    <!-- 顶部标题栏与全局动作 -->
    <div class="flex items-center justify-between bg-bg-panel border border-border px-6 py-4 rounded-xl shadow-card">
      <div>
        <div class="flex items-center gap-3">
          <h1 class="text-xl font-bold text-text-primary tracking-tight">自动化设备巡检与合流工作台</h1>
          <el-tag type="primary" effect="plain" size="small" class="font-mono">P4 Complete</el-tag>
        </div>
        <p class="text-xs text-text-muted mt-1">
          指标采集与外置阈值判定 · 前置项优先编排 · 专家处置指导 · 统一交付合流矩阵
        </p>
      </div>
      <div class="flex items-center gap-3">
        <!-- 运行实例切换 -->
        <el-select
          v-model="selectedRunID"
          placeholder="选择巡检执行记录"
          size="default"
          style="width: 260px"
          @change="handleRunChange"
        >
          <el-option label="全局最新巡检结果" value="" />
          <el-option
            v-for="run in recentRuns"
            :key="run.id"
            :label="`${run.name} (${formatTime(run.createdAt)})`"
            :value="run.id"
          />
        </el-select>

        <el-button type="primary" :icon="VideoPlay" @click="openLaunchModal">
          发起巡检
        </el-button>

        <el-dropdown trigger="click" @command="handleExportCommand">
          <el-button :icon="Download">
            导出报告 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="csv">导出为 CSV (含 UTF-8 BOM)</el-dropdown-item>
              <el-dropdown-item command="json">导出为 JSON 结构化数据</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <el-button :icon="Refresh" @click="handleRefreshAll" :loading="loading">
          刷新
        </el-button>
      </div>
    </div>

    <!-- 顶部指标统计看板 -->
    <div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-3">
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-text-muted">总巡检设备</div>
        <div class="text-lg font-bold font-mono text-text-primary mt-1">{{ summary.totalDevices }}</div>
      </div>
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-success">正常设备</div>
        <div class="text-lg font-bold font-mono text-success mt-1">{{ summary.passDevices }}</div>
      </div>
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-warning">告警设备</div>
        <div class="text-lg font-bold font-mono text-warning mt-1">{{ summary.warnDevices }}</div>
      </div>
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-danger">异常设备</div>
        <div class="text-lg font-bold font-mono text-danger mt-1">{{ summary.failDevices + summary.exceptDevices }}</div>
      </div>
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-text-muted">总检查项</div>
        <div class="text-lg font-bold font-mono text-text-primary mt-1">{{ summary.totalItems }}</div>
      </div>
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-danger">Blocker (致命)</div>
        <div class="text-lg font-bold font-mono text-danger mt-1">{{ summary.blockerCount }}</div>
      </div>
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-orange-500">Major (严重)</div>
        <div class="text-lg font-bold font-mono text-orange-500 mt-1">{{ summary.majorCount }}</div>
      </div>
      <div class="bg-bg-panel border border-border rounded-lg p-3 shadow-sm">
        <div class="text-[11px] text-warning">Minor (一般)</div>
        <div class="text-lg font-bold font-mono text-warning mt-1">{{ summary.minorCount }}</div>
      </div>
    </div>

    <!-- 核心标签页区域 -->
    <div class="flex-1 min-h-0 bg-bg-panel border border-border rounded-xl shadow-card flex flex-col overflow-hidden">
      <el-tabs v-model="activeTab" class="h-full flex flex-col custom-tabs" @tab-change="handleTabChange">
        <!-- 标签页 1: 巡检结果与矩阵 -->
        <el-tab-pane label="巡检报告与明细矩阵" name="matrix" class="h-full">
          <div class="h-full flex flex-col p-4 space-y-3">
            <!-- 过滤筛选栏 -->
            <div class="flex items-center justify-between gap-3 flex-wrap bg-bg-secondary/40 p-3 rounded-lg border border-border/50">
              <div class="flex items-center gap-2.5 flex-wrap">
                <el-input
                  v-model="searchFilter.deviceIP"
                  placeholder="搜索设备 IP..."
                  :prefix-icon="Search"
                  clearable
                  size="small"
                  style="width: 170px"
                  @change="loadResults"
                />
                <el-select
                  v-model="searchFilter.status"
                  placeholder="结果状态"
                  clearable
                  size="small"
                  style="width: 130px"
                  @change="loadResults"
                >
                  <el-option label="通过 (PASS)" value="TEST_PASS" />
                  <el-option label="失败 (FAIL)" value="TEST_FAIL" />
                  <el-option label="告警 (WARNING)" value="TEST_WARNING" />
                  <el-option label="异常 (EXCEPT)" value="TEST_EXCEPT" />
                </el-select>
                <el-select
                  v-model="searchFilter.severity"
                  placeholder="严重度"
                  clearable
                  size="small"
                  style="width: 120px"
                  @change="loadResults"
                >
                  <el-option label="Blocker (致命)" value="blocker" />
                  <el-option label="Major (严重)" value="major" />
                  <el-option label="Minor (一般)" value="minor" />
                  <el-option label="Info (提示)" value="info" />
                </el-select>
                <el-select
                  v-model="searchFilter.category"
                  placeholder="分类"
                  clearable
                  size="small"
                  style="width: 120px"
                  @change="filterLocalResults"
                >
                  <el-option label="系统基础" value="system" />
                  <el-option label="运行环境" value="environment" />
                  <el-option label="性能负载" value="performance" />
                  <el-option label="接口链路" value="interface" />
                  <el-option label="路由协议" value="routing" />
                  <el-option label="安全合规" value="security" />
                </el-select>
                <el-input
                  v-model="searchFilter.keyword"
                  placeholder="关键词 (项名/命令/建议)..."
                  clearable
                  size="small"
                  style="width: 200px"
                  @input="filterLocalResults"
                />
              </div>
              <div class="text-xs text-text-muted">
                共 <span class="font-mono font-semibold text-text-primary">{{ filteredResults.length }}</span> 条检查记录
              </div>
            </div>

            <!-- 结果列表表格 -->
            <div class="flex-1 overflow-hidden border border-border rounded-lg">
              <el-table
                :data="filteredResults"
                height="100%"
                stripe
                size="small"
                v-loading="loading"
              >
                <el-table-column label="设备 IP" prop="deviceIp" width="135">
                  <template #default="{ row }">
                    <span class="font-mono font-semibold text-accent">{{ row.deviceIp }}</span>
                  </template>
                </el-table-column>

                <el-table-column label="检查项" min-width="170">
                  <template #default="{ row }">
                    <div class="flex flex-col">
                      <span class="font-medium text-text-primary">{{ row.itemName }}</span>
                      <span class="font-mono text-[10px] text-text-muted">{{ row.itemCode }}</span>
                    </div>
                  </template>
                </el-table-column>

                <el-table-column label="分类" prop="category" width="95">
                  <template #default="{ row }">
                    <el-tag size="small" effect="plain" class="capitalize text-[11px]">
                      {{ formatCategory(row.category) }}
                    </el-tag>
                  </template>
                </el-table-column>

                <el-table-column label="判定状态" width="110" align="center">
                  <template #default="{ row }">
                    <el-tag
                      size="small"
                      :type="getResultTagType(row.status)"
                      effect="dark"
                      class="font-mono font-bold text-[11px]"
                    >
                      {{ row.status }}
                    </el-tag>
                  </template>
                </el-table-column>

                <el-table-column label="严重度" width="95" align="center">
                  <template #default="{ row }">
                    <el-tag
                      size="small"
                      :type="getSeverityTagType(row.severity)"
                      effect="light"
                      class="capitalize text-[11px]"
                    >
                      {{ row.severity }}
                    </el-tag>
                  </template>
                </el-table-column>

                <el-table-column label="实际采集值 / 判定基准" min-width="160">
                  <template #default="{ row }">
                    <div class="flex flex-col text-xs">
                      <div class="flex items-center gap-1">
                        <span class="text-text-muted">采集值:</span>
                        <span class="font-mono font-semibold text-text-primary">{{ row.actualValue || '-' }}</span>
                      </div>
                      <div class="text-[11px] text-text-muted truncate" :title="row.thresholdHit">
                        基准: {{ row.thresholdHit || '无阈值要求' }}
                      </div>
                    </div>
                  </template>
                </el-table-column>

                <el-table-column label="问题描述与修复建议" min-width="220">
                  <template #default="{ row }">
                    <div class="flex flex-col text-xs space-y-0.5">
                      <div v-if="row.problem" class="text-danger truncate" :title="row.problem">
                        {{ row.problem }}
                      </div>
                      <div v-if="row.advice" class="text-text-muted truncate" :title="row.advice">
                        💡 {{ row.advice }}
                      </div>
                      <div v-if="!row.problem && !row.advice" class="text-success text-[11px]">
                        指标正常，无需处理
                      </div>
                    </div>
                  </template>
                </el-table-column>

                <el-table-column label="操作" width="100" align="center" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" size="small" @click="openEvidenceDrawer(row)">
                      举证详情
                    </el-button>
                  </template>
                </el-table-column>

                <template #empty>
                  <el-empty description="暂无巡检结果数据，请先选择设备发起巡检" />
                </template>
              </el-table>
            </div>
          </div>
        </el-tab-pane>

        <!-- 标签页 2: 巡检模板与规则库 -->
        <el-tab-pane label="巡检模板与规则库" name="templates" class="h-full">
          <div class="h-full flex divide-x divide-border">
            <!-- 左侧：模板列表 -->
            <div class="w-80 flex flex-col h-full bg-bg-panel/50">
              <div class="p-3 border-b border-border flex items-center justify-between">
                <span class="text-xs font-semibold text-text-primary">巡检模板 ({{ templates.length }})</span>
                <el-button size="small" type="primary" link :icon="Plus" @click="openTemplateModal(null)">
                  新建模板
                </el-button>
              </div>

              <div class="flex-1 overflow-y-auto divide-y divide-border/60 scrollbar-custom">
                <div
                  v-for="tpl in templates"
                  :key="tpl.id"
                  class="p-3.5 cursor-pointer transition-colors duration-150 hover:bg-bg-hover"
                  :class="{ 'bg-accent/10 border-l-4 border-l-accent': selectedTemplate?.id === tpl.id }"
                  @click="selectTemplate(tpl)"
                >
                  <div class="flex items-center justify-between mb-1">
                    <span class="font-medium text-sm text-text-primary">{{ tpl.name }}</span>
                    <el-tag size="small" effect="plain" class="capitalize text-[10px]">
                      {{ tpl.category }}
                    </el-tag>
                  </div>
                  <div class="text-xs text-text-muted line-clamp-1 mb-2">
                    {{ tpl.description || '无模板说明' }}
                  </div>
                  <div class="flex items-center justify-between text-[11px] text-text-muted">
                    <span class="font-mono">{{ tpl.vendor }}</span>
                    <div class="flex items-center gap-1">
                      <el-button link type="primary" size="small" @click.stop="openTemplateModal(tpl)">编辑</el-button>
                      <el-button link type="danger" size="small" @click.stop="handleDeleteTemplate(tpl)">删除</el-button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- 右侧：当前模板项列表 -->
            <div class="flex-1 flex flex-col h-full min-w-0 bg-bg-panel p-4 space-y-3">
              <template v-if="selectedTemplate">
                <div class="flex items-center justify-between border-b border-border pb-3">
                  <div>
                    <h3 class="text-base font-bold text-text-primary">{{ selectedTemplate.name }} - 检查规则项</h3>
                    <p class="text-xs text-text-muted mt-0.5">
                      {{ selectedTemplate.description || '支持动态配置阈值与前置执行' }}
                    </p>
                  </div>
                  <el-button type="primary" size="small" :icon="Plus" @click="openItemModal(null)">
                    添加检查项
                  </el-button>
                </div>

                <div class="flex-1 overflow-hidden border border-border rounded-lg">
                  <el-table :data="templateItems" height="100%" stripe size="small" v-loading="itemsLoading">
                    <el-table-column label="启用" width="65" align="center">
                      <template #default="{ row }">
                        <el-switch
                          v-model="row.enabled"
                          size="small"
                          @change="(val: any) => handleToggleItem(row, Boolean(val))"
                        />
                      </template>
                    </el-table-column>

                    <el-table-column label="权重" prop="order" width="55" align="center" />

                    <el-table-column label="前置采集" width="80" align="center">
                      <template #default="{ row }">
                        <el-tag
                          v-if="row.isPreCollect"
                          size="small"
                          type="success"
                          effect="plain"
                          class="text-[10px]"
                        >
                          前置项
                        </el-tag>
                        <span v-else class="text-text-muted text-[11px]">-</span>
                      </template>
                    </el-table-column>

                    <el-table-column label="检查项名称 / 编码" min-width="160">
                      <template #default="{ row }">
                        <div class="flex flex-col">
                          <span class="font-medium text-text-primary">{{ row.name }}</span>
                          <span class="font-mono text-[10px] text-text-muted">{{ row.code }}</span>
                        </div>
                      </template>
                    </el-table-column>

                    <el-table-column label="分类" prop="category" width="90">
                      <template #default="{ row }">
                        <el-tag size="small" effect="plain" class="capitalize text-[11px]">
                          {{ formatCategory(row.category) }}
                        </el-tag>
                      </template>
                    </el-table-column>

                    <el-table-column label="执行命令" prop="commandKey" min-width="140">
                      <template #default="{ row }">
                        <code class="font-mono text-xs bg-bg-secondary px-1.5 py-0.5 rounded text-accent">
                          {{ row.commandKey }}
                        </code>
                      </template>
                    </el-table-column>

                    <el-table-column label="判定方式" prop="checkType" width="100">
                      <template #default="{ row }">
                        <span class="text-xs font-mono text-text-secondary">{{ row.checkType }}</span>
                      </template>
                    </el-table-column>

                    <el-table-column label="默认严重度" prop="severity" width="90" align="center">
                      <template #default="{ row }">
                        <el-tag size="small" :type="getSeverityTagType(row.severity)" effect="light" class="capitalize text-[10px]">
                          {{ row.severity }}
                        </el-tag>
                      </template>
                    </el-table-column>

                    <el-table-column label="操作" width="110" align="center" fixed="right">
                      <template #default="{ row }">
                        <el-button link type="primary" size="small" @click="openItemModal(row)">编辑</el-button>
                        <el-button link type="danger" size="small" @click="handleDeleteItem(row)">删除</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                </div>
              </template>
              <div v-else class="h-full flex items-center justify-center text-text-muted">
                请在左侧选择巡检模板
              </div>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 举证与指导建议 Drawer -->
    <el-drawer
      v-model="evidenceDrawerVisible"
      title="巡检结果举证与处置建议"
      size="540px"
      destroy-on-close
    >
      <div v-if="selectedResult" class="space-y-4 text-xs">
        <div class="bg-bg-secondary p-3 rounded-lg border border-border space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-text-muted">设备 IP:</span>
            <span class="font-mono font-bold text-accent text-sm">{{ selectedResult.deviceIp }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-text-muted">检查项目:</span>
            <span class="font-medium text-text-primary">{{ selectedResult.itemName }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-text-muted">状态与等级:</span>
            <div class="flex items-center gap-1.5">
              <el-tag size="small" :type="getResultTagType(selectedResult.status)" effect="dark">
                {{ selectedResult.status }}
              </el-tag>
              <el-tag size="small" :type="getSeverityTagType(selectedResult.severity)" effect="light">
                {{ selectedResult.severity }}
              </el-tag>
            </div>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-text-muted">实测采集值:</span>
            <span class="font-mono font-semibold text-text-primary">{{ selectedResult.actualValue || '-' }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-text-muted">触发判定规则:</span>
            <span class="font-mono text-text-muted">{{ selectedResult.thresholdHit || '基准匹配' }}</span>
          </div>
        </div>

        <div v-if="selectedResult.problem" class="border-l-4 border-danger bg-danger/10 p-3 rounded">
          <div class="font-semibold text-danger mb-1">异常问题描述:</div>
          <div class="text-text-primary whitespace-pre-wrap">{{ selectedResult.problem }}</div>
        </div>

        <div v-if="selectedResult.advice" class="border-l-4 border-accent bg-accent/10 p-3 rounded">
          <div class="font-semibold text-accent mb-1">💡 专家修复建议 (eDesk Pro 知识库):</div>
          <div class="text-text-primary leading-relaxed whitespace-pre-wrap">{{ selectedResult.advice }}</div>
        </div>

        <div>
          <div class="font-semibold text-text-primary mb-2 flex items-center justify-between">
            <span>关键举证输出 (Evidence Trace):</span>
            <span class="text-text-muted text-[11px] font-normal">共 {{ parsedEvidenceLines.length }} 行</span>
          </div>
          <div class="bg-gray-950 text-gray-200 font-mono p-3 rounded-lg max-h-72 overflow-y-auto border border-gray-800 text-[11px] leading-5 scrollbar-custom">
            <div v-for="(line, idx) in parsedEvidenceLines" :key="idx" class="whitespace-pre hover:bg-white/5 px-1 rounded">
              <span class="text-gray-500 mr-2 select-none">{{ idx + 1 }}.</span>
              <span>{{ line }}</span>
            </div>
            <div v-if="parsedEvidenceLines.length === 0" class="text-gray-500 italic">
              无关联的上下文行或未触发异常
            </div>
          </div>
        </div>
      </div>
    </el-drawer>

    <!-- 发起巡检弹窗 -->
    <el-dialog
      v-model="launchModalVisible"
      title="发起设备巡检任务"
      width="640px"
      destroy-on-close
    >
      <div class="space-y-4">
        <div>
          <label class="block text-xs font-semibold text-text-primary mb-1.5">选择巡检模板</label>
          <el-select v-model="launchForm.templateId" class="w-full" placeholder="请选择巡检模板">
            <el-option
              v-for="tpl in templates"
              :key="tpl.id"
              :label="`${tpl.name} (${tpl.vendor} · ${tpl.category})`"
              :value="tpl.id"
            />
          </el-select>
        </div>

        <div>
          <div class="flex items-center justify-between mb-1.5">
            <label class="text-xs font-semibold text-text-primary">
              选择巡检目标设备 (已选 {{ launchForm.deviceIps.length }} / {{ allDevices.length }})
            </label>
            <div class="flex items-center gap-2">
              <el-button link size="small" type="primary" @click="selectAllDevices">全选</el-button>
              <el-button link size="small" @click="launchForm.deviceIps = []">清空</el-button>
            </div>
          </div>
          <el-input
            v-model="deviceSearchInModal"
            placeholder="搜索设备 IP / 分组 / 款型..."
            :prefix-icon="Search"
            size="small"
            clearable
            class="mb-2"
          />
          <div class="border border-border rounded-lg max-h-56 overflow-y-auto p-2 divide-y divide-border/50 scrollbar-custom">
            <div
              v-for="dev in modalFilteredDevices"
              :key="dev.ip"
              class="py-1.5 px-2 flex items-center justify-between hover:bg-bg-hover rounded text-xs cursor-pointer"
              @click="toggleDeviceSelection(dev.ip)"
            >
              <div class="flex items-center gap-2.5">
                <el-checkbox
                  :model-value="launchForm.deviceIps.includes(dev.ip)"
                  @click.stop
                  @change="() => toggleDeviceSelection(dev.ip)"
                />
                <span class="font-mono font-medium text-text-primary">{{ dev.ip }}</span>
                <span class="text-text-muted text-[11px]">{{ dev.model || dev.group || '-' }}</span>
              </div>
              <el-tag size="small" effect="plain" class="capitalize text-[10px]">{{ dev.vendor || '通用' }}</el-tag>
            </div>
            <div v-if="modalFilteredDevices.length === 0" class="p-4 text-center text-text-muted text-xs">
              无匹配设备
            </div>
          </div>
        </div>

        <div>
          <div class="flex items-center justify-between text-xs mb-1">
            <span class="font-semibold text-text-primary">执行并发数</span>
            <span class="font-mono text-accent font-bold">{{ launchForm.concurrency }}</span>
          </div>
          <el-slider v-model="launchForm.concurrency" :min="1" :max="30" :step="1" />
        </div>
      </div>

      <template #footer>
        <div class="flex items-center justify-end gap-2">
          <el-button @click="launchModalVisible = false">取消</el-button>
          <el-button
            type="primary"
            :loading="launching"
            :disabled="launchForm.deviceIps.length === 0"
            @click="handleLaunchInspection"
          >
            开始执行巡检
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 模板编辑弹窗 -->
    <el-dialog
      v-model="templateModalVisible"
      :title="editingTemplate.id ? '编辑巡检模板' : '新建巡检模板'"
      width="480px"
      destroy-on-close
    >
      <el-form :model="editingTemplate" label-position="top" size="small">
        <el-form-item label="模板名称" required>
          <el-input v-model="editingTemplate.name" placeholder="如：华为交换机日常深度巡检" />
        </el-form-item>
        <el-form-item label="适用厂商">
          <el-select v-model="editingTemplate.vendor" class="w-full">
            <el-option label="Huawei (华为)" value="huawei" />
            <el-option label="H3C (华三)" value="h3c" />
            <el-option label="Cisco (思科)" value="cisco" />
            <el-option label="通用" value="general" />
          </el-select>
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="editingTemplate.category" class="w-full">
            <el-option label="数据中心 (CE)" value="ce" />
            <el-option label="园区接入/汇聚 (S)" value="s" />
            <el-option label="路由器/网关 (AR)" value="ar" />
            <el-option label="防火墙 (FW)" value="fw" />
            <el-option label="通用日常 (General)" value="general" />
          </el-select>
        </el-form-item>
        <el-form-item label="说明描述">
          <el-input v-model="editingTemplate.description" type="textarea" :rows="2" placeholder="模板适用范围与用途说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="templateModalVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSaveTemplate">保存</el-button>
      </template>
    </el-dialog>

    <!-- 检查项编辑弹窗 -->
    <el-dialog
      v-model="itemModalVisible"
      :title="editingItem.id ? '编辑巡检检查项' : '添加巡检检查项'"
      width="560px"
      destroy-on-close
    >
      <el-form :model="editingItem" label-position="top" size="small">
        <div class="grid grid-cols-2 gap-3">
          <el-form-item label="项目编码 (Code)" required>
            <el-input v-model="editingItem.code" placeholder="如: CHECK_CPU_USAGE" />
          </el-form-item>
          <el-form-item label="项目名称" required>
            <el-input v-model="editingItem.name" placeholder="如: CPU利用率检查" />
          </el-form-item>
        </div>

        <div class="grid grid-cols-3 gap-3">
          <el-form-item label="分类">
            <el-select v-model="editingItem.category">
              <el-option label="系统基础" value="system" />
              <el-option label="运行环境" value="environment" />
              <el-option label="性能负载" value="performance" />
              <el-option label="接口链路" value="interface" />
              <el-option label="路由协议" value="routing" />
              <el-option label="安全合规" value="security" />
            </el-select>
          </el-form-item>
          <el-form-item label="判定方式">
            <el-select v-model="editingItem.checkType">
              <el-option label="外置阈值 (threshold)" value="threshold" />
              <el-option label="必须包含 (must_contain)" value="must_contain" />
              <el-option label="不可包含 (must_not_contain)" value="must_not_contain" />
              <el-option label="正则匹配 (regex)" value="regex" />
              <el-option label="等于合格 (equals)" value="equals" />
            </el-select>
          </el-form-item>
          <el-form-item label="默认严重度">
            <el-select v-model="editingItem.severity">
              <el-option label="Blocker" value="blocker" />
              <el-option label="Major" value="major" />
              <el-option label="Minor" value="minor" />
              <el-option label="Info" value="info" />
            </el-select>
          </el-form-item>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <el-form-item label="执行 CLI 命令" required>
            <el-input v-model="editingItem.commandKey" placeholder="如: display cpu-usage" />
          </el-form-item>
          <el-form-item label="判定字段 (Field)">
            <el-input v-model="editingItem.field" placeholder="如: cpu_usage / status" />
          </el-form-item>
        </div>

        <div class="flex items-center gap-6 mb-3 p-2.5 bg-bg-secondary rounded border border-border">
          <div class="flex items-center gap-2">
            <el-switch v-model="editingItem.isPreCollect" />
            <span class="text-xs font-semibold text-text-primary">前置优先采集 (IsPreCollect)</span>
          </div>
          <span class="text-[11px] text-text-muted">优先执行并进入会话缓存，避免重复发命令</span>
        </div>

        <el-form-item label="阈值配置 JSON (Thresholds)">
          <el-input
            v-model="editingItem.thresholdsJson"
            type="textarea"
            :rows="2"
            placeholder='[{"name":"cpu_max","rangeType":"bound","maxValue":"80","unit":"%"}]'
          />
        </el-form-item>

        <el-form-item label="异常默认描述 (Problem)">
          <el-input v-model="editingItem.problem" placeholder="如: 设备当前 CPU 负载超过安全阈值" />
        </el-form-item>

        <el-form-item label="专家处置建议 (Advice - 来源于 eDesk Pro 知识库)">
          <el-input v-model="editingItem.advice" type="textarea" :rows="2" placeholder="指导运维人员排查具体进程与降载动作" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="itemModalVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSaveItem">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  VideoPlay,
  Refresh,
  Download,
  Search,
  Plus,
  ArrowDown,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  InspectionAPI,
  type InspectionTemplate,
  type InspectionItem,
  type InspectionResult,
  type InspectionSummaryVO,
  type TaskRun,
} from '@/services/inspectionApi'
import { DeviceAPI, type DeviceAsset } from '@/services/api'

const route = useRoute()

// 状态控制
const activeTab = ref('matrix')
const loading = ref(false)
const itemsLoading = ref(false)
const launching = ref(false)
const launchModalVisible = ref(false)
const templateModalVisible = ref(false)
const itemModalVisible = ref(false)
const evidenceDrawerVisible = ref(false)

// 数据源
const recentRuns = ref<TaskRun[]>([])
const selectedRunID = ref('')
const summary = reactive<InspectionSummaryVO>({
  totalDevices: 0,
  passDevices: 0,
  failDevices: 0,
  warnDevices: 0,
  exceptDevices: 0,
  totalItems: 0,
  passItems: 0,
  failItems: 0,
  warnItems: 0,
  exceptItems: 0,
  ignoreItems: 0,
  manualItems: 0,
  untestItems: 0,
  unaccordItems: 0,
  blockerCount: 0,
  majorCount: 0,
  minorCount: 0,
  infoCount: 0,
})
const allResults = ref<InspectionResult[]>([])
const templates = ref<InspectionTemplate[]>([])
const selectedTemplate = ref<InspectionTemplate | null>(null)
const templateItems = ref<InspectionItem[]>([])
const selectedResult = ref<InspectionResult | null>(null)
const allDevices = ref<DeviceAsset[]>([])

// 搜索与过滤
const searchFilter = reactive({
  deviceIP: '',
  status: '',
  severity: '',
  category: '',
  keyword: '',
})

// 弹窗表单数据
const deviceSearchInModal = ref('')
const launchForm = reactive({
  templateId: 'tpl-huawei-general',
  deviceIps: [] as string[],
  concurrency: 10,
})

const editingTemplate = reactive<Partial<InspectionTemplate>>({
  id: '',
  name: '',
  vendor: 'huawei',
  category: 'general',
  description: '',
})

const editingItem = reactive<Partial<InspectionItem>>({
  id: 0,
  templateId: '',
  code: '',
  name: '',
  category: 'system',
  commandKey: '',
  isPreCollect: false,
  checkType: 'threshold',
  field: '',
  thresholdsJson: '[]',
  severity: 'major',
  order: 0,
  problem: '',
  advice: '',
})

// 计算属性
const filteredResults = computed(() => {
  return allResults.value.filter((r) => {
    if (searchFilter.category && r.category !== searchFilter.category) {
      return false
    }
    if (searchFilter.keyword) {
      const kw = searchFilter.keyword.toLowerCase()
      const matchName = r.itemName?.toLowerCase().includes(kw)
      const matchCode = r.itemCode?.toLowerCase().includes(kw)
      const matchAdvice = r.advice?.toLowerCase().includes(kw)
      const matchProblem = r.problem?.toLowerCase().includes(kw)
      if (!matchName && !matchCode && !matchAdvice && !matchProblem) {
        return false
      }
    }
    return true
  })
})

const modalFilteredDevices = computed(() => {
  if (!deviceSearchInModal.value) return allDevices.value
  const q = deviceSearchInModal.value.toLowerCase()
  return allDevices.value.filter(
    (d) =>
      d.ip.toLowerCase().includes(q) ||
      d.model?.toLowerCase().includes(q) ||
      d.group?.toLowerCase().includes(q) ||
      d.vendor?.toLowerCase().includes(q)
  )
})

const parsedEvidenceLines = computed(() => {
  if (!selectedResult.value?.evidenceJson) return []
  try {
    const lines = JSON.parse(selectedResult.value.evidenceJson)
    return Array.isArray(lines) ? lines : []
  } catch {
    return [selectedResult.value.evidenceJson]
  }
})

// 初始化与加载方法
onMounted(async () => {
  await Promise.all([
    loadRecentRuns(),
    loadTemplates(),
    loadDevices(),
  ])

  // 支持路由 query 参数自动填充
  const queryIP = route.query.ip as string
  if (queryIP) {
    searchFilter.deviceIP = queryIP
    launchForm.deviceIps = [queryIP]
  }

  // 默认选中最新运行记录，避免无选择时混淆历史批次
  if (recentRuns.value.length > 0 && !selectedRunID.value && recentRuns.value[0]) {
    selectedRunID.value = recentRuns.value[0].id
  }

  await loadResults()
  await loadSummary()
})

async function loadRecentRuns() {
  try {
    recentRuns.value = await InspectionAPI.listRecentRuns(20)
  } catch (err) {
    console.error('加载最近巡检运行记录失败:', err)
  }
}

async function loadTemplates() {
  try {
    templates.value = await InspectionAPI.listTemplates()
    const first = templates.value[0]
    if (first && !selectedTemplate.value) {
      selectTemplate(first)
    }
  } catch (err) {
    console.error('加载巡检模板失败:', err)
  }
}

async function loadDevices() {
  try {
    allDevices.value = await DeviceAPI.listDevices()
  } catch (err) {
    console.error('加载设备列表失败:', err)
  }
}

async function loadResults() {
  loading.value = true
  try {
    allResults.value = await InspectionAPI.getResults(
      selectedRunID.value,
      searchFilter.deviceIP,
      searchFilter.status,
      searchFilter.severity
    )
  } catch (err) {
    console.error('查询巡检结果明细失败:', err)
    ElMessage.error('查询巡检结果明细失败')
  } finally {
    loading.value = false
  }
}

async function loadSummary() {
  try {
    const res = await InspectionAPI.getSummary(selectedRunID.value)
    if (res) {
      Object.assign(summary, res)
    }
  } catch (err) {
    console.error('获取巡检汇总指标失败:', err)
  }
}

function handleRunChange() {
  loadResults()
  loadSummary()
}

function handleTabChange(tabName: any) {
  if (tabName === 'templates' && selectedTemplate.value) {
    loadTemplateItems(selectedTemplate.value.id)
  }
}

function handleRefreshAll() {
  loadRecentRuns()
  loadResults()
  loadSummary()
  if (selectedTemplate.value) {
    loadTemplateItems(selectedTemplate.value.id)
  }
}

function filterLocalResults() {
  // 触发 computed 重新求值
}

// 模板与项管理
function selectTemplate(tpl: InspectionTemplate) {
  selectedTemplate.value = tpl
  loadTemplateItems(tpl.id)
}

async function loadTemplateItems(templateId: string) {
  itemsLoading.value = true
  try {
    templateItems.value = await InspectionAPI.listItems(templateId)
  } catch (err) {
    console.error('加载模板项失败:', err)
    ElMessage.error('加载模板项失败')
  } finally {
    itemsLoading.value = false
  }
}

function openTemplateModal(tpl: InspectionTemplate | null) {
  if (tpl) {
    Object.assign(editingTemplate, tpl)
  } else {
    editingTemplate.id = ''
    editingTemplate.name = ''
    editingTemplate.vendor = 'huawei'
    editingTemplate.category = 'general'
    editingTemplate.description = ''
  }
  templateModalVisible.value = true
}

async function handleSaveTemplate() {
  if (!editingTemplate.name) {
    ElMessage.warning('请输入模板名称')
    return
  }
  try {
    await InspectionAPI.saveTemplate(editingTemplate as InspectionTemplate)
    ElMessage.success('模板保存成功')
    templateModalVisible.value = false
    await loadTemplates()
  } catch (err) {
    ElMessage.error(`保存失败: ${err}`)
  }
}

async function handleDeleteTemplate(tpl: InspectionTemplate) {
  try {
    await ElMessageBox.confirm(`确定要删除模板 [${tpl.name}] 及其关联项吗？`, '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await InspectionAPI.deleteTemplate(tpl.id)
    ElMessage.success('模板已删除')
    if (selectedTemplate.value?.id === tpl.id) {
      selectedTemplate.value = null
      templateItems.value = []
    }
    await loadTemplates()
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error(`删除失败: ${err}`)
    }
  }
}

function openItemModal(item: InspectionItem | null) {
  if (item) {
    Object.assign(editingItem, item)
  } else {
    editingItem.id = 0
    editingItem.templateId = selectedTemplate.value?.id || 'tpl-huawei-general'
    editingItem.code = ''
    editingItem.name = ''
    editingItem.category = 'system'
    editingItem.commandKey = ''
    editingItem.isPreCollect = false
    editingItem.checkType = 'threshold'
    editingItem.field = ''
    editingItem.thresholdsJson = '[]'
    editingItem.severity = 'major'
    editingItem.order = (templateItems.value.length + 1) * 10
    editingItem.problem = ''
    editingItem.advice = ''
  }
  itemModalVisible.value = true
}

async function handleSaveItem() {
  if (!editingItem.name || !editingItem.code || !editingItem.commandKey) {
    ElMessage.warning('名称、编码和命令均必填')
    return
  }
  try {
    await InspectionAPI.saveItem(editingItem as InspectionItem)
    ElMessage.success('检查项保存成功')
    itemModalVisible.value = false
    if (selectedTemplate.value) {
      await loadTemplateItems(selectedTemplate.value.id)
    }
  } catch (err) {
    ElMessage.error(`保存检查项失败: ${err}`)
  }
}

async function handleDeleteItem(item: InspectionItem) {
  try {
    await ElMessageBox.confirm(`确定要删除检查项 [${item.name}] 吗？`, '删除确认', {
      type: 'warning',
    })
    await InspectionAPI.deleteItem(item.id)
    ElMessage.success('已删除检查项')
    if (selectedTemplate.value) {
      await loadTemplateItems(selectedTemplate.value.id)
    }
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error(`删除失败: ${err}`)
    }
  }
}

async function handleToggleItem(item: InspectionItem, enabled: boolean) {
  try {
    await InspectionAPI.toggleItemEnabled(item.id, enabled)
    ElMessage.success(`检查项 [${item.name}] 已${enabled ? '启用' : '禁用'}`)
  } catch (err) {
    item.enabled = !enabled
    ElMessage.error(`切换状态失败: ${err}`)
  }
}

// 举证抽屉
function openEvidenceDrawer(row: InspectionResult) {
  selectedResult.value = row
  evidenceDrawerVisible.value = true
}

// 巡检发起向导
function openLaunchModal() {
  if (selectedTemplate.value) {
    launchForm.templateId = selectedTemplate.value.id
  }
  launchModalVisible.value = true
}

function toggleDeviceSelection(ip: string) {
  const idx = launchForm.deviceIps.indexOf(ip)
  if (idx >= 0) {
    launchForm.deviceIps.splice(idx, 1)
  } else {
    launchForm.deviceIps.push(ip)
  }
}

function selectAllDevices() {
  launchForm.deviceIps = allDevices.value.map((d) => d.ip)
}

async function handleLaunchInspection() {
  if (launchForm.deviceIps.length === 0) {
    ElMessage.warning('请至少选择一台设备')
    return
  }
  launching.value = true
  try {
    const runID = await InspectionAPI.triggerInspection(
      launchForm.templateId,
      launchForm.deviceIps,
      launchForm.concurrency
    )
    ElMessage.success(`巡检任务已启动 (RunID: ${runID})`)
    launchModalVisible.value = false
    selectedRunID.value = runID
    activeTab.value = 'matrix'
    await loadRecentRuns()
    setTimeout(() => {
      loadResults()
      loadSummary()
    }, 1500)
  } catch (err) {
    ElMessage.error(`启动巡检失败: ${err}`)
  } finally {
    launching.value = false
  }
}

// 报告导出
async function handleExportCommand(cmd: string) {
  try {
    if (cmd === 'csv') {
      const csvData = await InspectionAPI.exportCSV(selectedRunID.value)
      downloadFile(csvData, `inspection-report-${new Date().toISOString().slice(0, 10)}.csv`, 'text/csv;charset=utf-8')
      ElMessage.success('已成功导出 CSV 巡检报告')
    } else if (cmd === 'json') {
      const jsonData = await InspectionAPI.exportJSON(selectedRunID.value)
      downloadFile(jsonData, `inspection-report-${new Date().toISOString().slice(0, 10)}.json`, 'application/json')
      ElMessage.success('已成功导出 JSON 巡检报告')
    }
  } catch (err) {
    ElMessage.error(`导出失败: ${err}`)
  }
}

function downloadFile(content: string, filename: string, mimeType: string) {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// 格式化辅助函数
function formatTime(t: any): string {
  if (!t) return '-'
  try {
    const d = new Date(t)
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch {
    return String(t)
  }
}

function formatCategory(cat: string): string {
  const map: Record<string, string> = {
    system: '系统基础',
    environment: '运行环境',
    performance: '性能负载',
    interface: '接口链路',
    routing: '路由协议',
    security: '安全合规',
  }
  return map[cat] || cat || '常规'
}

function getResultTagType(status: string): string {
  switch (status) {
    case 'TEST_PASS':
      return 'success'
    case 'TEST_FAIL':
      return 'danger'
    case 'TEST_WARNING':
      return 'warning'
    default:
      return 'info'
  }
}

function getSeverityTagType(sev: string): string {
  switch (sev) {
    case 'blocker':
      return 'danger'
    case 'major':
      return 'warning'
    case 'minor':
      return 'info'
    default:
      return ''
  }
}
</script>

<style scoped>
.custom-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
  padding: 0 1.5rem;
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
}

.custom-tabs :deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
}
</style>
