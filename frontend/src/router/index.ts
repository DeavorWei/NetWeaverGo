import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import Devices from '../views/Devices.vue'
import Commands from '../views/Commands.vue'
import Tasks from '../views/Tasks.vue'
import NetworkCalc from '../views/Tools/NetworkCalc.vue'
import ProtocolRef from '../views/Tools/ProtocolRef.vue'
import ConfigForge from '../views/Tools/ConfigForge.vue'
import Settings from '../views/Settings.vue'

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
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
