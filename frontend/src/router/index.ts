import { createRouter, createWebHashHistory } from 'vue-router'

// 首屏页面 - 保持静态导入以确保首页快速加载
import Dashboard from '../views/Dashboard.vue'

// 其他页面 - 懒加载，降低首屏资源体积
const Devices = () => import('../views/Devices.vue')
const Commands = () => import('../views/Commands.vue')
const Tasks = () => import('../views/Tasks.vue')
const TaskExecution = () => import('../views/TaskExecution.vue')
const NetworkCalc = () => import('../views/Tools/NetworkCalc.vue')
const ProtocolRef = () => import('../views/Tools/ProtocolRef.vue')
const ConfigForge = () => import('../views/Tools/ConfigForge.vue')
const Settings = () => import('../views/Settings.vue')
const Topology = () => import('../views/Topology.vue')
const PlanCompare = () => import('../views/PlanCompare.vue')

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard
  },
  {
    path: '/devices',
    name: 'Devices',
    component: Devices
  },
  {
    path: '/commands',
    name: 'Commands',
    component: Commands
  },
  {
    path: '/tasks',
    name: 'Tasks',
    component: Tasks
  },
  {
    path: '/task-execution',
    name: 'TaskExecution',
    component: TaskExecution
  },
  {
    path: '/tools/calc',
    name: 'NetworkCalc',
    component: NetworkCalc
  },
  {
    path: '/tools/protocol',
    name: 'ProtocolRef',
    component: ProtocolRef
  },
  {
    path: '/tools/config',
    name: 'ConfigForge',
    component: ConfigForge
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings
  },
  {
    path: '/discovery',
    redirect: '/tasks'
  },
  {
    path: '/topology',
    name: 'Topology',
    component: Topology
  },
  {
    path: '/plan-compare',
    name: 'PlanCompare',
    component: PlanCompare
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
