<template>
  <div class="animate-slide-in space-y-6">
    <!-- 页面标题 + 操作按钮 -->
    <div class="flex items-center justify-between">
      <!-- 搜索组件 -->
      <div class="flex items-center gap-2">
        <select
          v-model="searchType"
          class="px-3 h-9 text-sm bg-bg-panel border border-border rounded-lg text-text-primary focus:border-accent focus:outline-none transition-colors cursor-pointer"
        >
          <option v-for="opt in searchOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
        <div class="relative">
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="'搜索' + currentSearchLabel + '...'"
            class="w-64 pl-10 pr-10 h-9 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
          />
          <!-- 搜索图标 -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted pointer-events-none"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <!-- 重置按钮 -->
          <button
            v-if="searchQuery"
            @click="resetSearch"
            class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text-primary hover:bg-bg-hover rounded transition-all"
            title="清空搜索"
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
      </div>
      <button
        @click="openAddModal"
        class="flex items-center gap-2 px-4 h-9 text-sm font-medium text-white bg-accent hover:bg-accent/90 rounded-lg transition-all duration-200 shadow-sm"
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
        新增设备
      </button>
    </div>

    <!-- 数据表格 -->
    <div
      class="bg-bg-panel border border-border rounded-xl shadow-card overflow-hidden"
    >
      <div class="overflow-auto scrollbar-custom max-h-[calc(100vh-220px)]">
        <table class="w-full text-sm">
          <thead class="sticky top-0 z-10">
            <tr class="bg-bg-panel border-b border-border">
              <th
                class="px-4 py-3.5 text-center text-xs font-semibold text-text-muted uppercase tracking-wider w-12"
              >
                <button
                  @click="toggleSelectAll"
                  class="flex items-center justify-center w-4 h-4 mx-auto rounded border transition-all duration-200"
                  :class="[
                    isAllSelected 
                      ? 'bg-accent border-accent text-white' 
                      : isIndeterminate 
                        ? 'bg-accent/30 border-accent/50' 
                        : 'border-border hover:border-accent'
                  ]"
                  :title="isAllSelected ? '取消全选' : '全选所有设备'"
                >
                  <svg
                    v-if="isAllSelected"
                    xmlns="http://www.w3.org/2000/svg"
                    class="w-3 h-3"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="3"
                  >
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                </button>
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-12"
              >
                #
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-20"
              >
                <div class="flex items-center gap-1">
                  分组
                  <button
                    @click="openBatchEditModal('group')"
                    class="p-0.5 text-text-muted hover:text-accent transition-colors"
                    title="批量修改分组"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3.5 h-3.5"
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
                </div>
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-28"
              >
                IP 地址
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
              >
                <div class="flex items-center gap-1">
                  协议
                  <button
                    @click="openBatchEditModal('protocol')"
                    class="p-0.5 text-text-muted hover:text-accent transition-colors"
                    title="批量修改协议"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3.5 h-3.5"
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
                </div>
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-14"
              >
                <div class="flex items-center gap-1">
                  端口
                  <button
                    @click="openBatchEditModal('port')"
                    class="p-0.5 text-text-muted hover:text-accent transition-colors"
                    title="批量修改端口"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3.5 h-3.5"
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
                </div>
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
              >
                <div class="flex items-center gap-1">
                  用户名
                  <button
                    @click="openBatchEditModal('username')"
                    class="p-0.5 text-text-muted hover:text-accent transition-colors"
                    title="批量修改用户名"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3.5 h-3.5"
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
                </div>
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
              >
                <div class="flex items-center gap-1">
                  密码
                  <button
                    @click="openBatchEditModal('password')"
                    class="p-0.5 text-text-muted hover:text-accent transition-colors"
                    title="批量修改密码"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3.5 h-3.5"
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
                </div>
              </th>
              <th
                class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
              >
                <div class="flex items-center gap-1">
                  Tag
                  <button
                    @click="openBatchEditModal('tag')"
                    class="p-0.5 text-text-muted hover:text-accent transition-colors"
                    title="批量修改标签"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-3.5 h-3.5"
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
                </div>
              </th>
              <th
                class="px-4 py-3.5 text-center text-xs font-semibold text-text-muted uppercase tracking-wider w-24"
              >
                操作
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-if="data.length === 0">
              <td colspan="10" class="px-5 py-12 text-center text-text-muted">
                <div class="flex flex-col items-center gap-3">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="w-10 h-10 text-text-muted/40"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <rect x="2" y="2" width="20" height="8" rx="2" />
                    <rect x="2" y="14" width="20" height="8" rx="2" />
                  </svg>
                  <span class="text-sm">暂无设备数据，点击上方按钮新增</span>
                </div>
              </td>
            </tr>
            <tr
              v-for="(row, idx) in pagedData"
              :key="row.ip + idx"
              :class="[
                'transition-colors duration-150 group',
                isSelected(idx) 
                  ? 'bg-accent/8 hover:bg-accent/12' 
                  : 'hover:bg-bg-hover'
              ]"
            >
              <td class="px-4 py-3 text-center">
                <button
                  @click="toggleSelect(idx)"
                  class="flex items-center justify-center w-4 h-4 mx-auto rounded border transition-all duration-200"
                  :class="[
                    isSelected(idx)
                      ? 'bg-accent border-accent text-white'
                      : 'border-border hover:border-accent'
                  ]"
                >
                  <svg
                    v-if="isSelected(idx)"
                    xmlns="http://www.w3.org/2000/svg"
                    class="w-3 h-3"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="3"
                  >
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                </button>
              </td>
              <td class="px-4 py-3 text-text-muted font-mono text-xs">
                {{ (page - 1) * pageSize + idx + 1 }}
              </td>
              <td class="px-4 py-3 text-text-secondary text-xs">
                {{ row.group || "-" }}
              </td>
              <td class="px-4 py-3">
                <span class="font-mono text-accent font-medium">{{
                  row.ip
                }}</span>
              </td>
              <td class="px-4 py-3">
                <span
                  class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                  :class="getProtocolBadgeClass(row.protocol)"
                >
                  {{ row.protocol }}
                </span>
              </td>
              <td class="px-4 py-3 text-text-secondary font-mono">
                {{ row.port }}
              </td>
              <td class="px-4 py-3 text-text-secondary">
                {{ row.username || "-" }}
              </td>
              <td class="px-4 py-3">
                <span
                  class="font-mono text-text-muted tracking-widest text-xs"
                  >{{ row.password ? "••••••••" : "-" }}</span
                >
              </td>
              <td class="px-4 py-3 text-text-secondary text-xs">
                {{ row.tag || "-" }}
              </td>
              <td class="px-4 py-3">
                <div class="flex items-center justify-center gap-2">
                  <button
                    @click="openEditModal(idx)"
                    class="p-1.5 text-text-muted hover:text-accent hover:bg-accent/10 rounded transition-all duration-200"
                    title="编辑"
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
                    @click="confirmDelete(idx)"
                    class="p-1.5 text-text-muted hover:text-error hover:bg-error-bg rounded transition-all duration-200"
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
                      <polyline points="3,6 5,6 21,6" />
                      <path
                        d="M19,6v14a2,2,0,0,1-2,2H7a2,2,0,0,1-2-2V6m3,0V4a2,2,0,0,1,2-2h4a2,2,0,0,1,2,2v2"
                      />
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 选中提示 -->
      <div
        v-if="selectedCount > 0"
        class="flex items-center justify-between px-5 py-2.5 bg-accent/5 border-t border-accent/20"
      >
        <div class="flex items-center gap-2">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4 text-accent"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polyline points="9 11 12 14 22 4" />
            <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
          </svg>
          <span class="text-sm text-accent font-medium">
            已选中 <strong>{{ selectedCount }}</strong> 台设备
          </span>
          <button
            @click="clearSelection"
            class="ml-2 px-2 py-0.5 text-xs text-text-muted hover:text-text-primary hover:bg-bg-hover rounded transition-all"
          >
            清空选择
          </button>
        </div>
        <button
          @click="confirmBatchDelete"
          class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-error hover:bg-error/90 rounded-lg transition-all duration-200"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-3.5 h-3.5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polyline points="3,6 5,6 21,6" />
            <path d="M19,6v14a2,2,0,0,1-2,2H7a2,2,0,0,1-2-2V6m3,0V4a2,2,0,0,1,2-2h4a2,2,0,0,1,2,2v2" />
          </svg>
          批量删除
        </button>
      </div>

      <!-- 分页 -->
      <div
        class="flex items-center justify-between px-5 py-3.5 border-t border-border bg-bg-panel"
      >
      <span class="text-xs text-text-muted"
          >第 {{ page }} / {{ totalPages }} 页，共 {{ filteredData.length }} 条</span
        >
        <div class="flex items-center gap-2">
          <button
            @click="page = Math.max(1, page - 1)"
            :disabled="page === 1"
            class="px-3 py-1.5 text-xs rounded-lg border border-border text-text-secondary hover:border-accent/50 hover:text-accent disabled:opacity-30 disabled:cursor-not-allowed transition-all duration-200"
          >
            上一页
          </button>
          <button
            @click="page = Math.min(totalPages, page + 1)"
            :disabled="page === totalPages"
            class="px-3 py-1.5 text-xs rounded-lg border border-border text-text-secondary hover:border-accent/50 hover:text-accent disabled:opacity-30 disabled:cursor-not-allowed transition-all duration-200"
          >
            下一页
          </button>
        </div>
      </div>
    </div>

    <!-- 新增/编辑设备弹窗 -->
    <div
      v-if="showModal"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="absolute inset-0 bg-black/50 backdrop-blur-sm"
        @click="closeModal"
      ></div>
      <div
        class="relative bg-bg-card border border-border rounded-xl shadow-2xl w-full max-w-md mx-4 animate-slide-in"
      >
        <div
          class="flex items-center justify-between px-6 py-4 border-b border-border"
        >
          <h3 class="text-lg font-semibold text-text-primary">
            {{ isEditing ? "编辑设备" : "新增设备" }}
          </h3>
          <button
            @click="closeModal"
            class="p-1 text-text-muted hover:text-text-primary transition-colors"
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
        <form @submit.prevent="saveDevice" class="p-6 space-y-4">
          <div>
            <label class="block text-xs font-medium text-text-secondary mb-1.5"
              >分组 <span class="text-text-muted">(可选)</span></label
            >
            <input
              v-model="form.group"
              type="text"
              placeholder="设备分组名称"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
          </div>
          <div>
            <label class="block text-xs font-medium text-text-secondary mb-1.5"
              >IP 地址</label
            >
            <input
              v-model="form.ip"
              type="text"
              placeholder="例如: 192.168.1.10 或 192.168.1.10-20"
              :class="[
                'w-full px-3 py-2 text-sm bg-bg-panel border rounded-lg text-text-primary placeholder-text-muted/50 focus:outline-none transition-colors',
                ipValidationError ? 'border-error focus:border-error' : 'border-border focus:border-accent'
              ]"
              required
            />
            <!-- IP 输入错误提示 -->
            <div
              v-if="ipValidationError"
              class="mt-2 px-3 py-2 text-xs bg-error-bg border border-error/30 rounded-lg text-error flex items-center gap-2"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-4 h-4 flex-shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
              <span>{{ ipValidationError }}</span>
            </div>
            <!-- IP 语法糖提示 -->
            <div
              v-if="ipRangeHint"
              class="mt-2 px-3 py-2 text-xs bg-accent/10 border border-accent/20 rounded-lg text-accent flex items-center gap-2"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-4 h-4 flex-shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="16" x2="12" y2="12" />
                <line x1="12" y1="8" x2="12.01" y2="8" />
              </svg>
              <span
                >语法糖：将新增 <strong>{{ ipRangeHint.count }}</strong> 台设备
                ({{ ipRangeHint.start }} - {{ ipRangeHint.end }})</span
              >
            </div>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label
                class="block text-xs font-medium text-text-secondary mb-1.5"
                >协议</label
              >
              <select
                v-model="form.protocol"
                @change="onProtocolChange"
                class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary focus:border-accent focus:outline-none transition-colors"
              >
                <option v-for="p in validProtocols" :key="p" :value="p">
                  {{ p }}
                </option>
              </select>
            </div>
            <div>
              <label
                class="block text-xs font-medium text-text-secondary mb-1.5"
                >端口</label
              >
              <input
                v-model.number="form.port"
                type="number"
                placeholder="端口号"
                class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
                min="1"
                max="65535"
              />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-xs font-medium text-text-secondary mb-1.5"
                >用户名 <span class="text-text-muted">(可选)</span></label
              >
              <input
                v-model="form.username"
                type="text"
                placeholder="登录用户名"
                class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
              />
            </div>
            <div>
              <label class="block text-xs font-medium text-text-secondary mb-1.5"
                >密码 <span class="text-text-muted">(可选)</span></label
              >
              <div class="relative">
                <input
                  v-model="form.password"
                  :type="showPassword ? 'text' : 'password'"
                  placeholder="登录密码"
                  class="w-full px-3 py-2 pr-10 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
                />
                <button
                  type="button"
                  @click="showPassword = !showPassword"
                  class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text-primary transition-colors"
                >
                  <svg
                    v-if="!showPassword"
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
                  <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    class="w-4 h-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"
                    />
                    <line x1="1" y1="1" x2="23" y2="23" />
                  </svg>
                </button>
              </div>
            </div>
          </div>
          <div>
            <label class="block text-xs font-medium text-text-secondary mb-1.5"
              >Tag <span class="text-text-muted">(可选)</span></label
            >
            <input
              v-model="form.tag"
              type="text"
              placeholder="设备标签"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
          </div>
          <div
            v-if="errorMessage"
            class="px-3 py-2 text-sm text-error bg-error-bg border border-error/30 rounded-lg"
          >
            {{ errorMessage }}
          </div>
          <div class="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              @click="closeModal"
              class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              :disabled="isSaving"
              class="px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent/90 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {{ isSaving ? "保存中..." : "确定" }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- 批量编辑弹窗 -->
    <div
      v-if="showBatchModal"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="absolute inset-0 bg-black/50 backdrop-blur-sm"
        @click="closeBatchModal"
      ></div>
      <div
        class="relative bg-bg-card border border-border rounded-xl shadow-2xl w-full max-w-sm mx-4 animate-slide-in"
      >
        <div
          class="flex items-center justify-between px-6 py-4 border-b border-border"
        >
          <h3 class="text-lg font-semibold text-text-primary">
            批量修改{{ batchFieldLabel }}
            <span
              v-if="selectedCount > 0"
              class="ml-2 text-sm font-normal text-accent"
            >
              ({{ selectedCount }} 台设备)
            </span>
          </h3>
          <button
            @click="closeBatchModal"
            class="p-1 text-text-muted hover:text-text-primary transition-colors"
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
        <form @submit.prevent="saveBatchEdit" class="p-6 space-y-4">
          <p class="text-sm text-text-secondary">
            将{{ selectedCount > 0 ? '选中的' : '所有' }}设备的{{ batchFieldLabel }}修改为：
          </p>

          <!-- 协议选择 -->
          <div v-if="batchField === 'protocol'">
            <select
              v-model="batchValue"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary focus:border-accent focus:outline-none transition-colors"
            >
              <option v-for="p in validProtocols" :key="p" :value="p">
                {{ p }}
              </option>
            </select>
          </div>

          <!-- 端口输入 -->
          <div v-else-if="batchField === 'port'">
            <input
              v-model.number="batchValue"
              type="number"
              placeholder="端口号"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
              min="1"
              max="65535"
            />
          </div>

          <!-- 用户名输入 -->
          <div v-else-if="batchField === 'username'">
            <input
              v-model="batchValue"
              type="text"
              :placeholder="'请输入' + batchFieldLabel"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
          </div>

          <!-- 密码输入 -->
          <div v-else-if="batchField === 'password'">
            <input
              v-model="batchValue"
              type="password"
              :placeholder="'请输入' + batchFieldLabel"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
          </div>

          <!-- 分组输入 -->
          <div v-else-if="batchField === 'group'">
            <input
              v-model="batchValue"
              type="text"
              :placeholder="'请输入' + batchFieldLabel"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
          </div>

          <!-- 标签输入 -->
          <div v-else-if="batchField === 'tag'">
            <input
              v-model="batchValue"
              type="text"
              :placeholder="'请输入' + batchFieldLabel"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
          </div>

          <div
            v-if="batchErrorMessage"
            class="px-3 py-2 text-sm text-error bg-error-bg border border-error/30 rounded-lg"
          >
            {{ batchErrorMessage }}
          </div>
          <div class="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              @click="closeBatchModal"
              class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              :disabled="isBatchSaving"
              class="px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent/90 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {{ isBatchSaving ? "保存中..." : "确定" }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- 删除确认弹窗 -->
    <div
      v-if="showDeleteConfirm"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="absolute inset-0 bg-black/50 backdrop-blur-sm"
        @click="showDeleteConfirm = false"
      ></div>
      <div
        class="relative bg-bg-card border border-border rounded-xl shadow-2xl w-full max-w-sm mx-4 animate-slide-in"
      >
        <div class="p-6">
          <div class="flex items-center gap-3 mb-4">
            <div class="p-2 bg-error-bg rounded-lg">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-6 h-6 text-error"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-text-primary">确认删除</h3>
              <p class="text-sm text-text-muted">此操作不可撤销</p>
            </div>
          </div>
          <p class="text-sm text-text-secondary mb-6">
            确定要删除设备
            <span class="font-mono text-accent">{{ deviceToDelete?.ip }}</span>
            吗？
          </p>
          <div class="flex items-center justify-end gap-3">
            <button
              @click="showDeleteConfirm = false"
              class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
            >
              取消
            </button>
            <button
              @click="deleteDevice"
              :disabled="isDeleting"
              class="px-4 py-2 text-sm font-medium text-white bg-error hover:bg-error/90 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {{ isDeleting ? "删除中..." : "删除" }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 批量删除确认弹窗 -->
    <div
      v-if="showBatchDeleteConfirm"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="absolute inset-0 bg-black/50 backdrop-blur-sm"
        @click="showBatchDeleteConfirm = false"
      ></div>
      <div
        class="relative bg-bg-card border border-border rounded-xl shadow-2xl w-full max-w-sm mx-4 animate-slide-in"
      >
        <div class="p-6">
          <div class="flex items-center gap-3 mb-4">
            <div class="p-2 bg-error-bg rounded-lg">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-6 h-6 text-error"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-text-primary">批量删除确认</h3>
              <p class="text-sm text-text-muted">此操作不可撤销</p>
            </div>
          </div>
          <p class="text-sm text-text-secondary mb-6">
            确定要删除选中的
            <span class="font-mono text-accent font-bold">{{ selectedCount }}</span>
            台设备吗？
          </p>
          <div class="flex items-center justify-end gap-3">
            <button
              @click="showBatchDeleteConfirm = false"
              class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
            >
              取消
            </button>
            <button
              @click="batchDeleteDevices"
              :disabled="isBatchDeleting"
              class="px-4 py-2 text-sm font-medium text-white bg-error hover:bg-error/90 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {{ isBatchDeleting ? "删除中..." : "确认删除" }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { Call } from "@wailsio/runtime";

