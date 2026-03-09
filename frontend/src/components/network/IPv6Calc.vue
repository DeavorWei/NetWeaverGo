<script setup lang="ts">
import { ref, computed } from "vue";

const ipv6Str = ref("");
const prefixStr = ref("");
const v6CheckIpStr = ref(""); // 新增包含关系检查 IP

// —— IPv6 Utilities ——

// 验证 IPv6 地址格式 (支持压缩格式)
const isValidIPv6 = (ip: string) => {
  const ipv6Regex =
    /^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$/;
  return ipv6Regex.test(ip.trim());
};

// 展开 IPv6 地址 (将 :: 替换为确切的 0000 块，确保 8 个块)
const expandIPv6 = (ip: string) => {
  if (!isValidIPv6(ip)) return null;
  let full = ip.trim().toLowerCase();

  if (full === "::") return "0000:0000:0000:0000:0000:0000:0000:0000";

  let parts = full.split("::");
  if (parts.length > 2) return null; // 错误格式：多个 ::

  if (parts.length === 2) {
    let left = parts[0] ? parts[0].split(":") : [];
    let right = parts[1] ? parts[1].split(":") : [];
    let missing = 8 - (left.length + right.length);
    let middle = new Array(missing).fill("0000");
    full = [...left, ...middle, ...right].join(":");
  }

  // 补齐每个块的 4 位 16 进制
  return full
    .split(":")
    .map((block) => block.padStart(4, "0"))
    .join(":");
};

// 压缩 IPv6 地址 (将最长的连续 0000 块替换为 ::)
const compressIPv6 = (ip: string) => {
  const expanded = expandIPv6(ip);
  if (!expanded) return ip;

  const blocks = expanded.split(":").map((b) => parseInt(b, 16).toString(16)); // 去除前导0
  let maxZeroStart = -1;
  let maxZeroLen = 0;
  let currentZeroStart = -1;
  let currentZeroLen = 0;

  for (let i = 0; i < blocks.length; i++) {
    if (blocks[i] === "0") {
      if (currentZeroStart === -1) currentZeroStart = i;
      currentZeroLen++;
      if (currentZeroLen > maxZeroLen) {
        maxZeroLen = currentZeroLen;
        maxZeroStart = currentZeroStart;
      }
    } else {
      currentZeroStart = -1;
      currentZeroLen = 0;
    }
  }

  if (maxZeroLen > 1) {
    // 只有连续两个及以上的 0 才压缩
    blocks.splice(maxZeroStart, maxZeroLen, "");
    let result = blocks.join(":");
    if (result.startsWith(":")) result = ":" + result;
    if (result.endsWith(":")) result = result + ":";
    return result;
  }

  return blocks.join(":");
};

// 将 IPv6 转换为 128 位 BigInt
const ipv6ToBigInt = (ip: string) => {
  const expanded = expandIPv6(ip);
  if (!expanded) return 0n;
  const hex = expanded.replace(/:/g, "");
  return BigInt("0x" + hex);
};

// 将 BigInt 转换为展开的 IPv6 字符串
const bigIntToIPv6 = (num: bigint) => {
  let hex = num.toString(16).padStart(32, "0");
  let blocks = [];
  for (let i = 0; i < 32; i += 4) {
    blocks.push(hex.substring(i, i + 4));
  }
  return blocks.join(":");
};

// 获取前缀长度掩码 (BigInt)
const cidrToMaskBigInt = (prefix: number) => {
  if (prefix === 0) return 0n;
  if (prefix === 128) return (1n << 128n) - 1n; // 全 1
  return ((1n << BigInt(prefix)) - 1n) << BigInt(128 - prefix);
};

