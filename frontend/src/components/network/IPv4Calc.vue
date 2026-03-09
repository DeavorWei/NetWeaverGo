<script setup lang="ts">
import { ref, computed, watch } from "vue";

const ipStr = ref("");
const maskStr = ref("");
const hostCountStr = ref("");
const subnetCountStr = ref("");
const forceDisplayAllSubnets = ref(false);

// —— Utilities ——
const isValidIp = (ip: string) => {
  const ipRegex =
    /^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
  return ipRegex.test(ip);
};

const ipToLong = (ip: string) => {
  return (
    ip
      .split(".")
      .reduce((acc, octet) => (acc << 8) + parseInt(octet, 10), 0) >>> 0
  );
};

const longToIp = (long: number) => {
  return [
    (long >>> 24) & 255,
    (long >>> 16) & 255,
    (long >>> 8) & 255,
    long & 255,
  ].join(".");
};

const cidrToMaskLong = (cidr: number) => {
  if (cidr === 0) return 0;
  return ~(Math.pow(2, 32 - cidr) - 1) >>> 0;
};

const ipToBinary = (ip: string) => {
  return ip
    .split(".")
    .map((octet) => parseInt(octet, 10).toString(2).padStart(8, "0"))
    .join("");
};

const parseMaskOrWildcard = (maskStrInput: string) => {
  let potentialCidr = maskStrInput.startsWith("/")
    ? maskStrInput.substring(1)
    : maskStrInput;
  const cidrNum = parseInt(potentialCidr, 10);

  if (
    !isNaN(cidrNum) &&
    String(cidrNum) === potentialCidr &&
    cidrNum >= 0 &&
    cidrNum <= 32
  ) {
    return cidrNum;
  }

  if (isValidIp(maskStrInput)) {
    const maskBinary = ipToBinary(maskStrInput);
    if (/^1*0*$/.test(maskBinary)) {
      return (maskBinary.match(/1/g) || []).length;
    }
    if (/^0*1*$/.test(maskBinary)) {
      return (maskBinary.match(/0/g) || []).length;
    }
  }

  return -1;
};

// —— Computed Results ——
const calculateSubnetDetails = (ipStr: string, cidr: number) => {
  const ipLong = ipToLong(ipStr);
  const maskLong = cidrToMaskLong(cidr);
  const networkLong = ipLong & maskLong;
  const broadcastLong = networkLong | (~maskLong >>> 0);

  const hostBits = 32 - cidr;
  const totalHosts = Math.pow(2, hostBits);
  const usableHosts = cidr < 31 ? totalHosts - 2 : cidr === 31 ? 2 : 1;

  const firstUsableLong = cidr < 31 ? networkLong + 1 : networkLong;
  const lastUsableLong = cidr < 31 ? broadcastLong - 1 : broadcastLong;

  return [
    { label: "网络地址", value: longToIp(networkLong) },
    { label: "广播地址", value: longToIp(broadcastLong) },
    { label: "子网掩码", value: longToIp(maskLong) },
    { label: "反掩码", value: longToIp(~maskLong >>> 0) },
    { label: "CIDR", value: `/${cidr}` },
    {
      label: "子网范围",
      value: `${longToIp(networkLong)} - ${longToIp(broadcastLong)}`,
    },
    {
      label: "可用主机数",
      value: usableHosts > 0 ? usableHosts.toLocaleString() : "0",
    },
    {
      label: "首个可用地址",
      value: usableHosts > 0 ? longToIp(firstUsableLong) : "N/A",
    },
    {
      label: "最后一个可用地址",
      value: usableHosts > 0 ? longToIp(lastUsableLong) : "N/A",
    },
  ];
};

const getMaskOnlyResults = (cidr: number) => {
  const maskLong = cidrToMaskLong(cidr);
  return [
    { label: "子网掩码", value: longToIp(maskLong) },
    { label: "反掩码", value: longToIp(~maskLong >>> 0) },
    { label: "CIDR", value: `/${cidr}` },
  ];
};

const evaluation = computed(() => {
  const ip = ipStr.value.trim();
  const mask = maskStr.value.trim();

  if (!mask) return { error: null, records: [] };

  const cidr = parseMaskOrWildcard(mask);
  if (cidr === -1) {
    return {
      error: "无效的掩码格式，请输入CIDR、子网掩码或反掩码",
      records: [],
    };
  }

  if (!ip) {
    return { error: null, records: getMaskOnlyResults(cidr) };
  }

  if (!isValidIp(ip)) {
    return { error: "无效的 IP 地址格式", records: [] };
  }

  return { error: null, records: calculateSubnetDetails(ip, cidr) };
});