interface Device {
  ip: string;
  port: number;
  protocol: string;
  username: string;
  password: string;
  group: string;
  tag: string;
}

interface IpRangeHint {
  count: number;
  start: string;
  end: string;
}

const data = ref<Device[]>([]);
const page = ref(1);
const pageSize = 10; // 修改为10条每页

// 搜索相关
const searchQuery = ref("");
const searchType = ref<"group" | "tag" | "ip">("group");
const searchOptions = [
  { value: "group", label: "分组" },
  { value: "ip", label: "IP地址" },
  { value: "tag", label: "TAG" }
];
let searchTimeout: ReturnType<typeof setTimeout> | null = null;

// 多选状态
const selectedIndexes = ref<Set<number>>(new Set());

// 弹窗状态
const showModal = ref(false);
const isEditing = ref(false);
const editingIndex = ref(-1);
const isSaving = ref(false);
const showPassword = ref(false);
const errorMessage = ref("");

// IP 语法糖
const ipRangeHint = ref<IpRangeHint | null>(null);
const ipValidationError = ref("");

// 删除确认
const showDeleteConfirm = ref(false);
const deleteIndex = ref(-1);
const deviceToDelete = ref<Device | null>(null);
const isDeleting = ref(false);

// 批量删除确认
const showBatchDeleteConfirm = ref(false);
const isBatchDeleting = ref(false);

