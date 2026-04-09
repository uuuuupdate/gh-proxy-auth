<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import api from '@/api'
import { Github, Eye, EyeOff } from 'lucide-vue-next'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const showPassword = ref(false)
const openRegistration = ref(false)

// MFA state
const mfaRequired = ref(false)
const mfaType = ref('')
const mfaUserId = ref(0)
const totpCode = ref('')
const availableMFAMethods = ref<string[]>([])

onMounted(async () => {
  try {
    const res = await api.get('/system/init-status')
    if (!res.data.initialized) {
      router.push('/register')
      return
    }
    openRegistration.value = res.data.open_registration
  } catch {
    // ignore
  }
})

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.post('/auth/login', {
      username: username.value,
      password: password.value,
    })

    if (res.data.require_mfa) {
      mfaRequired.value = true
      mfaType.value = res.data.mfa_type
      mfaUserId.value = res.data.user_id
      availableMFAMethods.value = res.data.available_mfa_methods || [res.data.mfa_type]
      loading.value = false
      return
    }

    auth.setAuth(res.data.token, res.data.user)
    router.push('/')
  } catch (err: any) {
    error.value = err.response?.data?.error || '登录失败'
  } finally {
    loading.value = false
  }
}

async function handleTOTPVerify() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.post('/auth/totp/verify', {
      user_id: mfaUserId.value,
      code: totpCode.value,
    })
    auth.setAuth(res.data.token, res.data.user)
    router.push('/')
  } catch (err: any) {
    error.value = err.response?.data?.error || '验证失败'
  } finally {
    loading.value = false
  }
}

async function handlePasskeyLogin() {
  error.value = ''
  loading.value = true
  try {
    // Begin passkey login
    const beginRes = await api.post('/auth/passkey/begin-login', {
      user_id: mfaUserId.value,
    })

    const options = beginRes.data.publicKey

    // Convert challenge and credential IDs
    options.challenge = base64URLToBuffer(options.challenge)
    if (options.allowCredentials) {
      options.allowCredentials = options.allowCredentials.map((c: any) => ({
        ...c,
        id: base64URLToBuffer(c.id),
      }))
    }

    const credential = await navigator.credentials.get({ publicKey: options }) as PublicKeyCredential
    if (!credential) {
      error.value = 'Passkey 认证被取消'
      loading.value = false
      return
    }

    const response = credential.response as AuthenticatorAssertionResponse

    const finishRes = await api.post(`/auth/passkey/finish-login?user_id=${mfaUserId.value}`, {
      id: credential.id,
      rawId: bufferToBase64URL(credential.rawId),
      type: credential.type,
      response: {
        authenticatorData: bufferToBase64URL(response.authenticatorData),
        clientDataJSON: bufferToBase64URL(response.clientDataJSON),
        signature: bufferToBase64URL(response.signature),
        userHandle: response.userHandle ? bufferToBase64URL(response.userHandle) : undefined,
      },
    })

    auth.setAuth(finishRes.data.token, finishRes.data.user)
    router.push('/')
  } catch (err: any) {
    error.value = err.response?.data?.error || 'Passkey 认证失败'
  } finally {
    loading.value = false
  }
}

function switchMFAMethod() {
  const other = availableMFAMethods.value.find(m => m !== mfaType.value)
  if (other) {
    mfaType.value = other
    error.value = ''
    totpCode.value = ''
  }
}

function base64URLToBuffer(base64url: string): ArrayBuffer {
  const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/')
  const pad = base64.length % 4
  const padded = pad ? base64 + '='.repeat(4 - pad) : base64
  const binary = atob(padded)
  const buffer = new ArrayBuffer(binary.length)
  const view = new Uint8Array(buffer)
  for (let i = 0; i < binary.length; i++) {
    view[i] = binary.charCodeAt(i)
  }
  return buffer
}

