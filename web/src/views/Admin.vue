<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/api'
import { Settings, Users, FileText, ChevronLeft, ChevronRight, RefreshCw } from 'lucide-vue-next'

const activeTab = ref('settings')

// Settings
const openRegistration = ref(false)
const settingsMsg = ref({ text: '', type: '' })
const settingsLoading = ref(false)
const globalSpeedLimit = ref(0) // MB/s stored as float, 0 = unlimited

// Users
const users = ref<any[]>([])
const usersPage = ref(1)
const usersTotal = ref(0)
const editingUserId = ref<number | null>(null)
const editingSpeedMbps = ref('')

// Logs
const logs = ref<any[]>([])
const logsPage = ref(1)
const logsTotal = ref(0)

// Update
const updateInfo = ref<any>(null)
const updateChecking = ref(false)
const updateApplying = ref(false)
const updateMsg = ref({ text: '', type: '' })
const restarting = ref(false)
let restartPollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  fetchSettings()
})

async function fetchSettings() {
  try {
    const res = await api.get('/admin/settings')
    openRegistration.value = res.data.open_registration
    // Convert bytes/sec to MB/s
    const bps = res.data.global_speed_limit || 0
    globalSpeedLimit.value = bps > 0 ? Math.round(bps / (1024 * 1024) * 10) / 10 : 0
  } catch {
    // ignore
  }
}

async function updateSettings() {
  settingsMsg.value = { text: '', type: '' }
  settingsLoading.value = true
  try {
    const bps = globalSpeedLimit.value > 0
      ? Math.round(globalSpeedLimit.value * 1024 * 1024)
      : 0
    await api.put('/admin/settings', {
      open_registration: openRegistration.value,
      global_speed_limit: bps,
    })
    settingsMsg.value = { text: '设置已保存', type: 'success' }
  } catch (err: any) {
    settingsMsg.value = { text: err.response?.data?.error || '保存失败', type: 'error' }
  } finally {
    settingsLoading.value = false
  }
}

async function fetchUsers(page = 1) {
  try {
    const res = await api.get('/admin/users', { params: { page, page_size: 20 } })
    users.value = res.data.data || []
    usersTotal.value = res.data.total
    usersPage.value = page
  } catch {
    // ignore
  }
}

async function fetchLogs(page = 1) {
  try {
    const res = await api.get('/admin/logs', { params: { page, page_size: 20 } })
    logs.value = res.data.data || []
    logsTotal.value = res.data.total
    logsPage.value = page
  } catch {
    // ignore
  }
}

function formatSpeed(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec <= 0) return '不限制'
  if (bytesPerSec >= 1024 * 1024) {
    return (bytesPerSec / (1024 * 1024)).toFixed(1) + ' MB/s'
  }
  return Math.round(bytesPerSec / 1024) + ' KB/s'
}

function startEditSpeed(user: any) {
  editingUserId.value = user.id
  const mbps = user.speed_limit > 0 ? Math.round(user.speed_limit / (1024 * 1024) * 10) / 10 : 0
  editingSpeedMbps.value = mbps > 0 ? String(mbps) : ''
}

function cancelEditSpeed() {
  editingUserId.value = null
  editingSpeedMbps.value = ''
}

async function saveUserSpeed(user: any) {
  const mbps = parseFloat(editingSpeedMbps.value) || 0
  const bps = mbps > 0 ? Math.round(mbps * 1024 * 1024) : 0
  try {
    await api.put(`/admin/users/${user.id}/speed-limit`, { speed_limit: bps })
    user.speed_limit = bps
    editingUserId.value = null
  } catch (err: any) {
    alert(err.response?.data?.error || '更新失败')
  }
}

function formatDate(dateStr: string) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

// --- Update ---

async function checkUpdate() {
  updateMsg.value = { text: '', type: '' }
  updateChecking.value = true
  try {
    const res = await api.get('/admin/update/check')
    updateInfo.value = res.data
    if (!res.data.has_update) {
      updateMsg.value = { text: '当前已是最新版本', type: 'success' }
    }
  } catch (err: any) {
    updateMsg.value = { text: err.response?.data?.error || '检查更新失败', type: 'error' }
  } finally {
    updateChecking.value = false
  }
}

