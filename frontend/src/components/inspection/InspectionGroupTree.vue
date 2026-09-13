<template>
  <div class="border border-border rounded-lg p-3 bg-bg-secondary/30 space-y-3">
    <div class="flex items-center justify-between">
      <span class="text-xs font-semibold text-text-primary">检查项分组树</span>
      <div class="flex items-center gap-2">
        <span class="text-[11px] text-text-muted">共 {{ totalGroups }} 组</span>
        <el-button size="small" type="primary" link :icon="Plus" @click="addRootGroup">新增根分组</el-button>
      </div>
    </div>

    <div class="flex gap-3">
      <!-- 分组树 -->
      <div class="w-60 shrink-0 border border-border rounded bg-bg-panel p-2 max-h-64 overflow-auto">
        <el-tree
          v-if="groups.length > 0"
          :data="groups"
          node-key="code"
          :props="treeProps"
          default-expand-all
          highlight-current
          :current-node-key="currentCode"
          @node-click="handleNodeClick"
        >
          <template #default="{ data }">
            <span class="text-xs">
              {{ data.name || data.code }}
              <span class="text-text-muted">({{ (data.itemCodes || []).length }})</span>
            </span>
          </template>
        </el-tree>
        <div v-else class="text-[11px] text-text-muted p-2">
          暂无分组，旧模板按扁平列表展示。
        </div>
      </div>

      <!-- 节点编辑区 -->
      <div v-if="current" class="flex-1 min-w-0 space-y-2">
        <div class="grid grid-cols-2 gap-2">
          <div>
            <div class="text-[11px] text-text-muted mb-1">分组编码（唯一）</div>
            <el-input :model-value="current.code" size="small" disabled />
          </div>
          <div>
            <div class="text-[11px] text-text-muted mb-1">分组名称</div>
            <el-input v-model="form.name" size="small" placeholder="如 系统资源" />
          </div>
        </div>

        <div>
          <div class="text-[11px] text-text-muted mb-1">归属检查项</div>
          <el-select
            v-model="form.itemCodes"
            multiple
            filterable
            size="small"
            class="w-full"
            placeholder="选择该分组下的检查项编码"
          >
            <el-option v-for="code in itemCodes" :key="code" :label="code" :value="code" />
          </el-select>
        </div>

        <div>
          <div class="text-[11px] text-text-muted mb-1">父分组（变更即移动节点）</div>
          <el-select v-model="form.parentCode" clearable size="small" class="w-full" placeholder="留空为根分组">
            <el-option v-for="opt in parentOptions" :key="opt.code" :label="opt.name" :value="opt.code" />
          </el-select>
        </div>

        <div class="flex items-center gap-2 pt-1">
          <el-button size="small" type="primary" @click="saveCurrent">应用修改</el-button>
          <el-button size="small" @click="addChildGroup">新增子分组</el-button>
          <el-button size="small" type="danger" plain @click="deleteCurrent">删除该分组</el-button>
        </div>
      </div>
      <div v-else class="flex-1 text-[11px] text-text-muted">
        选择左侧分组节点进行编辑。
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { InspectionGroup } from '@/services/inspectionApi'

const props = defineProps<{
  groups: InspectionGroup[]
  itemCodes: string[]
}>()

const emit = defineEmits<{
  (e: 'update:groups', groups: InspectionGroup[]): void
}>()

const treeProps = { label: 'name', children: 'children' }

const currentCode = ref('')
const form = reactive<{ name: string; itemCodes: string[]; parentCode: string }>({
  name: '',
  itemCodes: [],
  parentCode: '',
})

const current = computed<InspectionGroup | null>(() => findNode(props.groups, currentCode.value))
const totalGroups = computed(() => countNodes(props.groups))

// 父分组候选：排除自身及其子树，避免形成环路
const parentOptions = computed(() => {
  const excluded = new Set<string>()
  if (current.value) collectCodes([current.value], excluded)
  const out: { code: string; name: string }[] = []
  flatten(props.groups, (node) => {
    if (!excluded.has(node.code)) out.push({ code: node.code, name: node.name || node.code })
  })
  return out
})

