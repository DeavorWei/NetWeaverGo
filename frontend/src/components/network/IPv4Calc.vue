<script setup lang="ts">
import { ref, watch } from "vue";
import { ForgeAPI } from "../../services/api";

type CalcRecord = { label: string; value: string };
type IPv4SubnetRow = {
  index: number;
  network: string;
  cidr: number;
  firstUsable: string;
  lastUsable: string;
  broadcast: string;
  mask: string;
};

const ipStr = ref("");
const maskStr = ref("");
const hostCountStr = ref("");
const subnetCountStr = ref("");
const forceDisplayAllSubnets = ref(false);

const evaluation = ref<{ error: string | null; records: CalcRecord[] }>({
  error: null,
  records: [],
});

const subnettingEvaluation = ref<{
  error: string | null;
  warning: string | null;
  showForceButton: boolean;
  subnets: IPv4SubnetRow[];
  totalSubnets: number;
}>({
  error: null,
  warning: null,
  showForceButton: false,
  subnets: [],
  totalSubnets: 0,
});

let calcTicket = 0;
const calculateFromBackend = async () => {
  const ticket = ++calcTicket;
  try {
    const result = await ForgeAPI.calculateIPv4({
      ip: ipStr.value,
      mask: maskStr.value,
      hostCount: hostCountStr.value,
      subnetCount: subnetCountStr.value,
      forceDisplayAllSubnets: forceDisplayAllSubnets.value,
    });

    if (ticket !== calcTicket) {
      return;
    }

    if (!result) {
      evaluation.value = { error: null, records: [] };
      subnettingEvaluation.value = {
        error: null,
        warning: null,
        showForceButton: false,
        subnets: [],
        totalSubnets: 0,
      };
      return;
    }

    evaluation.value = {
      error: result.baseError || null,
      records: result.baseRecords || [],
    };

    subnettingEvaluation.value = {
      error: result.subnetError || null,
      warning: result.subnetWarning || null,
      showForceButton: !!result.showForceButton,
      subnets: result.subnets || [],
      totalSubnets: result.totalSubnets || 0,
    };
  } catch {
    if (ticket !== calcTicket) {
      return;
    }
    evaluation.value = { error: "后端计算失败，请重试", records: [] };
    subnettingEvaluation.value = {
      error: null,
      warning: null,
      showForceButton: false,
      subnets: [],
      totalSubnets: 0,
    };
  }
};

watch([ipStr, maskStr, hostCountStr, subnetCountStr], () => {
  forceDisplayAllSubnets.value = false;
});

watch(
  [ipStr, maskStr, hostCountStr, subnetCountStr, forceDisplayAllSubnets],
  () => {
    void calculateFromBackend();
  },
  { immediate: true },
);

watch(ipStr, (newVal) => {
  if (!newVal) return;
  const sanitized = newVal.replace(/[^0-9.]/g, "");
  if (sanitized !== newVal) {
    ipStr.value = sanitized;
  }
});

watch(hostCountStr, (newVal) => {
  if (!newVal) return;
  const sanitized = newVal.replace(/[^0-9]/g, "");
  if (sanitized !== newVal) {
    hostCountStr.value = sanitized;
  }
});

watch(subnetCountStr, (newVal) => {
  if (!newVal) return;
  const sanitized = newVal.replace(/[^0-9]/g, "");
  if (sanitized !== newVal) {
    subnetCountStr.value = sanitized;
  }
});

const copyText = (val: string) => {
  if (val && val !== "N/A") {
    navigator.clipboard.writeText(val);
  }
};

