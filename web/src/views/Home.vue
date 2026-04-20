<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Copy, Check, Github, Download, Terminal, Link, ChevronDown } from 'lucide-vue-next'
import api from '@/api'

interface TokenItem {
  id: number
  token: string
  name: string
  expires_at: string | null
}

const inputUrl = ref('')
const copied = ref('')
const tokens = ref<TokenItem[]>([])
const selectedTokenIndex = ref(0)

const domain = computed(() => {
  return window.location.origin
})

const currentToken = computed(() => {
  if (tokens.value.length === 0) return ''
  return tokens.value[selectedTokenIndex.value]?.token || ''
})

const hasMultipleTokens = computed(() => tokens.value.length > 1)

onMounted(async () => {
  try {
    const res = await api.get('/tokens')
    tokens.value = res.data || []
  } catch {
    // ignore
  }
})

const isValidGithubUrl = computed(() => {
  if (!inputUrl.value) return false
  const patterns = [
    /^(?:https?:\/\/)?github\.com\/.+?\/.+?\/(?:releases|archive)\/.*/i,
    /^(?:https?:\/\/)?github\.com\/.+?\/.+?\/(?:blob|raw)\/.*/i,
    /^(?:https?:\/\/)?github\.com\/.+?\/.+?\/(?:info|git-).*/i,
    /^(?:https?:\/\/)?raw\.(?:githubusercontent|github)\.com\/.+?\/.+?\/.+?\/.+/i,
    /^(?:https?:\/\/)?gist\.(?:githubusercontent|github)\.com\/.+?\/.+?\/.+/i,
    /^(?:https?:\/\/)?github\.com\/.+?\/.+?\/tags.*/i,
  ]
  return patterns.some(p => p.test(inputUrl.value))
})

const normalizedUrl = computed(() => {
  if (!inputUrl.value) return ''
  let url = inputUrl.value.trim()
  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    url = 'https://' + url
  }
  return url
})

const proxyUrl = computed(() => {
  if (!isValidGithubUrl.value) return ''
  return `${domain.value}/${normalizedUrl.value}`
})

const commands = computed(() => {
  if (!proxyUrl.value) return []
  const url = proxyUrl.value
  const token = currentToken.value

  // Check if it's a git clone URL
  const isGitUrl = /\/(?:info|git-)/.test(normalizedUrl.value)

  const cmds = []

  if (isGitUrl) {
    // Extract the repo URL (before /info/ or /git-)
    const repoUrl = normalizedUrl.value.replace(/\/(?:info|git-).*$/, '')
    const proxyRepoUrl = `${domain.value}/${repoUrl}`
    cmds.push({
      label: 'Git Clone',
      icon: Terminal,
      command: `git -c http.extraHeader="X-XN-Token: ${token}" clone ${proxyRepoUrl}`,
    })
  }

  if (!isGitUrl) {
    cmds.push({
      label: 'curl 下载',
      icon: Download,
      command: `curl -H "X-XN-Token: ${token}" -L -O ${url}`,
    })
    cmds.push({
      label: 'wget 下载',
      icon: Download,
      command: `wget --header="X-XN-Token: ${token}" ${url}`,
    })
  }

  cmds.push({
    label: '代理链接',
    icon: Link,
    command: `${url}${url.includes('?') ? '&' : '?'}token=${token}`,
  })

  return cmds
})

async function copyToClipboard(text: string, id: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = id
    setTimeout(() => (copied.value = ''), 2000)
  } catch {
    // Fallback
    const el = document.createElement('textarea')
    el.value = text
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
    copied.value = id
    setTimeout(() => (copied.value = ''), 2000)
  }
}
</script>

<template>
  <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
    <!-- Hero -->
    <div class="text-center mb-10">
      <div class="flex items-center justify-center gap-3 mb-3">
        <Github class="w-10 h-10" />
        <h1 class="text-3xl font-bold">GitHub 代理加速</h1>
      </div>
      <p class="text-[hsl(var(--muted-foreground))]">输入 GitHub 文件链接，生成加速下载链接</p>
    </div>

    <!-- Input -->
    <div class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm p-6 mb-6">
      <label class="block text-sm font-medium mb-2">GitHub 链接</label>
      <div class="flex gap-3">
        <input
          v-model="inputUrl"
          type="text"
          class="flex-1 px-4 py-2.5 rounded-md border border-[hsl(var(--input))] bg-transparent text-sm focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:ring-offset-1"
          placeholder="输入 GitHub 文件链接，例如：https://github.com/user/repo/releases/download/v1.0/file.zip"
        />
      </div>

      <div class="mt-3 text-xs text-[hsl(var(--muted-foreground))]">
        支持: releases, archive, blob/raw, raw.githubusercontent.com, gist, tags, git clone
      </div>
    </div>

    <!-- Token selector (only shown when multiple tokens exist) -->
    <div v-if="hasMultipleTokens && isValidGithubUrl" class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm p-4 mb-6">
      <label class="block text-sm font-medium mb-2">选择 Token</label>
      <div class="relative">
        <select
          v-model="selectedTokenIndex"
          class="w-full appearance-none px-4 py-2.5 pr-10 rounded-md border border-[hsl(var(--input))] bg-transparent text-sm focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:ring-offset-1"
        >
          <option v-for="(t, idx) in tokens" :key="t.id" :value="idx">
            {{ t.name }} ({{ t.token.slice(0, 8) }}...)
          </option>
        </select>
        <ChevronDown class="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[hsl(var(--muted-foreground))] pointer-events-none" />
      </div>
    </div>

    <!-- No token warning -->
    <div v-if="tokens.length === 0 && isValidGithubUrl" class="bg-orange-50 rounded-lg border border-orange-200 shadow-sm p-4 mb-6">
      <p class="text-sm text-orange-600">
        您还没有创建 Token，请前往
        <router-link to="/profile" class="underline font-medium">个人中心</router-link>
        创建 Token 后再生成代理链接。
      </p>
    </div>

    <!-- Results -->
    <div v-if="inputUrl && !isValidGithubUrl" class="bg-white rounded-lg border border-orange-200 shadow-sm p-4 mb-6">
      <p class="text-sm text-orange-600">请输入有效的 GitHub 链接</p>
    </div>

    <div v-if="isValidGithubUrl && tokens.length > 0" class="space-y-4">
      <div v-for="(cmd, idx) in commands" :key="idx" class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm p-4">
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-2">
            <component :is="cmd.icon" class="w-4 h-4 text-[hsl(var(--muted-foreground))]" />
            <span class="text-sm font-medium">{{ cmd.label }}</span>
          </div>
          <button @click="copyToClipboard(cmd.command, `cmd-${idx}`)" class="flex items-center gap-1 px-2.5 py-1 rounded text-xs border border-[hsl(var(--border))] hover:bg-[hsl(var(--secondary))] transition-colors">
            <Check v-if="copied === `cmd-${idx}`" class="w-3 h-3 text-green-500" />
            <Copy v-else class="w-3 h-3" />
            {{ copied === `cmd-${idx}` ? '已复制' : '复制' }}
          </button>
        </div>
        <div class="bg-[hsl(var(--secondary))] rounded-md p-3 font-mono text-xs break-all leading-relaxed">
          {{ cmd.command }}
        </div>
      </div>
    </div>
  </div>
</template>
