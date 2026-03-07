<script setup lang="ts">
import { ref, computed } from 'vue'

interface PortItem {
  port: string | number
  protocol: string
  name: string
  description: string
}

interface ProtocolItem {
  number: number
  hex: string
  name: string
  description: string
}

const currentTab = ref<'port' | 'protocol'>('port')
const searchQuery = ref('')

const portsData: PortItem[] = [
  { port: '20/21', protocol: 'TCP', name: 'FTP', description: '文件传输协议 (20 数据, 21 控制)' },
  { port: 22, protocol: 'TCP', name: 'SSH', description: '安全外壳协议，用于安全远程登录' },
  { port: 23, protocol: 'TCP', name: 'Telnet', description: '不安全的远程登录协议' },
  { port: 25, protocol: 'TCP', name: 'SMTP', description: '简单邮件传输协议 (发送邮件)' },
  { port: 53, protocol: 'TCP/UDP', name: 'DNS', description: '域名系统 (UDP 为主，TCP 用于区域传输)' },
  { port: '67/68', protocol: 'UDP', name: 'DHCP', description: '动态主机配置协议 (67 服务端, 68 客户端)' },
  { port: 69, protocol: 'UDP', name: 'TFTP', description: '简单文件传输协议' },
  { port: 80, protocol: 'TCP', name: 'HTTP', description: '超文本传输协议' },
  { port: 110, protocol: 'TCP', name: 'POP3', description: '邮局协议版本3 (接收邮件)' },
  { port: 123, protocol: 'UDP', name: 'NTP', description: '网络时间协议' },
  { port: 143, protocol: 'TCP', name: 'IMAP', description: '因特网信息访问协议 (接收邮件)' },
  { port: '161/162', protocol: 'UDP', name: 'SNMP', description: '简单网络管理协议 (161 代理, 162 Trap)' },
  { port: 179, protocol: 'TCP', name: 'BGP', description: '边界网关协议' },
  { port: 443, protocol: 'TCP', name: 'HTTPS', description: '安全的超文本传输协议' },
  { port: 500, protocol: 'UDP', name: 'ISAKMP', description: 'IPsec Internet 安全关联和密钥管理协议' },
  { port: 514, protocol: 'UDP', name: 'Syslog', description: '系统日志服务' },
  { port: 520, protocol: 'UDP', name: 'RIP', description: '路由信息协议' },
  { port: 1812, protocol: 'UDP', name: 'RADIUS', description: '远程认证拨号用户服务 (认证和授权)' },
  { port: 1813, protocol: 'UDP', name: 'RADIUS', description: '远程认证拨号用户服务 (计费)' },
  { port: 3389, protocol: 'TCP', name: 'RDP', description: '远程桌面协议 (Windows)' },
  { port: 8080, protocol: 'TCP', name: 'HTTP Proxy', description: '常见的备用 HTTP 端口或代理端口' }
]

const protocolsData: ProtocolItem[] = [
  { number: 1, hex: '01', name: 'ICMP', description: '互联网控制消息协议' },
  { number: 2, hex: '02', name: 'IGMP', description: '互联网组管理协议' },
  { number: 4, hex: '04', name: 'IPv4', description: 'IPv4 封装' },
  { number: 6, hex: '06', name: 'TCP', description: '传输控制协议' },
  { number: 17, hex: '11', name: 'UDP', description: '用户数据报协议' },
  { number: 41, hex: '29', name: 'IPv6', description: 'IPv6 封装' },
  { number: 47, hex: '2F', name: 'GRE', description: '通用路由封装' },
  { number: 50, hex: '32', name: 'ESP', description: 'IPsec 封装安全有效载荷' },
  { number: 51, hex: '33', name: 'AH', description: 'IPsec 认证头' },
  { number: 58, hex: '3A', name: 'IPv6-ICMP', description: 'ICMP for IPv6' },
  { number: 88, hex: '58', name: 'EIGRP', description: '增强型内部网关路由协议' },
  { number: 89, hex: '59', name: 'OSPF', description: '开放式最短路径优先' },
  { number: 103, hex: '67', name: 'PIM', description: '协议独立组播' },
  { number: 112, hex: '70', name: 'VRRP', description: '虚拟路由器冗余协议' },
  { number: 115, hex: '73', name: 'L2TP', description: '第二层隧道协议' }
]

const filteredPorts = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return portsData
  return portsData.filter(item => 
    item.name.toLowerCase().includes(q) ||
    String(item.port).toLowerCase().includes(q) ||
    item.description.toLowerCase().includes(q) ||
    item.protocol.toLowerCase().includes(q)
  )
})