// 根据前缀计算可用地址范围等
const calculateIPv6Subnet = (ip: string, prefix: number) => {
  const ipBigInt = ipv6ToBigInt(ip);
  const maskBigInt = cidrToMaskBigInt(prefix);

  const networkBigInt = ipBigInt & maskBigInt;
  const broadcastBigInt = networkBigInt | (~maskBigInt & ((1n << 128n) - 1n)); // 128位取反

  const networkStr = compressIPv6(bigIntToIPv6(networkBigInt));
  const broadcastStr = compressIPv6(bigIntToIPv6(broadcastBigInt));

  // 计算可用主机数 (2 ^ (128 - prefix))
  // 注意：BigInt 运算，数量巨大，通常使用科学计数法或 2^n 表示
  const hostBits = 128 - prefix;
  let hostsStr = "";
  if (hostBits === 0) {
    hostsStr = "1 (表示单一地址)";
  } else if (hostBits < 53) {
    // 小于 Number.MAX_SAFE_INTEGER
    hostsStr = Math.pow(2, hostBits).toLocaleString();
  } else {
    hostsStr = `2^${hostBits} (极其庞大)`;
  }

  return [
    { label: "完整地址展示", value: expandIPv6(ip) || "" },
    { label: "压缩地址展示", value: compressIPv6(ip) || "" },
    { label: "网络前缀 / CIDR", value: `/${prefix}` },
    { label: "网络地址", value: networkStr },
    { label: "类型", value: getIPv6Type(expandIPv6(ip)) },
    {
      label: "可用 IP 范围",
      value: hostBits === 0 ? networkStr : `${networkStr} - ${broadcastStr}`,
    },
    { label: "地址总数", value: hostsStr },
  ];
};

const getIPv6Type = (expandedIp: string | null | undefined) => {
  if (!expandedIp) return "未知";
  const parts = expandedIp.split(":");
  const firstBlockStr = parts[0];
  if (!firstBlockStr) return "未知";
  const firstBlock = parseInt(firstBlockStr, 16);

  if (expandedIp === "0000:0000:0000:0000:0000:0000:0000:0000")
    return "未指定地址 (Unspecified)";
  if (expandedIp === "0000:0000:0000:0000:0000:0000:0000:0001")
    return "环回地址 (Loopback)";

  if ((firstBlock & 0xff00) === 0xff00) return "组播地址 (Multicast)";
  if ((firstBlock & 0xfe80) === 0xfe80) return "链路本地单播 (Link-Local)";
  if ((firstBlock & 0xfec0) === 0xfec0)
    return "站点本地单播 (Site-Local) - 已废弃";
  if ((firstBlock & 0xfc00) === 0xfc00 || (firstBlock & 0xfd00) === 0xfd00)
    return "唯一本地地址 (ULA)";
  if ((firstBlock & 0xe000) === 0x2000) return "全球单播地址 (Global Unicast)";

  return "未知/保留地址";
};

const evaluation = computed(() => {
  const ip = ipv6Str.value.trim();
  const prefix = prefixStr.value.trim();

  if (!ip) return { error: null, records: [] };
  if (!isValidIPv6(ip)) return { error: "无效的 IPv6 地址格式", records: [] };

  if (!prefix) {
    return {
      error: null,
      records: [
        { label: "完整地址展示", value: expandIPv6(ip) || "" },
        { label: "压缩地址展示", value: compressIPv6(ip) || "" },
        { label: "类型", value: getIPv6Type(expandIPv6(ip)) },
      ],
    };
  }

  const prefixNum = parseInt(
    prefix.startsWith("/") ? prefix.substring(1) : prefix,
    10,
  );

  if (isNaN(prefixNum) || prefixNum < 0 || prefixNum > 128) {
    return { error: "前缀长度必须在 0 到 128 之间", records: [] };
  }

  return {
    error: null,
    records: calculateIPv6Subnet(ip, prefixNum).map((r) => ({
      ...r,
      value: r.value || "",
    })),
  };
});

// --- IPv6 包含关系检查逻辑 ---
const inclusionCheck = computed(() => {
  const ip = ipv6Str.value.trim();
  const prefix = prefixStr.value.trim();
  const checkIp = v6CheckIpStr.value.trim();

  if (!ip || !prefix || !checkIp) return null;
  if (!isValidIPv6(ip) || !isValidIPv6(checkIp)) return null;

  const prefixNum = parseInt(
    prefix.startsWith("/") ? prefix.substring(1) : prefix,
    10,
  );
  if (isNaN(prefixNum) || prefixNum < 0 || prefixNum > 128) return null;

  const baseIpBigInt = ipv6ToBigInt(ip);
  const checkIpBigInt = ipv6ToBigInt(checkIp);
  const maskBigInt = cidrToMaskBigInt(prefixNum);

  const baseNetwork = baseIpBigInt & maskBigInt;
  const checkNetwork = checkIpBigInt & maskBigInt;

  const isIncluded = baseNetwork === checkNetwork;

  return {
    isIncluded,
    message: isIncluded
      ? `地址 ${checkIp} 包含在当前 ${compressIPv6(bigIntToIPv6(baseNetwork))}/${prefixNum} 网段内`
      : `地址 ${checkIp} 不在此网段内`,
  };
});

