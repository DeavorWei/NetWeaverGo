<script setup lang="ts">
import { ref, watch } from "vue";
import { ForgeAPI } from "../../services/api";

type CalcRecord = { label: string; value: string };
type InclusionState = { isIncluded: boolean; message: string } | null;
type IPv6SubnetRow = {
  index: number;
  network: string;
  cidr: number;
  isIncluded: boolean;
};

const ipv6Str = ref("");
const prefixStr = ref("");
const v6CheckIpStr = ref("");
const v6NewPrefixStr = ref("");

const evaluation = ref<{ error: string | null; records: CalcRecord[] }>({
  error: null,
  records: [],
});

const inclusionCheck = ref<InclusionState>(null);

const v6SubnetEvaluation = ref<{
  error: string | null;
  warning: string | null;
  subnets: IPv6SubnetRow[];
  total: number;
}>({
  error: null,
  warning: null,
  subnets: [],
  total: 0,
});

let calcTicket = 0;
const calculateFromBackend = async () => {
  const ticket = ++calcTicket;
  try {
    const result = await ForgeAPI.calculateIPv6({
      ip: ipv6Str.value,
      prefix: prefixStr.value,
      checkIp: v6CheckIpStr.value,
      newPrefix: v6NewPrefixStr.value,
    });

    if (ticket !== calcTicket) {
      return;
    }

    if (!result) {
      evaluation.value = { error: null, records: [] };
      inclusionCheck.value = null;
      v6SubnetEvaluation.value = {
        error: null,
        warning: null,
        subnets: [],
        total: 0,
      };
      return;
    }

    evaluation.value = {
      error: result.baseError || null,
      records: result.baseRecords || [],
    };

    inclusionCheck.value = result.inclusionCheck || null;

    v6SubnetEvaluation.value = {
      error: result.subnetError || null,
      warning: result.subnetWarning || null,
      subnets: result.subnets || [],
      total: result.totalSubnets || 0,
    };
  } catch {
    if (ticket !== calcTicket) {
      return;
    }
    evaluation.value = { error: "后端计算失败，请重试", records: [] };
    inclusionCheck.value = null;
    v6SubnetEvaluation.value = {
      error: null,
      warning: null,
      subnets: [],
      total: 0,
    };
  }
};

watch(
  [ipv6Str, prefixStr, v6CheckIpStr, v6NewPrefixStr],
  () => {
    void calculateFromBackend();
  },
  { immediate: true },
);

const copyText = (val: string) => {
  if (val && val !== "N/A") {
    navigator.clipboard.writeText(val);
  }
};
</script>