function bufferToBase64URL(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-4">
    <div class="w-full max-w-md">
      <div class="text-center mb-8">
        <div class="flex items-center justify-center gap-2 mb-2">
          <Github class="w-8 h-8" />
          <h1 class="text-2xl font-bold">GH Proxy Auth</h1>
        </div>
        <p class="text-sm text-[hsl(var(--muted-foreground))]">GitHub 代理加速服务</p>
      </div>

      <div class="bg-white rounded-lg border border-[hsl(var(--border))] shadow-sm p-6">
        <!-- Login Form -->
        <template v-if="!mfaRequired">
          <h2 class="text-lg font-semibold mb-4">登录</h2>

          <div v-if="error" class="mb-4 p-3 rounded-md bg-red-50 text-red-600 text-sm">{{ error }}</div>

          <form @submit.prevent="handleLogin" class="space-y-4">
            <div>
              <label class="block text-sm font-medium mb-1.5">用户名</label>
              <input v-model="username" type="text" required class="w-full px-3 py-2 rounded-md border border-[hsl(var(--input))] bg-transparent text-sm focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:ring-offset-1" placeholder="请输入用户名" />
            </div>

            <div>
              <label class="block text-sm font-medium mb-1.5">密码</label>
              <div class="relative">
                <input v-model="password" :type="showPassword ? 'text' : 'password'" required class="w-full px-3 py-2 pr-10 rounded-md border border-[hsl(var(--input))] bg-transparent text-sm focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:ring-offset-1" placeholder="请输入密码" />
                <button type="button" @click="showPassword = !showPassword" class="absolute right-3 top-1/2 -translate-y-1/2 text-[hsl(var(--muted-foreground))]">
                  <Eye v-if="!showPassword" class="w-4 h-4" />
                  <EyeOff v-else class="w-4 h-4" />
                </button>
              </div>
            </div>

            <button type="submit" :disabled="loading" class="w-full py-2 px-4 rounded-md bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] text-sm font-medium hover:opacity-90 disabled:opacity-50 transition-opacity">
              {{ loading ? '登录中...' : '登录' }}
            </button>
          </form>

          <div v-if="openRegistration" class="mt-4 text-center">
            <router-link to="/register" class="text-sm text-[hsl(var(--primary))] hover:underline">还没有账号？注册</router-link>
          </div>
        </template>

        <!-- MFA Verification -->
        <template v-else>
          <h2 class="text-lg font-semibold mb-1">二步验证</h2>
          <p class="text-xs text-[hsl(var(--muted-foreground))] mb-4">
            当前方式：<span class="font-medium">{{ mfaType === 'totp' ? 'TOTP 验证码' : 'Passkey' }}</span>
          </p>

          <div v-if="error" class="mb-4 p-3 rounded-md bg-red-50 text-red-600 text-sm">{{ error }}</div>

          <!-- TOTP -->
          <template v-if="mfaType === 'totp'">
            <p class="text-sm text-[hsl(var(--muted-foreground))] mb-4">请输入您的 TOTP 验证码</p>
            <form @submit.prevent="handleTOTPVerify" class="space-y-4">
              <input v-model="totpCode" type="text" maxlength="6" required class="w-full px-3 py-2 rounded-md border border-[hsl(var(--input))] bg-transparent text-sm text-center tracking-widest focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))]" placeholder="000000" />
              <button type="submit" :disabled="loading" class="w-full py-2 px-4 rounded-md bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] text-sm font-medium hover:opacity-90 disabled:opacity-50">
                {{ loading ? '验证中...' : '验证' }}
              </button>
            </form>
          </template>

          <!-- Passkey -->
          <template v-else-if="mfaType === 'passkey'">
            <p class="text-sm text-[hsl(var(--muted-foreground))] mb-4">请使用您的 Passkey 完成验证</p>
            <button @click="handlePasskeyLogin" :disabled="loading" class="w-full py-2 px-4 rounded-md bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] text-sm font-medium hover:opacity-90 disabled:opacity-50">
              {{ loading ? '验证中...' : '使用 Passkey 验证' }}
            </button>
          </template>

          <!-- Switch MFA method (only shown when both are available) -->
          <button
            v-if="availableMFAMethods.length > 1"
            @click="switchMFAMethod"
            :disabled="loading"
            class="w-full mt-3 py-2 px-4 rounded-md border border-[hsl(var(--border))] text-sm hover:bg-[hsl(var(--secondary))] transition-colors disabled:opacity-50"
          >
            切换为{{ mfaType === 'totp' ? 'Passkey' : 'TOTP 验证码' }}验证
          </button>

          <button @click="mfaRequired = false; error = ''" class="w-full mt-3 py-2 px-4 rounded-md border border-[hsl(var(--border))] text-sm hover:bg-[hsl(var(--secondary))] transition-colors">
            返回登录
          </button>
        </template>
      </div>
    </div>
  </div>
</template>