// 批量编辑
const showBatchModal = ref(false);
const batchField = ref<"protocol" | "port" | "username" | "password" | "group" | "tag" | "">("");
const batchValue = ref<string | number>("");
const isBatchSaving = ref(false);
const batchErrorMessage = ref("");

// 协议相关
const protocolDefaultPorts = ref<Record<string, number>>({
  SSH: 22,
  SNMP: 161,
  TELNET: 23,
});
const validProtocols = ref<string[]>(["SSH", "SNMP", "TELNET"]);

// 表单数据
const form = ref<Device>({
  ip: "",
  port: 22,
  protocol: "SSH",
  username: "",
  password: "",
  group: "",
  tag: "",
});

// 记录上次的协议，用于判断端口是否需要自动更新
const lastProtocol = ref("SSH");

// 当前搜索类型的标签
const currentSearchLabel = computed(() => {
  const opt = searchOptions.find(o => o.value === searchType.value);
  return opt ? opt.label : "";
});

// 过滤后的设备数据（模糊搜索）
const filteredData = computed(() => {
  // 如果没有搜索内容，返回所有数据
  if (!searchQuery.value.trim()) {
    return data.value;
  }

  const query = searchQuery.value.toLowerCase().trim();
  
  return data.value.filter(device => {
    let searchValue = "";
    
    switch (searchType.value) {
      case "group":
        searchValue = (device.group || "").toLowerCase();
        break;
      case "tag":
        searchValue = (device.tag || "").toLowerCase();
        break;
      case "ip":
        searchValue = (device.ip || "").toLowerCase();
        break;
    }
    
    // 模糊搜索：包含即可匹配
    return searchValue.includes(query);
  });
});