const exportToCSV = () => {
  void (async () => {
    try {
      const exportResult = await ForgeAPI.exportIPv4SubnetsCSV({
        ip: ipStr.value,
        mask: maskStr.value,
        hostCount: hostCountStr.value,
        subnetCount: subnetCountStr.value,
        forceDisplayAllSubnets: forceDisplayAllSubnets.value,
      });

      if (!exportResult || !exportResult.content) {
        return;
      }

      const blob = new Blob([exportResult.content], {
        type: "text/csv;charset=utf-8;",
      });
      const url = URL.createObjectURL(blob);

      const link = document.createElement("a");
      link.setAttribute("href", url);
      link.setAttribute(
        "download",
        exportResult.fileName || "子网划分明细.csv",
      );
      link.style.visibility = "hidden";

      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch {
      // 导出失败时静默处理，避免影响页面交互
    }
  })();
};
</script>

<template>
  <div class="space-y-4">
    <!-- 输入模块 -->
    <section
      class="bg-bg-secondary/60 glass border border-border rounded-xl shadow-card p-4 md:p-5"
    >
      <h2
        class="text-sm font-semibold text-text-primary mb-3 flex items-center"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 mr-2 text-accent"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"
          />
        </svg>
        参数输入
      </h2>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-medium text-text-secondary">IP 地址</label>
          <input
            type="text"
            v-model="ipStr"
            placeholder="例如: 192.168.1.10"
            class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono text-sm text-text-primary placeholder-text-muted"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-medium text-text-secondary"
            >子网掩码
            <span class="font-normal text-text-muted"
              >(CIDR/十进制/反掩码)</span
            ></label
          >
          <input
            type="text"
            v-model="maskStr"
            placeholder="例如: 24 或 255.255.255.0"
            class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono text-text-primary placeholder-text-muted"
          />
        </div>
      </div>

      <div
        class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4 pt-4 border-t border-border"
      >
        <div class="flex flex-col gap-1.5">
          <label
            class="text-xs font-medium text-text-secondary flex items-center justify-between"
          >
            选项一：需划分的主机数
            <span
              v-if="subnetCountStr.length > 0"
              class="text-xs text-warning font-medium"
              >已被互斥禁用</span
            >
          </label>
          <div class="relative">
            <input
              type="text"
              v-model="hostCountStr"
              :disabled="subnetCountStr.length > 0"
              placeholder="例如: 50"
              class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono disabled:opacity-50 disabled:cursor-not-allowed text-text-primary placeholder-text-muted"
            />
            <button
              v-if="hostCountStr"
              @click="hostCountStr = ''"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-error transition-colors p-1"
              title="清除输入"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                viewBox="0 0 20 20"
                fill="currentColor"
              >
                <path
                  fill-rule="evenodd"
                  d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                  clip-rule="evenodd"
                />
              </svg>
            </button>
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <label
            class="text-xs font-medium text-text-secondary flex items-center justify-between"
          >
            选项二：需划分的小子网数
            <span
              v-if="hostCountStr.length > 0"
              class="text-xs text-warning font-medium"
              >已被互斥禁用</span
            >
          </label>
          <div class="relative">
            <input
              type="text"
              v-model="subnetCountStr"
              :disabled="hostCountStr.length > 0"
              placeholder="例如: 4"
              class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono disabled:opacity-50 disabled:cursor-not-allowed text-text-primary placeholder-text-muted"
            />
            <button
              v-if="subnetCountStr"
              @click="subnetCountStr = ''"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-error transition-colors p-1"
              title="清除输入"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                viewBox="0 0 20 20"
                fill="currentColor"
              >
                <path
                  fill-rule="evenodd"
                  d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                  clip-rule="evenodd"
                />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- 错误提示 (原有逻辑，现统一展示在最前) -->
    <transition name="fade">
      <div
        v-if="evaluation.error"
        class="bg-error-bg text-error border border-error/30 p-3 rounded-lg flex items-center shadow-sm"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 mr-3 flex-shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <span class="text-sm font-medium">{{ evaluation.error }}</span>
      </div>
    </transition>

    <!-- 划分子网错误提示 -->
    <transition name="fade">
      <div
        v-if="subnettingEvaluation.error"
        class="bg-error-bg text-error border border-error/30 p-3 rounded-lg flex items-center shadow-sm"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 mr-3 flex-shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <span class="text-sm font-medium">{{
          subnettingEvaluation.error
        }}</span>
      </div>
    </transition>

    <!-- 划分子网结果展示 (排在计算结果之前) -->
    <transition name="fade">
      <section
        v-if="subnettingEvaluation.subnets.length > 0"
        class="bg-bg-secondary/60 glass border border-border rounded-xl shadow-card p-4 md:p-5"
      >
        <div
          class="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-3 gap-2"
        >
          <h2 class="text-sm font-semibold text-text-primary flex items-center">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 mr-2 text-accent"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"
              />
            </svg>
            子网划分列表
          </h2>
          <div class="flex items-center gap-2">
            <span
              class="text-xs font-medium text-text-secondary bg-bg-tertiary px-3 py-1 rounded-full border border-border"
            >
              共划分出
              <span class="text-accent font-bold mx-1">{{
                subnettingEvaluation.totalSubnets.toLocaleString()
              }}</span>
              个网段
            </span>
            <button
              @click="exportToCSV"
              class="flex items-center text-xs font-medium text-white bg-accent hover:bg-accent/90 px-3 py-1 rounded-full shadow-glow transition-all focus:ring-2 focus:ring-accent outline-none"
              title="导出为 CSV (Excel可打开)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4 mr-1.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                />
              </svg>
              下载报表
            </button>
          </div>
        </div>

        <div
          v-if="subnettingEvaluation.warning"
          class="bg-warning-bg text-warning border border-warning/30 p-3 rounded-lg flex shadow-sm mb-3 text-xs items-start justify-between"
        >
          <div class="flex items-start">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 mr-3 flex-shrink-0 mt-0.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            <span class="leading-relaxed">{{
              subnettingEvaluation.warning
            }}</span>
          </div>
          <button
            v-if="subnettingEvaluation.showForceButton"
            @click="forceDisplayAllSubnets = true"
            class="ml-3 shrink-0 px-2 py-1 bg-warning/20 hover:bg-warning/30 text-warning font-medium rounded transition-colors border border-warning/30 shadow-sm text-xs focus:ring-2 focus:ring-warning outline-none"
          >
            强制展示
          </button>
        </div>

        <div
          class="overflow-x-auto rounded-lg border border-border bg-bg-tertiary max-h-[400px] overflow-y-auto scrollbar-custom"
        >
          <table class="w-full text-left border-collapse min-w-[700px]">
            <thead
              class="sticky top-0 bg-bg-tertiary backdrop-blur shadow-sm z-10 border-b border-border"
            >
              <tr>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  序号 #
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  网络号
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  CIDR
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  首个可用 IP
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  最后可用 IP
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  广播地址
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  子网掩码
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr
                v-for="item in subnettingEvaluation.subnets"
                :key="item.index"
                class="hover:bg-bg-hover transition-colors"
              >
                <td
                  class="px-3 py-1.5 text-xs text-text-secondary tabular-nums"
                >
                  {{ item.index }}
                </td>
                <td
                  class="px-3 py-1.5 text-xs text-text-primary font-mono font-medium"
                >
                  {{ item.network }}
                </td>
                <td class="px-3 py-1.5 text-xs text-text-primary font-mono">
                  {{ item.cidr }}
                </td>
                <td class="px-3 py-1.5 text-xs text-text-primary font-mono">
                  {{ item.firstUsable }}
                </td>
                <td class="px-3 py-1.5 text-xs text-text-primary font-mono">
                  {{ item.lastUsable }}
                </td>
                <td class="px-3 py-1.5 text-xs text-text-primary font-mono">
                  {{ item.broadcast }}
                </td>
                <td class="px-3 py-1.5 text-xs text-text-primary font-mono">
                  {{ item.mask }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </transition>

    <!-- 结果展示 (排在子网列表之后) -->
    <transition name="fade">
      <section
        v-if="evaluation.records.length > 0"
        class="bg-bg-secondary/60 glass border border-border rounded-xl shadow-card p-4 md:p-5"
      >
        <h2
          class="text-sm font-semibold text-text-primary mb-3 flex items-center"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 mr-2 text-info"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
            />
          </svg>
          计算结果
        </h2>
        <div
          class="overflow-x-auto rounded-lg border border-border bg-bg-tertiary"
        >
          <table class="w-full text-left border-collapse">
            <tbody>
              <tr
                v-for="(record, idx) in evaluation.records"
                :key="idx"
                class="border-b border-border last:border-b-0 hover:bg-bg-hover transition-colors group"
              >
                <td
                  class="px-3 py-2 text-xs font-medium text-text-secondary w-[30%] sm:w-1/3"
                >
                  {{ record.label }}
                </td>
                <td class="px-3 py-2 text-xs text-text-primary font-mono">
                  {{ record.value }}
                </td>
                <td class="px-3 py-2 text-right w-16">
                  <button
                    @click="copyText(record.value)"
                    class="text-xs text-accent hover:text-white hover:bg-accent border border-accent/30 px-2 py-0.5 rounded transition-all opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 focus:opacity-100"
                  >
                    复制
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </transition>
  </div>
</template>

<style scoped>
/* fade 过渡动画已移至全局 _animations.css */
</style>