// --- Subnetting Extension 计算逻辑 ---
const getRequiredHostBits = (hosts: number) => {
  let bits = 0;
  while (Math.pow(2, bits) < hosts + 2) {
    if (bits >= 32) break;
    bits++;
  }
  return bits;
};

const getRequiredSubnetBits = (subnets: number) => {
  let bits = 0;
  while (Math.pow(2, bits) < subnets) {
    if (bits >= 32) break;
    bits++;
  }
  return bits;
};

const subnettingEvaluation = computed(() => {
  const ip = ipStr.value.trim();
  const mask = maskStr.value.trim();
  const hStr = hostCountStr.value.trim();
  const sStr = subnetCountStr.value.trim();

  if (!ip || !mask || (!hStr && !sStr)) {
    return {
      error: null,
      warning: null,
      showForceButton: false,
      subnets: [],
      totalSubnets: 0,
    };
  }

  const cidr = parseMaskOrWildcard(mask);
  if (cidr === -1 || !isValidIp(ip)) {
    return {
      error: "请先完成上方基础网络信息的有效填写",
      warning: null,
      showForceButton: false,
      subnets: [],
      totalSubnets: 0,
    };
  }

  let targetCidr = cidr;

  if (hStr) {
    const hosts = parseInt(hStr, 10);
    if (isNaN(hosts) || hosts <= 0) {
      return {
        error: "请输入有效的主机数",
        warning: null,
        showForceButton: false,
        subnets: [],
        totalSubnets: 0,
      };
    }
    const hostBits = getRequiredHostBits(hosts);
    targetCidr = 32 - hostBits;
    if (targetCidr < cidr) {
      return {
        error: `当前掩码 /${cidr} 的网络无法提供 ${hosts} 个连续主机的空间`,
        warning: null,
        showForceButton: false,
        subnets: [],
        totalSubnets: 0,
      };
    }
  } else if (sStr) {
    const subnets = parseInt(sStr, 10);
    if (isNaN(subnets) || subnets <= 0) {
      return {
        error: "请输入有效的子网数",
        warning: null,
        showForceButton: false,
        subnets: [],
        totalSubnets: 0,
      };
    }
    const subnetBits = getRequiredSubnetBits(subnets);
    targetCidr = cidr + subnetBits;
    if (targetCidr > 32) {
      return {
        error: `当前掩码 /${cidr} 无法划分出 ${subnets} 个子网，位空间不足`,
        warning: null,
        showForceButton: false,
        subnets: [],
        totalSubnets: 0,
      };
    }
  }

  const baseIpLong = ipToLong(ip);
  const baseMaskLong = cidrToMaskLong(cidr);
  const baseNetworkLong = baseIpLong & baseMaskLong;

  const newMaskLong = cidrToMaskLong(targetCidr);

  // Maximum subnets to avoid performance issues
  const maxSubnetsToGenerate = 256;
  const absoluteMaxSubnets = 65535; // Hard limit even on force display
  const totalSubnets = Math.pow(2, targetCidr - cidr);

  const subnetsList = [];
  const step = Math.pow(2, 32 - targetCidr);

  let generateCount = totalSubnets;
  if (!forceDisplayAllSubnets.value) {
    generateCount = Math.min(totalSubnets, maxSubnetsToGenerate);
  } else {
    generateCount = Math.min(totalSubnets, absoluteMaxSubnets);
  }

  for (let i = 0; i < generateCount; i++) {
    const networkLong = baseNetworkLong + i * step;
    const broadcastLong = networkLong | (~newMaskLong >>> 0);
    const firstUsableLong = targetCidr < 31 ? networkLong + 1 : networkLong;
    const lastUsableLong = targetCidr < 31 ? broadcastLong - 1 : broadcastLong;
    const usableHosts =
      targetCidr < 31
        ? Math.pow(2, 32 - targetCidr) - 2
        : targetCidr === 31
          ? 2
          : 1;

    subnetsList.push({
      index: i + 1,
      network: longToIp(networkLong),
      cidr: targetCidr,
      firstUsable: usableHosts > 0 ? longToIp(firstUsableLong) : "N/A",
      lastUsable: usableHosts > 0 ? longToIp(lastUsableLong) : "N/A",
      broadcast: longToIp(broadcastLong),
      mask: longToIp(newMaskLong),
    });
  }

  let warning = null;
  let showForceButton = false;
  if (!forceDisplayAllSubnets.value && totalSubnets > maxSubnetsToGenerate) {
    warning = `由于数据量过大，总计划分出 ${totalSubnets.toLocaleString()} 个子网，此处仅展示前 ${maxSubnetsToGenerate} 个保护浏览器性能。`;
    showForceButton = true;
  } else if (
    forceDisplayAllSubnets.value &&
    totalSubnets > absoluteMaxSubnets
  ) {
    warning = `数据量极其庞大！已强制展示前 ${absoluteMaxSubnets.toLocaleString()} 个子网。为防止浏览器崩溃，剩余数据不再渲染。`;
    showForceButton = false;
  } else if (
    forceDisplayAllSubnets.value &&
    totalSubnets > maxSubnetsToGenerate
  ) {
    warning = `已强制展示全部 ${totalSubnets.toLocaleString()} 个子网，由于节点众多，页面若有轻微卡顿属于正常现象。`;
    showForceButton = false;
  }

  return {
    error: null,
    warning,
    showForceButton,
    subnets: subnetsList,
    totalSubnets,
  };
});