const totalPages = computed(() =>
  Math.max(1, Math.ceil(filteredData.value.length / pageSize)),
);
const pagedData = computed(() => {
  const start = (page.value - 1) * pageSize;
  return filteredData.value.slice(start, start + pageSize);
});

// 批量编辑字段标签
const batchFieldLabel = computed(() => {
  const labels: Record<string, string> = {
    protocol: "协议",
    port: "端口",
    username: "用户名",
    password: "密码",
    group: "分组",
    tag: "标签",
  };
  return labels[batchField.value] || "";
});

// 多选相关计算属性
const selectedCount = computed(() => selectedIndexes.value.size);

// 是否选中了全部设备（所有页）
const isAllSelected = computed(() => {
  if (data.value.length === 0) return false;
  return selectedIndexes.value.size === data.value.length;
});

// 是否部分选中（有选中但未全选）
const isIndeterminate = computed(() => {
  const count = selectedIndexes.value.size;
  return count > 0 && count < data.value.length;
});

// 监听 IP 输入，解析语法糖并验证
watch(
  () => form.value.ip,
  (newIp) => {
    ipRangeHint.value = parseIpRange(newIp);
    // 实时验证 IP 输入
    const validation = validateIpInput(newIp);
    ipValidationError.value = validation.error;
  },
);