<template>
  <div class="space-y-4">
    <section
      class="bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4 md:p-5"
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
        IPv6 参数输入
      </h2>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-medium text-text-secondary"
            >IPv6 地址</label
          >
          <input
            type="text"
            v-model="ipv6Str"
            placeholder="例如: 2001:db8::1"
            class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono text-sm text-text-primary placeholder-text-muted"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-medium text-text-secondary"
            >前缀长度 (/n)</label
          >
          <input
            type="text"
            v-model="prefixStr"
            placeholder="例如: 64"
            class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono text-sm text-text-primary placeholder-text-muted"
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
            子网划分: 目标新前缀
          </label>
          <div class="relative">
            <input
              type="text"
              v-model="v6NewPrefixStr"
              placeholder="例如: 68"
              class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono text-sm text-text-primary placeholder-text-muted"
            />
            <button
              v-if="v6NewPrefixStr"
              @click="v6NewPrefixStr = ''"
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
          <p class="text-xs text-text-muted mt-1">
            注意: 出于性能考虑，单次计算最多允许分配 16 位深度
          </p>
        </div>

        <div class="flex flex-col gap-1.5">
          <label
            class="text-xs font-medium text-text-secondary flex items-center justify-between"
          >
            地址包含关系检查 (可选)
          </label>
          <div class="relative">
            <input
              type="text"
              v-model="v6CheckIpStr"
              placeholder="在此输入待检查的 IPv6 地址"
              class="px-3 py-2 bg-bg-tertiary/50 border border-border rounded-lg focus:ring-2 focus:ring-accent focus:border-accent outline-none transition-all w-full font-mono text-sm text-text-primary placeholder-text-muted"
            />
            <button
              v-if="v6CheckIpStr"
              @click="v6CheckIpStr = ''"
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
          <p
            v-if="inclusionCheck"
            :class="[
              'text-xs font-semibold mt-1',
              inclusionCheck.isIncluded ? 'text-success' : 'text-error',
            ]"
          >
            {{ inclusionCheck.message }}
          </p>
        </div>
      </div>
    </section>

    <!-- 错误提示 -->
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

    <transition name="fade">
      <div
        v-if="v6SubnetEvaluation.error"
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
        <span class="text-sm font-medium">{{ v6SubnetEvaluation.error }}</span>
      </div>
    </transition>

    <!-- 划分子网结果展示 -->
    <transition name="fade">
      <section
        v-if="v6SubnetEvaluation.subnets.length > 0"
        class="bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4 md:p-5"
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
            IPv6 子网划分列表
          </h2>
          <div class="flex items-center gap-2">
            <span
              class="text-xs font-medium text-text-secondary bg-bg-tertiary px-3 py-1 rounded-full border border-border"
            >
              共划分出
              <span class="text-accent font-bold mx-1">{{
                v6SubnetEvaluation.total.toLocaleString()
              }}</span>
              个网段
            </span>
          </div>
        </div>

        <div
          v-if="v6SubnetEvaluation.warning"
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
              v6SubnetEvaluation.warning
            }}</span>
          </div>
        </div>

        <div
          class="overflow-x-auto rounded-lg border border-border bg-bg-tertiary max-h-[400px] overflow-y-auto scrollbar-custom"
        >
          <table class="w-full text-left border-collapse min-w-[500px]">
            <thead
              class="sticky top-0 bg-bg-tertiary backdrop-blur shadow-sm z-10 border-b border-border"
            >
              <tr>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider w-20"
                >
                  序号
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider"
                >
                  IPv6 网络号
                </th>
                <th
                  class="px-3 py-2 text-xs font-semibold text-text-muted uppercase tracking-wider w-20"
                >
                  CIDR
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr
                v-for="item in v6SubnetEvaluation.subnets"
                :key="item.index"
                :class="[
                  'transition-colors',
                  item.isIncluded
                    ? 'bg-success-bg hover:bg-success-bg'
                    : 'hover:bg-bg-hover',
                ]"
              >
                <td
                  class="px-3 py-1.5 text-xs text-text-secondary tabular-nums"
                >
                  <div class="flex items-center">
                    <span
                      v-if="item.isIncluded"
                      class="w-1.5 h-1.5 rounded-full bg-success mr-2"
                      style="box-shadow: var(--shadow-success-glow);"
                    ></span>
                    {{ item.index }}
                  </div>
                </td>
                <td
                  :class="[
                    'px-3 py-1.5 text-xs font-mono font-medium',
                    item.isIncluded ? 'text-success' : 'text-text-primary',
                  ]"
                >
                  {{ item.network }}
                </td>
                <td
                  :class="[
                    'px-3 py-1.5 text-xs font-mono',
                    item.isIncluded ? 'text-success' : 'text-text-secondary',
                  ]"
                >
                  /{{ item.cidr }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </transition>

    <!-- 基础计算结果展示 -->
    <transition name="fade">
      <section
        v-if="evaluation.records.length > 0"
        class="bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4 md:p-5"
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
          分析结果
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
                <td
                  class="px-3 py-2 text-xs text-text-primary font-mono break-all"
                >
                  {{ record.value }}
                </td>
                <td class="px-3 py-2 text-right w-16">
                  <button
                    @click="copyText(record.value || '')"
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