// 重置强制展示状态
watch([ipStr, maskStr, hostCountStr, subnetCountStr], () => {
  forceDisplayAllSubnets.value = false;
});

// IP 自动格式化简化版 (限制输入字符)
watch(ipStr, (newVal) => {
  if (!newVal) return;
  let sanitized = newVal.replace(/[^0-9.]/g, "");
  if (sanitized !== newVal) {
    ipStr.value = sanitized;
  }
});

watch(hostCountStr, (newVal) => {
  if (!newVal) return;
  let sanitized = newVal.replace(/[^0-9]/g, "");
  if (sanitized !== newVal) {
    hostCountStr.value = sanitized;
  }
});

watch(subnetCountStr, (newVal) => {
  if (!newVal) return;
  let sanitized = newVal.replace(/[^0-9]/g, "");
  if (sanitized !== newVal) {
    subnetCountStr.value = sanitized;
  }
});

const copyText = (val: string) => {
  if (val && val !== "N/A") {
    navigator.clipboard.writeText(val);
  }
};

// 导出 CSV 功能
const exportToCSV = () => {
  const { subnets } = subnettingEvaluation.value;
  if (!subnets || subnets.length === 0) return;

  // 1. 准备 CSV 表头
  const headers = [
    "序号",
    "网络号",
    "CIDR",
    "首个可用 IP",
    "最后可用 IP",
    "广播地址",
    "子网掩码",
  ];

  // 2. 准备 CSV 数据行
  const csvRows = [];
  csvRows.push(headers.join(",")); // 加入表头行

  subnets.forEach((item: any) => {
    const row = [
      item.index,
      item.network,
      item.cidr,
      item.firstUsable,
      item.lastUsable,
      item.broadcast,
      item.mask,
    ];
    // 使用双引号包裹每个字段，防止字段内部含有逗号破坏格式
    const csvRow = row.map((val) => `"${val}"`).join(",");
    csvRows.push(csvRow);
  });

  const csvString = csvRows.join("\n");

  // 3. 构建 Blob 和下载链接 (添加 \ufeff BOM 头防止 Excel 中文乱码)
  const blobData: any[] = ["\ufeff" + csvString];
  const blob = new Blob(blobData, { type: "text/csv;charset=utf-8;" });

  // 绕过严格 TS 类型检查
  const rawWindow: any = window;
  const url = rawWindow.URL.createObjectURL(blob);

  const link = document.createElement("a");
  link.setAttribute("href", url);
  // 动态文件名 (例如: 192.168.1.0_子网划分.csv)
  const baseNet = subnets[0]?.network || "未知网络";
  link.setAttribute(
    "download",
    `${baseNet.replace(/\./g, "_")}_子网划分明细.csv`,
  );
  link.style.visibility = "hidden";

  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  // 释放内存
  URL.revokeObjectURL(url);
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
.fade-enter-active,
.fade-leave-active {
  transition:
    opacity 0.3s ease,
    transform 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