// 监听搜索输入，实现防抖
watch(searchQuery, (_newQuery) => {
  // 清除之前的定时器
  if (searchTimeout) {
    clearTimeout(searchTimeout);
  }

  // 防抖：300ms 后执行搜索
  searchTimeout = setTimeout(() => {
    // 搜索时重置到第一页
    page.value = 1;
  }, 300);
});

// 监听搜索类型变化，重置页码
watch(searchType, () => {
  page.value = 1;
});

// 重置搜索
function resetSearch() {
  searchQuery.value = "";
  page.value = 1;
}

// 验证单个 IP 地址格式
function isValidIp(ip: string): boolean {
  const parts = ip.split(".");
  if (parts.length !== 4) return false;
  return parts.every((part) => {
    if (part === "" || part.length > 3) return false;
    const num = parseInt(part, 10);
    return !isNaN(num) && num >= 0 && num <= 255 && part === num.toString();
  });
}

// 验证 IP 输入（单个 IP 或语法糖格式）
function validateIpInput(ip: string): { valid: boolean; error: string } {
  if (!ip) return { valid: false, error: "" };

  // 检查是否为单个有效 IP
  if (isValidIp(ip)) {
    return { valid: true, error: "" };
  }

  // 检查是否为有效的语法糖格式
  const rangeHint = parseIpRange(ip);
  if (rangeHint) {
    return { valid: true, error: "" };
  }

  // 检查语法糖格式的具体错误
  const rangeMatch = ip.match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$/);
  if (rangeMatch && rangeMatch[2] && rangeMatch[3]) {
    const start = parseInt(rangeMatch[2], 10);
    const end = parseInt(rangeMatch[3], 10);
    // 检查 IP 段值是否有效
    if (start > 255 || end > 255) {
      return { valid: false, error: "IP 段值必须在 0-255 范围内" };
    }
    if (start >= end) {
      return { valid: false, error: "起始值必须小于结束值" };
    }
  }

  // 其他无效格式
  return {
    valid: false,
    error: "请输入有效 IP（如 192.168.1.10）或范围格式（如 192.168.1.10-20）",
  };
}

