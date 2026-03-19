import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import Devices from '../views/Devices.vue'
import Commands from '../views/Commands.vue'
import Tasks from '../views/Tasks.vue'
import TaskExecution from '../views/TaskExecution.vue'
import NetworkCalc from '../views/Tools/NetworkCalc.vue'
import ProtocolRef from '../views/Tools/ProtocolRef.vue'
import ConfigForge from '../views/Tools/ConfigForge.vue'
import Settings from '../views/Settings.vue'
import Discovery from '../views/Discovery.vue'
import Topology from '../views/Topology.vue'
import PlanCompare from '../views/PlanCompare.vue'

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
    name: 'Discovery',
    component: Discovery
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
