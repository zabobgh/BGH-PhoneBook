<script setup lang="ts">
import { ref } from 'vue'
import { supabaseService, isSupabaseConfigured } from '../services/supabase'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
  loginSuccess: []
}>()

const email = ref('')
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')

async function handleSubmit() {
  if (!email.value.trim() || !password.value.trim()) {
    errorMsg.value = 'กรุณากรอกอีเมลและรหัสผ่าน'
    return
  }

  errorMsg.value = ''
  loading.value = true

  try {
    if (isSupabaseConfigured()) {
      await supabaseService.login(email.value.trim(), password.value)
    } else {
      // Local fallback: simple check or prompt
      if (password.value === 'admin1234') {
        localStorage.setItem('bgh-local-admin', 'true')
      } else {
        throw new Error('รหัสผ่านไม่ถูกต้อง (สำหรับ local mode คือ admin1234)')
      }
    }
    emit('loginSuccess')
    emit('close')
  } catch (err: any) {
    errorMsg.value = err?.message || 'เข้าสู่ระบบไม่สำเร็จ กรุณาตรวจสอบอีเมลและรหัสผ่าน'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal modal-md" role="dialog" aria-modal="true">
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-badge-icon" style="background: rgba(30, 64, 175, 0.1); color: #2563eb;">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </div>
          <div>
            <h3 class="modal-title">เข้าสู่ระบบผู้ดูแลระบบ (Admin Login)</h3>
            <p class="modal-subtitle">สำหรับเจ้าหน้าที่ IT เพื่อจัดการข้อมูลสมุดโทรศัพท์</p>
          </div>
        </div>
        <button type="button" class="btn-close" @click="$emit('close')">✕</button>
      </div>

      <form @submit.prevent="handleSubmit" class="modal-body">
        <div v-if="errorMsg" class="login-error-alert">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="8" x2="12" y2="12" />
            <line x1="12" y1="16" x2="12.01" y2="16" />
          </svg>
          <span>{{ errorMsg }}</span>
        </div>

        <div class="form-group">
          <label class="form-label" for="admin-email">อีเมล (Admin Email)</label>
          <input
            id="admin-email"
            v-model="email"
            type="email"
            class="form-control"
            placeholder="เช่น admin@bgh.go.th"
            required
            autocomplete="email"
          />
        </div>

        <div class="form-group">
          <label class="form-label" for="admin-password">รหัสผ่าน (Password)</label>
          <input
            id="admin-password"
            v-model="password"
            type="password"
            class="form-control"
            placeholder="กรอกรหัสผ่านผู้ดูแลระบบ"
            required
            autocomplete="current-password"
          />
        </div>

        <div class="modal-footer" style="padding-top: 1rem; margin-top: 1rem; border-top: 1px solid var(--border-color);">
          <button type="button" class="btn btn-light" @click="$emit('close')">
            ยกเลิก
          </button>
          <button type="submit" class="btn btn-signal" :disabled="loading">
            <span v-if="loading">กำลังเข้าสู่ระบบ...</span>
            <span v-else>เข้าสู่ระบบ</span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-error-alert {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #dc2626;
  border-radius: 8px;
  font-size: 0.875rem;
  margin-bottom: 1rem;
}
</style>