// 解析 IP 范围语法糖
function parseIpRange(ip: string): IpRangeHint | null {
  if (!ip) return null;

  // 匹配 192.168.1.10-20 格式
  const match = ip.match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$/);
  if (match && match[1] && match[2] && match[3]) {
    const prefix = match[1];
    const start = parseInt(match[2], 10);
    const end = parseInt(match[3], 10);

    if (start < end && start >= 0 && end <= 255) {
      return {
        count: end - start + 1,
        start: prefix + start,
        end: prefix + end,
      };
    }
  }

  return null;
}

// 加载设备列表
async function loadDevices() {
  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const devices: any = await Call.ByName(
      "github.com/NetWeaverGo/core/internal/ui.AppService.ListDevices",
    );
    if (devices && Array.isArray(devices)) {
      data.value = devices.map((d: any) => ({
        ip: d.ip || d.IP || "",
        port: d.port || d.Port || 22,
        protocol: d.protocol || d.Protocol || "SSH",
        username: d.username || d.Username || "",
        password: d.password || d.Password || "",
        group: d.group || d.Group || "",
        tag: d.tag || d.Tag || "",
      }));
    }
  } catch (e) {
    console.error("Failed to load devices", e);
  }
}

// 加载协议配置
async function loadProtocolConfig() {
  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const ports: any = await Call.ByName(
      "github.com/NetWeaverGo/core/internal/ui.AppService.GetProtocolDefaultPorts",
    );
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const protocols: any = await Call.ByName(
      "github.com/NetWeaverGo/core/internal/ui.AppService.GetValidProtocols",
    );
    if (ports) protocolDefaultPorts.value = ports;
    if (protocols) validProtocols.value = protocols;
  } catch (e) {
    console.error("Failed to load protocol config", e);
  }
}