watch(
  current,
  (node) => {
    if (!node) {
      form.name = ''
      form.itemCodes = []
      form.parentCode = ''
      return
    }
    form.name = node.name || ''
    form.itemCodes = [...(node.itemCodes ?? [])]
    form.parentCode = findParentCode(props.groups, node.code) ?? ''
  },
  { immediate: true },
)

function handleNodeClick(data: InspectionGroup) {
  currentCode.value = data.code
}

// ==================== 纯函数（不可变树操作） ====================

function cloneTree(nodes: InspectionGroup[]): InspectionGroup[] {
  return (nodes ?? []).map((node) => ({
    code: node.code,
    name: node.name,
    itemCodes: [...(node.itemCodes ?? [])],
    children: cloneTree(node.children ?? []),
  }))
}

function findNode(nodes: InspectionGroup[], code: string): InspectionGroup | null {
  for (const node of nodes ?? []) {
    if (node.code === code) return node
    const hit = findNode(node.children ?? [], code)
    if (hit) return hit
  }
  return null
}

function findParentCode(nodes: InspectionGroup[], code: string): string | null {
  for (const node of nodes ?? []) {
    if ((node.children ?? []).some((child) => child.code === code)) return node.code
    const hit = findParentCode(node.children ?? [], code)
    if (hit !== null) return hit
  }
  return null
}

function removeNode(nodes: InspectionGroup[], code: string): InspectionGroup[] {
  return (nodes ?? [])
    .filter((node) => node.code !== code)
    .map((node) => ({ ...node, children: removeNode(node.children ?? [], code) }))
}

function appendChild(nodes: InspectionGroup[], parentCode: string, child: InspectionGroup): InspectionGroup[] {
  return (nodes ?? []).map((node) => {
    if (node.code === parentCode) {
      return { ...node, children: [...(node.children ?? []), child] }
    }
    return { ...node, children: appendChild(node.children ?? [], parentCode, child) }
  })
}

function countNodes(nodes: InspectionGroup[]): number {
  return (nodes ?? []).reduce((sum, node) => sum + 1 + countNodes(node.children ?? []), 0)
}

function collectCodes(nodes: InspectionGroup[], acc: Set<string>) {
  for (const node of nodes ?? []) {
    acc.add(node.code)
    collectCodes(node.children ?? [], acc)
  }
}

function flatten(nodes: InspectionGroup[], visit: (node: InspectionGroup) => void) {
  for (const node of nodes ?? []) {
    visit(node)
    flatten(node.children ?? [], visit)
  }
}

function uniqueCode(nodes: InspectionGroup[]): string {
  const used = new Set<string>()
  collectCodes(nodes, used)
  let i = 1
  while (used.has(`group_${i}`)) i++
  return `group_${i}`
}

// ==================== 交互动作 ====================

function commit(next: InspectionGroup[], focusCode = '') {
  emit('update:groups', next)
  if (focusCode) currentCode.value = focusCode
}

function addRootGroup() {
  const tree = cloneTree(props.groups)
  const code = uniqueCode(tree)
  tree.push({ code, name: `新分组 ${countNodes(tree) + 1}`, itemCodes: [], children: [] })
  commit(tree, code)
}

function addChildGroup() {
  if (!current.value) return
  const tree = cloneTree(props.groups)
  const code = uniqueCode(tree)
  const parentCode = current.value.code
  const next = appendChild(tree, parentCode, {
    code,
    name: `新子分组 ${countNodes(tree) + 1}`,
    itemCodes: [],
    children: [],
  })
  commit(next, code)
}

function saveCurrent() {
  const node = current.value
  if (!node) return

  const tree = cloneTree(props.groups)
  const moved = findNode(tree, node.code)
  if (!moved) return

  moved.name = form.name.trim() || moved.code
  moved.itemCodes = [...form.itemCodes]

  const targetParent = form.parentCode || ''
  const pruned = removeNode(tree, node.code)
  const next =
    targetParent && findNode(pruned, targetParent)
      ? appendChild(pruned, targetParent, moved)
      : [...pruned, moved]

  commit(next, node.code)
  ElMessage.success('分组已更新，请点击「保存分组」持久化')
}

function deleteCurrent() {
  const node = current.value
  if (!node) return
  const next = removeNode(cloneTree(props.groups), node.code)
  currentCode.value = ''
  commit(next)
  ElMessage.success('分组已删除，请点击「保存分组」持久化')
}
</script>