const filteredProtocols = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return protocolsData
  return protocolsData.filter(item => 
    item.name.toLowerCase().includes(q) ||
    String(item.number).includes(q) ||
    item.hex.toLowerCase().includes(q) ||
    item.description.toLowerCase().includes(q)
  )
})

const switchTab = (tab: 'port' | 'protocol') => {
  currentTab.value = tab
  searchQuery.value = ''
}
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    <!-- 主要内容区 -->
    <div class="flex-1 flex flex-col min-h-0">
      
      <!-- 控制栏：搜索和 Tabs -->
      <div class="flex flex-col md:flex-row justify-between items-center gap-4 mb-6">
        <!-- Tabs -->
        <div class="flex p-1 space-x-1 bg-bg-tertiary rounded-xl w-full md:w-auto">
          <button
            @click="switchTab('port')"
            :class="[
              'w-full md:w-32 py-2 text-sm font-medium rounded-lg transition-all duration-200',
              currentTab === 'port' 
                ? 'bg-bg-secondary text-accent shadow'
                : 'text-text-muted hover:text-text-primary hover:bg-bg-hover'
            ]"
          >
            常见端口
          </button>
          <button
            @click="switchTab('protocol')"
            :class="[
              'w-full md:w-32 py-2 text-sm font-medium rounded-lg transition-all duration-200',
              currentTab === 'protocol' 
                ? 'bg-bg-secondary text-accent shadow'
                : 'text-text-muted hover:text-text-primary hover:bg-bg-hover'
            ]"
          >
            IP 协议号
          </button>
        </div>

        <!-- 全局搜索 -->
        <div class="relative w-full md:w-72">
          <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
            <svg class="h-5 w-5 text-text-muted" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd" />
            </svg>
          </div>
          <input
            v-model="searchQuery"
            type="text"
            class="block w-full pl-10 pr-3 py-2 border border-border rounded-xl leading-5 bg-bg-secondary/60 placeholder-text-muted focus:outline-none focus:ring-2 focus:ring-accent focus:border-accent sm:text-sm transition-colors duration-200 text-text-primary"
            placeholder="输入名称、端口或描述进行过滤..."
          />
        </div>
      </div>

      <!-- 常见端口表格视图 -->
      <div v-if="currentTab === 'port'" class="flex-1 overflow-auto rounded-xl border border-border bg-bg-secondary/30">
        <table class="min-w-full divide-y divide-border">
          <thead class="bg-bg-tertiary/80 sticky top-0">
            <tr>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider w-24">端口</th>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider w-32">协议</th>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider w-32">名称</th>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider">描述</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border/50">
            <template v-if="filteredPorts.length > 0">
              <tr 
                v-for="item in filteredPorts" 
                :key="item.name + item.port"
                class="hover:bg-bg-hover transition-colors"
              >
                <td class="px-6 py-4 whitespace-nowrap text-sm font-semibold text-text-primary">
                  {{ item.port }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-accent-bg text-accent border border-accent/30">
                    {{ item.protocol }}
                  </span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-info">
                  {{ item.name }}
                </td>
                <td class="px-6 py-4 text-sm text-text-secondary">
                  {{ item.description }}
                </td>
              </tr>
            </template>
            <tr v-else>
              <td colspan="4" class="px-6 py-12 text-center text-sm text-text-muted">
                未找到匹配的端口信息
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- IP协议号表格视图 -->
      <div v-if="currentTab === 'protocol'" class="flex-1 overflow-auto rounded-xl border border-border bg-bg-secondary/30">
        <table class="min-w-full divide-y divide-border">
          <thead class="bg-bg-tertiary/80 sticky top-0">
            <tr>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider w-24">协议号</th>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider w-24">十六进制</th>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider w-32">名称</th>
              <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-text-muted uppercase tracking-wider">描述</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border/50">
            <template v-if="filteredProtocols.length > 0">
              <tr 
                v-for="item in filteredProtocols" 
                :key="item.name + item.number"
                class="hover:bg-bg-hover transition-colors"
              >
                <td class="px-6 py-4 whitespace-nowrap text-sm font-semibold text-text-primary">
                  {{ item.number }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-text-muted font-mono">
                  0x{{ item.hex }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-success">
                  {{ item.name }}
                </td>
                <td class="px-6 py-4 text-sm text-text-secondary">
                  {{ item.description }}
                </td>
              </tr>
            </template>
            <tr v-else>
              <td colspan="4" class="px-6 py-12 text-center text-sm text-text-muted">
                未找到匹配的协议信息
              </td>
            </tr>
          </tbody>
        </table>
      </div>

    </div>
  </div>
</template>