// 协议切换处理
function onProtocolChange() {
  const oldDefaultPort = protocolDefaultPorts.value[lastProtocol.value] || 22;
  const newDefaultPort = protocolDefaultPorts.value[form.value.protocol] || 22;

  // 只有当前端口是旧协议的默认端口时才自动更新
  if (form.value.port === oldDefaultPort) {
    form.value.port = newDefaultPort;
  }

  lastProtocol.value = form.value.protocol;
}

// 打开新增弹窗
function openAddModal() {
  isEditing.value = false;
  editingIndex.value = -1;
  form.value = {
    ip: "",
    port: 22,
    protocol: "SSH",
    username: "",
    password: "",
    group: "",
    tag: "",
  };
  lastProtocol.value = "SSH";
  errorMessage.value = "";
  showPassword.value = false;
  ipRangeHint.value = null;
  ipValidationError.value = "";
  showModal.value = true;
}

// 打开编辑弹窗
function openEditModal(idx: number) {
  const actualIdx = (page.value - 1) * pageSize + idx;
  const device = data.value[actualIdx];
  if (!device) return;

  isEditing.value = true;
  editingIndex.value = actualIdx;
  form.value = { ...device };
  lastProtocol.value = device.protocol;
  errorMessage.value = "";
  showPassword.value = false;
  ipRangeHint.value = null;
  ipValidationError.value = "";
  showModal.value = true;
}

// 关闭弹窗
function closeModal() {
  showModal.value = false;
  errorMessage.value = "";
  ipRangeHint.value = null;
  ipValidationError.value = "";
}

// 保存设备
async function saveDevice() {
  errorMessage.value = "";
  isSaving.value = true;

  try {
    // 验证 IP 输入
    const validation = validateIpInput(form.value.ip);
    if (!validation.valid) {
      errorMessage.value = validation.error || "请输入有效的 IP 地址";
      isSaving.value = false;
      return;
    }

    if (isEditing.value) {
      // 编辑模式 - 单设备
      await Call.ByName(
        "github.com/NetWeaverGo/core/internal/ui.AppService.UpdateDevice",
        editingIndex.value,
        form.value,
      );
    } else {
      // 新增模式 - 检查语法糖
      if (ipRangeHint.value) {
        // 批量新增设备
        const match = form.value.ip.match(
          /^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$/,
        );
        if (match && match[1] && match[2] && match[3]) {
          const prefix = match[1];
          const start = parseInt(match[2], 10);
          const end = parseInt(match[3], 10);

          const newDevices: Device[] = [];
          for (let i = start; i <= end; i++) {
            newDevices.push({
              ip: prefix + i,
              port: form.value.port,
              protocol: form.value.protocol,
              username: form.value.username,
              password: form.value.password,
              group: form.value.group,
              tag: form.value.tag,
            });
          }

          // 合并现有设备并保存
          const allDevices = [...data.value, ...newDevices];
          await Call.ByName(
            "github.com/NetWeaverGo/core/internal/ui.AppService.SaveDevices",
            allDevices,
          );
        }
      } else {
        // 单设备新增
        await Call.ByName(
          "github.com/NetWeaverGo/core/internal/ui.AppService.AddDevice",
          form.value,
        );
      }
    }

    await loadDevices();
    closeModal();
  } catch (e: any) {
    errorMessage.value = e.message || "保存失败";
  } finally {
    isSaving.value = false;
  }
}

// 打开批量编辑弹窗
function openBatchEditModal(
  field: "protocol" | "port" | "username" | "password" | "group" | "tag",
) {
  batchField.value = field;
  batchErrorMessage.value = "";

  // 设置默认值
  if (field === "protocol") {
    batchValue.value = "SSH";
  } else if (field === "port") {
    batchValue.value = 22;
  } else {
    batchValue.value = "";
  }

  showBatchModal.value = true;
}

// 关闭批量编辑弹窗
function closeBatchModal() {
  showBatchModal.value = false;
  batchErrorMessage.value = "";
}