async function applyUpdate() {
  if (!updateInfo.value?.download_url) {
    updateMsg.value = { text: '未找到适合当前系统的安装包，请手动更新', type: 'error' }
    return
  }
  updateMsg.value = { text: '', type: '' }
  updateApplying.value = true
  try {
    await api.post('/admin/update/apply')
    restarting.value = true
    updateMsg.value = { text: '新版本下载完成，服务正在重启，请稍候...', type: 'success' }
    startRestartPolling()
  } catch (err: any) {
    updateMsg.value = { text: err.response?.data?.error || '更新失败', type: 'error' }
    updateApplying.value = false
  }
}

function startRestartPolling() {
  if (restartPollTimer) clearInterval(restartPollTimer)
  // Give the server a moment to start restarting before polling.
  setTimeout(() => {
    restartPollTimer = setInterval(async () => {
      try {
        await api.get('/system/init-status', { timeout: 3000 })
        // Server is back up.
        if (restartPollTimer) clearInterval(restartPollTimer)
        restarting.value = false
        updateApplying.value = false
        updateInfo.value = null
        updateMsg.value = { text: '更新完成，服务已重启', type: 'success' }
        // Reload the page so the new frontend assets are loaded.
        setTimeout(() => window.location.reload(), 1500)
      } catch {
        // Server still restarting, keep polling.
      }
    }, 2000)
  }, 3000)
}
</script>