const copyText = (val: string) => {
  if (val && val !== "N/A") {
    navigator.clipboard.writeText(val);
  }
};

// --- v6 Subnetting ---
const v6NewPrefixStr = ref("");

const v6SubnetEvaluation = computed(() => {
  const ip = ipv6Str.value.trim();
  const prefix = prefixStr.value.trim();
  const newPrefixRaw = v6NewPrefixStr.value.trim();

  if (!ip || !prefix || !newPrefixRaw) {
    return { error: null, subnets: [], total: 0 };
  }

  if (!isValidIPv6(ip))
    return { error: "请先完成上方有效 IPv6 填写", subnets: [], total: 0 };

  const prefixNum = parseInt(
    prefix.startsWith("/") ? prefix.substring(1) : prefix,
    10,
  );
  if (isNaN(prefixNum) || prefixNum < 0 || prefixNum > 128)
    return { error: "请填写正确的原前缀长度", subnets: [], total: 0 };

  const newPrefixNum = parseInt(
    newPrefixRaw.startsWith("/") ? newPrefixRaw.substring(1) : newPrefixRaw,
    10,
  );
  if (isNaN(newPrefixNum) || newPrefixNum <= prefixNum || newPrefixNum > 128) {
    return {
      error: `新前缀必须大于原前缀 (${prefixNum}) 且不超过 128`,
      subnets: [],
      total: 0,
    };
  }

  const diff = newPrefixNum - prefixNum;
  if (diff > 16) {
    return {
      error: `单次最多仅支持下拨 16 位 (即 65536 个子网)，当前尝试下拨 ${diff} 位，数据过大导致浏览器越界。`,
      subnets: [],
      total: 0,
    };
  }

  const totalSubnets = Math.pow(2, diff);
  const maxSubnets = 256;

  const subnets = [];
  const baseIpBigInt = ipv6ToBigInt(ip);
  const baseMaskBigInt = cidrToMaskBigInt(prefixNum);
  const networkBigInt = baseIpBigInt & baseMaskBigInt;

  const step = 1n << BigInt(128 - newPrefixNum); // 每次增加的步长

  const limit = Math.min(totalSubnets, maxSubnets);
  const checkIp = v6CheckIpStr.value.trim();
  const isCheckValid = isValidIPv6(checkIp);
  let checkIpBigInt = 0n;
  let targetSubnetMask = 0n;
  if (isCheckValid) {
    checkIpBigInt = ipv6ToBigInt(checkIp);
    targetSubnetMask = cidrToMaskBigInt(newPrefixNum);
  }

  for (let i = 0n; i < BigInt(limit); i++) {
    const subNetInt = networkBigInt + i * step;
    const subNetStr = compressIPv6(bigIntToIPv6(subNetInt));

    // 检查用户输入的待检IP是否落在当前这个拆分的小子网内
    let isIncluded = false;
    if (isCheckValid) {
      const currentSubNetwork = subNetInt & targetSubnetMask;
      const checkIpNetwork = checkIpBigInt & targetSubnetMask;
      if (
        currentSubNetwork === checkIpNetwork &&
        inclusionCheck.value?.isIncluded
      ) {
        isIncluded = true;
      }
    }

    subnets.push({
      index: Number(i) + 1,
      network: subNetStr,
      cidr: newPrefixNum,
      isIncluded,
    });
  }

  let warning = null;
  if (totalSubnets > maxSubnets) {
    warning = `总计划分出 ${totalSubnets.toLocaleString()} 个子网，为保护页面性能，此处仅展示前 ${maxSubnets} 个。`;
  }

  return { error: null, warning, subnets, total: totalSubnets };
});
</script>

<template>
  <div class="space-y-4">
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
                      class="w-1.5 h-1.5 rounded-full bg-success mr-2 shadow-[0_0_8px_rgba(34,197,94,0.8)]"
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