// 保存批量编辑
async function saveBatchEdit() {
  if (data.value.length === 0) {
    batchErrorMessage.value = "没有可修改的设备";
    return;
  }

  batchErrorMessage.value = "";
  isBatchSaving.value = true;

  try {
    // 判断是修改选中的设备还是所有设备
    const indexesToModify = selectedCount.value > 0
      ? new Set(selectedIndexes.value)  // 选中的索引
      : new Set(data.value.keys());      // 所有索引

    // 复制设备列表，只修改目标设备
    const updatedDevices = data.value.map((d, idx) => {
      // 不在修改范围内，保持原样
      if (!indexesToModify.has(idx)) {
        return d;
      }

      const newDevice = { ...d };
      if (batchField.value === "protocol") {
        const newProtocol = batchValue.value as string;
        const oldDefaultPort = protocolDefaultPorts.value[d.protocol] || 22;
        const newDefaultPort = protocolDefaultPorts.value[newProtocol] || 22;
        newDevice.protocol = newProtocol;
        // 如果当前端口是旧协议的默认端口，则同步更新为新协议的默认端口
        if (d.port === oldDefaultPort) {
          newDevice.port = newDefaultPort;
        }
      } else if (batchField.value === "port") {
        newDevice.port = batchValue.value as number;
      } else if (batchField.value === "username") {
        newDevice.username = batchValue.value as string;
      } else if (batchField.value === "password") {
        newDevice.password = batchValue.value as string;
      } else if (batchField.value === "group") {
        newDevice.group = batchValue.value as string;
      } else if (batchField.value === "tag") {
        newDevice.tag = batchValue.value as string;
      }
      return newDevice;
    });

    await Call.ByName(
      "github.com/NetWeaverGo/core/internal/ui.AppService.SaveDevices",
      updatedDevices,
    );
    await loadDevices();
    closeBatchModal();
    // 保存成功后清空选择
    clearSelection();
  } catch (e: any) {
    batchErrorMessage.value = e.message || "保存失败";
  } finally {
    isBatchSaving.value = false;
  }
}

// 确认删除
function confirmDelete(idx: number) {
  const actualIdx = (page.value - 1) * pageSize + idx;
  const device = data.value[actualIdx];
  if (!device) return;

  deleteIndex.value = actualIdx;
  deviceToDelete.value = device;
  showDeleteConfirm.value = true;
}

// 执行删除
async function deleteDevice() {
  isDeleting.value = true;

  try {
    await Call.ByName(
      "github.com/NetWeaverGo/core/internal/ui.AppService.DeleteDevice",
      deleteIndex.value,
    );
    await loadDevices();
    showDeleteConfirm.value = false;

    // 如果删除后当前页没有数据，跳转到上一页
    if (pagedData.value.length === 0 && page.value > 1) {
      page.value--;
    }
  } catch (e: any) {
    console.error("Delete failed:", e);
  } finally {
    isDeleting.value = false;
  }
}

// 协议徽章样式
function getProtocolBadgeClass(protocol: string) {
  const classes: Record<string, string> = {
    SSH: "bg-success-bg text-success",
    SNMP: "bg-info-bg text-info",
    TELNET: "bg-warning-bg text-warning",
  };
  return classes[protocol] || "bg-bg-hover text-text-muted";
}

// 多选相关函数

// 判断某行是否被选中
function isSelected(idx: number): boolean {
  const actualIdx = (page.value - 1) * pageSize + idx;
  return selectedIndexes.value.has(actualIdx);
}

// 切换单行选中状态
function toggleSelect(idx: number) {
  const actualIdx = (page.value - 1) * pageSize + idx;
  if (selectedIndexes.value.has(actualIdx)) {
    selectedIndexes.value.delete(actualIdx);
  } else {
    selectedIndexes.value.add(actualIdx);
  }
}

// 切换全选/取消全选（全量设备）
function toggleSelectAll() {
  if (isAllSelected.value) {
    // 取消所有选中
    selectedIndexes.value.clear();
  } else {
    // 选中所有设备
    for (let i = 0; i < data.value.length; i++) {
      selectedIndexes.value.add(i);
    }
  }
}

// 清空所有选择
function clearSelection() {
  selectedIndexes.value.clear();
}

// 打开批量删除确认弹窗
function confirmBatchDelete() {
  showBatchDeleteConfirm.value = true;
}

// 执行批量删除
async function batchDeleteDevices() {
  isBatchDeleting.value = true;

  try {
    // 获取要删除的索引，按降序排列以便从后往前删除
    const indexesToDelete = Array.from(selectedIndexes.value).sort((a, b) => b - a);

    // 从后往前删除设备
    for (const idx of indexesToDelete) {
      await Call.ByName(
        "github.com/NetWeaverGo/core/internal/ui.AppService.DeleteDevice",
        idx,
      );
    }

    await loadDevices();
    showBatchDeleteConfirm.value = false;
    clearSelection();

    // 如果删除后当前页没有数据，跳转到上一页
    if (pagedData.value.length === 0 && page.value > 1) {
      page.value--;
    }
  } catch (e: any) {
    console.error("Batch delete failed:", e);
  } finally {
    isBatchDeleting.value = false;
  }
}

onMounted(() => {
  loadDevices();
  loadProtocolConfig();
});
</script>