<template>
  <div class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <h1 class="text-2xl font-bold mb-6">管理后台</h1>

    <!-- Tabs -->
    <div class="flex gap-1 mb-6 border-b border-[hsl(var(--border))]">
      <button @click="activeTab = 'settings'" class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-px" :class="activeTab === 'settings' ? 'border-[hsl(var(--primary))] text-[hsl(var(--foreground))]' : 'border-transparent text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]'">
        <Settings class="w-4 h-4 inline mr-1.5" />系统设置
      </button>
      <button @click="activeTab = 'users'; fetchUsers()" class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-px" :class="activeTab === 'users' ? 'border-[hsl(var(--primary))] text-[hsl(var(--foreground))]' : 'border-transparent text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]'">
        <Users class="w-4 h-4 inline mr-1.5" />用户管理
      </button>
      <button @click="activeTab = 'logs'; fetchLogs()" class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-px" :class="activeTab === 'logs' ? 'border-[hsl(var(--primary))] text-[hsl(var(--foreground))]' : 'border-transparent text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]'">
        <FileText class="w-4 h-4 inline mr-1.5" />下载记录
      </button>
      <button @click="activeTab = 'update'; checkUpdate()" class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-px" :class="activeTab === 'update' ? 'border-[hsl(var(--primary))] text-[hsl(var(--foreground))]' : 'border-transparent text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]'">
        <RefreshCw class="w-4 h-4 inline mr-1.5" />版本更新
      </button>
    </div>

    <!-- Settings -->
    <div v-if="activeTab === 'settings'" class="space-y-6">
      <div class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm p-6">
        <h3 class="text-base font-semibold mb-4">注册设置</h3>
        <div v-if="settingsMsg.text" class="mb-4 p-3 rounded-md text-sm" :class="settingsMsg.type === 'error' ? 'bg-red-50 text-red-600' : 'bg-green-50 text-green-600'">{{ settingsMsg.text }}</div>

        <div class="flex items-center justify-between max-w-md">
          <div>
            <div class="text-sm font-medium">开放注册</div>
            <div class="text-xs text-[hsl(var(--muted-foreground))]">允许新用户自行注册账号</div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="openRegistration" class="sr-only peer">
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-[hsl(var(--ring))] rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[hsl(var(--primary))]"></div>
          </label>
        </div>

        <div class="flex items-center justify-between max-w-md mt-4">
          <div>
            <div class="text-sm font-medium">全局带宽限制</div>
            <div class="text-xs text-[hsl(var(--muted-foreground))]">整体下载带宽上限（MB/s），0 或留空表示不限制</div>
          </div>
          <div class="flex items-center gap-2">
            <input
              v-model="globalSpeedLimit"
              type="number"
              min="0"
              step="0.1"
              class="w-24 px-3 py-1.5 rounded-md border border-[hsl(var(--input))] bg-transparent text-sm text-right focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:ring-offset-1"
              placeholder="0"
            />
            <span class="text-sm text-[hsl(var(--muted-foreground))]">MB/s</span>
          </div>
        </div>

        <button @click="updateSettings" :disabled="settingsLoading" class="mt-4 px-4 py-2 rounded-md bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] text-sm font-medium hover:opacity-90 disabled:opacity-50">
          {{ settingsLoading ? '保存中...' : '保存设置' }}
        </button>
      </div>
    </div>

    <!-- Users -->
    <div v-if="activeTab === 'users'">
      <div class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-[hsl(var(--secondary))]">
            <tr>
              <th class="text-left px-4 py-3 font-medium">ID</th>
              <th class="text-left px-4 py-3 font-medium">用户名</th>
              <th class="text-left px-4 py-3 font-medium">角色</th>
              <th class="text-left px-4 py-3 font-medium">TOTP</th>
              <th class="text-left px-4 py-3 font-medium">限速</th>
              <th class="text-left px-4 py-3 font-medium">注册时间</th>
              <th class="text-left px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[hsl(var(--border))]">
            <tr v-for="user in users" :key="user.id" class="hover:bg-[hsl(var(--secondary)_/_0.5)]">
              <td class="px-4 py-3">{{ user.id }}</td>
              <td class="px-4 py-3 font-medium">{{ user.username }}</td>
              <td class="px-4 py-3">
                <span v-if="user.is_admin" class="text-xs px-2 py-0.5 rounded-full bg-purple-100 text-purple-700">管理员</span>
                <span v-else class="text-xs px-2 py-0.5 rounded-full bg-gray-100 text-gray-600">普通用户</span>
              </td>
              <td class="px-4 py-3">
                <span v-if="user.totp_enabled" class="text-xs text-green-600">✓</span>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>
              <td class="px-4 py-3">
                <div v-if="editingUserId === user.id" class="flex items-center gap-1">
                  <input
                    v-model="editingSpeedMbps"
                    type="number"
                    min="0"
                    step="0.1"
                    class="w-20 px-2 py-1 text-xs rounded border border-[hsl(var(--input))] bg-transparent focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))]"
                    placeholder="MB/s"
                    @keyup.enter="saveUserSpeed(user)"
                    @keyup.esc="cancelEditSpeed()"
                  />
                  <span class="text-xs text-[hsl(var(--muted-foreground))]">MB/s</span>
                </div>
                <span v-else class="text-xs" :class="user.speed_limit > 0 ? 'text-amber-600' : 'text-[hsl(var(--muted-foreground))]'">
                  {{ formatSpeed(user.speed_limit) }}
                </span>
              </td>
              <td class="px-4 py-3 text-[hsl(var(--muted-foreground))]">{{ formatDate(user.created_at) }}</td>
              <td class="px-4 py-3">
                <div v-if="editingUserId === user.id" class="flex gap-1">
                  <button @click="saveUserSpeed(user)" class="px-2 py-1 text-xs rounded bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] hover:opacity-90">保存</button>
                  <button @click="cancelEditSpeed()" class="px-2 py-1 text-xs rounded border border-[hsl(var(--border))] hover:bg-[hsl(var(--secondary))]">取消</button>
                </div>
                <button v-else @click="startEditSpeed(user)" class="px-2 py-1 text-xs rounded border border-[hsl(var(--border))] hover:bg-[hsl(var(--secondary))]">限速</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="usersTotal > 20" class="flex items-center justify-between px-4 py-3 border-t border-[hsl(var(--border))]">
          <span class="text-xs text-[hsl(var(--muted-foreground))]">共 {{ usersTotal }} 条</span>
          <div class="flex items-center gap-1">
            <button @click="fetchUsers(usersPage - 1)" :disabled="usersPage <= 1" class="p-1.5 rounded hover:bg-[hsl(var(--secondary))] disabled:opacity-30">
              <ChevronLeft class="w-4 h-4" />
            </button>
            <span class="text-xs px-2">{{ usersPage }}</span>
            <button @click="fetchUsers(usersPage + 1)" :disabled="usersPage * 20 >= usersTotal" class="p-1.5 rounded hover:bg-[hsl(var(--secondary))] disabled:opacity-30">
              <ChevronRight class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Download Logs -->
    <div v-if="activeTab === 'logs'">
      <div class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-[hsl(var(--secondary))]">
            <tr>
              <th class="text-left px-4 py-3 font-medium">时间</th>
              <th class="text-left px-4 py-3 font-medium">用户</th>
              <th class="text-left px-4 py-3 font-medium">Token</th>
              <th class="text-left px-4 py-3 font-medium">下载链接</th>
              <th class="text-left px-4 py-3 font-medium">IP</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[hsl(var(--border))]">
            <tr v-for="log in logs" :key="log.id" class="hover:bg-[hsl(var(--secondary)_/_0.5)]">
              <td class="px-4 py-3 whitespace-nowrap text-[hsl(var(--muted-foreground))]">{{ log.created_at }}</td>
              <td class="px-4 py-3 font-medium">{{ log.username }}</td>
              <td class="px-4 py-3">
                <span class="text-xs font-mono bg-[hsl(var(--secondary))] px-1.5 py-0.5 rounded">{{ log.token_name }}</span>
              </td>
              <td class="px-4 py-3 max-w-xs truncate font-mono text-xs">{{ log.url }}</td>
              <td class="px-4 py-3 text-[hsl(var(--muted-foreground))]">{{ log.ip }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="logs.length === 0" class="p-8 text-center text-sm text-[hsl(var(--muted-foreground))]">暂无下载记录</div>
        <div v-if="logsTotal > 20" class="flex items-center justify-between px-4 py-3 border-t border-[hsl(var(--border))]">
          <span class="text-xs text-[hsl(var(--muted-foreground))]">共 {{ logsTotal }} 条</span>
          <div class="flex items-center gap-1">
            <button @click="fetchLogs(logsPage - 1)" :disabled="logsPage <= 1" class="p-1.5 rounded hover:bg-[hsl(var(--secondary))] disabled:opacity-30">
              <ChevronLeft class="w-4 h-4" />
            </button>
            <span class="text-xs px-2">{{ logsPage }}</span>
            <button @click="fetchLogs(logsPage + 1)" :disabled="logsPage * 20 >= logsTotal" class="p-1.5 rounded hover:bg-[hsl(var(--secondary))] disabled:opacity-30">
              <ChevronRight class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Version Update -->
    <div v-if="activeTab === 'update'" class="space-y-6">
      <div class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm p-6">
        <h3 class="text-base font-semibold mb-4">版本更新</h3>

        <!-- Restarting overlay -->
        <div v-if="restarting" class="mb-4 p-4 rounded-md bg-blue-50 text-blue-700 flex items-center gap-3">
          <RefreshCw class="w-5 h-5 animate-spin flex-shrink-0" />
          <span class="text-sm">服务正在重启，请稍候...</span>
        </div>

        <div v-if="updateMsg.text && !restarting" class="mb-4 p-3 rounded-md text-sm" :class="updateMsg.type === 'error' ? 'bg-red-50 text-red-600' : 'bg-green-50 text-green-600'">
          {{ updateMsg.text }}
        </div>

        <!-- Version info -->
        <div v-if="updateInfo" class="mb-4 space-y-2">
          <div class="flex items-center gap-2 text-sm">
            <span class="text-[hsl(var(--muted-foreground))]">当前版本：</span>
            <span class="font-mono font-medium">{{ updateInfo.current_version }}</span>
          </div>
          <div class="flex items-center gap-2 text-sm">
            <span class="text-[hsl(var(--muted-foreground))]">最新版本：</span>
            <span class="font-mono font-medium">{{ updateInfo.latest_version }}</span>
            <span v-if="updateInfo.has_update" class="text-xs px-2 py-0.5 rounded-full bg-amber-100 text-amber-700">有新版本</span>
            <span v-else class="text-xs px-2 py-0.5 rounded-full bg-green-100 text-green-700">已是最新</span>
          </div>
          <div v-if="updateInfo.has_update && !updateInfo.download_url" class="text-xs text-amber-600 mt-1">
            未找到适合当前系统 ({{ updateInfo.os }}) 的安装包，请前往 GitHub 手动下载更新。
          </div>
        </div>

        <div class="flex gap-2 flex-wrap">
          <button
            @click="checkUpdate"
            :disabled="updateChecking || updateApplying"
            class="px-4 py-2 rounded-md border border-[hsl(var(--border))] text-sm font-medium hover:bg-[hsl(var(--secondary))] disabled:opacity-50 flex items-center gap-1.5"
          >
            <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': updateChecking }" />
            {{ updateChecking ? '检查中...' : '检查更新' }}
          </button>

          <button
            v-if="updateInfo?.has_update && updateInfo?.download_url"
            @click="applyUpdate"
            :disabled="updateApplying"
            class="px-4 py-2 rounded-md bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] text-sm font-medium hover:opacity-90 disabled:opacity-50 flex items-center gap-1.5"
          >
            <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': updateApplying }" />
            {{ updateApplying ? '更新中...' : '立即更新到 ' + updateInfo.latest_version }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
