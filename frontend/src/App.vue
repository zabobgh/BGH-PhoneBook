<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api } from './services/api'
import { exportToExcel } from './services/excel'
import type { Entry, BuildingMeta, Stats, ImportPayload, LocationItem } from './types'
import EntryModal from './components/EntryModal.vue'
import ImportModal from './components/ImportModal.vue'
import RelocateModal from './components/RelocateModal.vue'
import AdminLoginModal from './components/AdminLoginModal.vue'
import LocationSettingsModal from './components/LocationSettingsModal.vue'
import { supabaseService, isSupabaseConfigured } from './services/supabase'
import defaultSeedData from './data/seed.json'

// Admin & Authentication State
const isAdmin = ref(api.isWails() || localStorage.getItem('bgh-local-admin') === 'true')
const adminLoginModalOpen = ref(false)

async function checkAdminStatus() {
  if (api.isWails()) {
    isAdmin.value = true
    return
  }
  if (isSupabaseConfigured()) {
    const user = await supabaseService.getCurrentUser()
    isAdmin.value = !!user
    supabaseService.onAuthStateChange((user) => {
      isAdmin.value = !!user
    })
  } else {
    isAdmin.value = localStorage.getItem('bgh-local-admin') === 'true'
  }
}

async function handleAdminLogout() {
  if (isSupabaseConfigured()) {
    await supabaseService.logout()
  }
  localStorage.removeItem('bgh-local-admin')
  isAdmin.value = false
  showToast('ออกจากระบบผู้ดูแลแล้ว', 'success')
}

function handleLoginSuccess() {
  isAdmin.value = true
  showToast('เข้าสู่ระบบผู้ดูแลสำเร็จ ยินดีต้อนรับ!', 'success')
}

// Local Cache Key & Initial Data Loader (0ms Instant Load)
const CACHE_KEY = 'bgh_entries_cache_v2'

function getInitialEntries(): Entry[] {
  try {
    const raw = localStorage.getItem(CACHE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed
      }
    }
  } catch (e) {
    console.warn('Failed to parse cache', e)
  }
  return (defaultSeedData as any[]).map((item, idx) => ({
    id: item.id || (idx + 1),
    building: item.building || '',
    floor: item.floor || '',
    department: item.department || '',
    internal_phone: item.internal_phone || '',
    external_phone: item.external_phone || '',
    sort_order: item.sort_order || (idx + 1),
  }))
}

// Master Entries State (Instant In-Memory Store)
const entries = ref<Entry[]>(getInitialEntries())
const activeBuilding = ref('') // '' means all buildings
const activeFloor = ref('') // '' means all floors
const q = ref('')
const loading = ref(false)
const error = ref('')

// Relocate Modal State
const relocateModalOpen = ref(false)
const relocatingEntries = ref<Entry[]>([])

// View Mode: 'cards' or 'table'
const viewMode = ref<'cards' | 'table'>(
  (localStorage.getItem('bgh-view-mode') as 'cards' | 'table') || 'cards'
)

function setViewMode(mode: 'cards' | 'table') {
  viewMode.value = mode
  localStorage.setItem('bgh-view-mode', mode)
}

// Dark Mode Theme
const isDark = ref(localStorage.getItem('bgh-theme') === 'dark')

function toggleTheme() {
  isDark.value = !isDark.value
  localStorage.setItem('bgh-theme', isDark.value ? 'dark' : 'light')
  document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light')
}

watch(
  isDark,
  (val) => {
    document.documentElement.setAttribute('data-theme', val ? 'dark' : 'light')
  },
  { immediate: true }
)

// Search Scope
const searchScope = ref<'all' | 'building'>('all')
const hasRequestedAll = ref(false)

function setSearchScope(scope: 'all' | 'building') {
  searchScope.value = scope
}

// Modals
const modalOpen = ref(false)
const importModalOpen = ref(false)
const locationSettingsModalOpen = ref(false)
const configuredLocations = ref<LocationItem[]>([])
const editing = ref<Entry | null>(null)

// Toast & Feedback
const copiedPhoneKey = ref<string | null>(null)
let copiedTimer: number | undefined

function isPhoneCopied(id: number, phone: string) {
  return copiedPhoneKey.value === `${id}::${phone.trim()}`
}

const toast = ref<{ show: boolean; msg: string; type: 'success' | 'error' }>({
  show: false,
  msg: '',
  type: 'success',
})
let toastTimer: number | undefined

function showToast(msg: string, type: 'success' | 'error' = 'success') {
  window.clearTimeout(toastTimer)
  toast.value = { show: true, msg, type }
  toastTimer = window.setTimeout(() => {
    toast.value.show = false
  }, 3200)
}

async function loadConfiguredLocations() {
  try {
    configuredLocations.value = await api.getLocations()
  } catch (e) {
    console.warn('Failed to load configured locations', e)
  }
}

async function handleLocationsUpdated(oldBuilding?: string, newBuilding?: string) {
  if (oldBuilding && newBuilding && activeBuilding.value === oldBuilding) {
    activeBuilding.value = newBuilding
  } else if (oldBuilding && !newBuilding && activeBuilding.value === oldBuilding) {
    activeBuilding.value = ''
  }
  await loadConfiguredLocations()
  await refreshMeta()
  if (activeBuilding.value || hasRequestedAll.value || q.value) {
    await load()
  }
}

// Computed Meta & Stats directly derived from in-memory entries (0ms instant sync)
const meta = computed<BuildingMeta[]>(() => {
  const counts: Record<string, number> = {}
  for (const e of entries.value) {
    if (e.building) {
      counts[e.building] = (counts[e.building] || 0) + 1
    }
  }
  for (const loc of configuredLocations.value) {
    if (loc.building && counts[loc.building] === undefined) {
      counts[loc.building] = 0
    }
  }
  return Object.entries(counts).map(([building, count]) => ({
    building,
    count,
  }))
})

const stats = computed<Stats>(() => {
  const bldgSet = new Set<string>()
  const floorSet = new Set<string>()
  for (const e of entries.value) {
    if (e.building) bldgSet.add(e.building)
    if (e.floor) floorSet.add(e.floor)
  }
  return {
    total_entries: entries.value.length,
    total_buildings: bldgSet.size,
    total_floors: floorSet.size,
  }
})

const buildings = computed(() => {
  const set = new Set(meta.value.map((x) => x.building))
  for (const loc of configuredLocations.value) {
    if (loc.building) set.add(loc.building)
  }
  return Array.from(set).filter(Boolean)
})

const allFloors = computed(() => {
  const set = new Set<string>()
  for (const e of entries.value) {
    if (!activeBuilding.value || e.building === activeBuilding.value) {
      if (e.floor) set.add(e.floor)
    }
  }
  for (const loc of configuredLocations.value) {
    if (!activeBuilding.value || loc.building === activeBuilding.value) {
      if (loc.floor) set.add(loc.floor)
    }
  }
  return Array.from(set).sort(floorSort)
})

function floorSort(a: string, b: string) {
  const val = (s: string) => {
    if (s.includes('B2') || s.includes('b2')) return -2
    if (s.includes('B1') || s.includes('b1') || s.includes('ใต้ดิน')) return -1
    if (s.includes('G') || s.includes('g')) return 0
    const m = s.match(/\d+/)
    return m ? Number(m[0]) + 1 : 999
  }
  return val(a) - val(b) || a.localeCompare(b, 'th')
}

// Flattened list for Table View & Search (Instant in-memory multi-word search in 0ms)
const filteredEntries = computed(() => {
  let list = entries.value

  // Scope filter: if an active building is selected
  // If not searching, OR if user specifically scoped search to 'building'
  if (activeBuilding.value && (!q.value.trim() || searchScope.value === 'building')) {
    list = list.filter((e) => e.building === activeBuilding.value)
  }

  // Floor filter
  if (activeFloor.value) {
    list = list.filter((e) => e.floor === activeFloor.value)
  }

  const term = q.value.trim().toLowerCase()
  if (term) {
    const words = term.split(/\s+/).filter(Boolean)
    list = list.filter((e) => {
      const dep = (e.department || '').toLowerCase()
      const internal = (e.internal_phone || '').toLowerCase()
      const external = (e.external_phone || '').toLowerCase()
      const bldg = (e.building || '').toLowerCase()
      const flr = (e.floor || '').toLowerCase()
      const combined = `${dep} ${internal} ${external} ${bldg} ${flr}`
      return words.every((w) => combined.includes(w))
    })
  }

  return list
})

// Grouped by floor for Cards View
const grouped = computed(() => {
  const map = new Map<string, Entry[]>()
  for (const e of filteredEntries.value) {
    if (!map.has(e.floor)) map.set(e.floor, [])
    map.get(e.floor)!.push(e)
  }
  return Array.from(map.entries()).sort((a, b) => floorSort(a[0], b[0]))
})

const totalFilteredCount = computed(() => filteredEntries.value.length)

// Relocation triggers
function openRelocate(e: Entry) {
  relocatingEntries.value = [e]
  relocateModalOpen.value = true
}

async function handleRelocate(ids: number[], building: string, floor: string) {
  try {
    const res = await api.relocate(ids, building, floor)
    relocateModalOpen.value = false
    showToast(`ย้ายสถานที่ไปยัง “${building} (${floor})” เรียบร้อยแล้ว`)
    await refreshMeta()
    await load()
  } catch (err) {
    showToast(err instanceof Error ? err.message : String(err), 'error')
  }
}

// Highlight Match Utility
function escapeRegExp(string: string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function escapeHtml(text: string) {
  const map: Record<string, string> = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;',
  }
  return text.replace(/[&<>"']/g, (m) => map[m])
}

function highlight(text: string) {
  if (!text) return ''
  const term = q.value.trim()
  const safeText = escapeHtml(text)
  if (!term) return safeText

  const words = term.split(/\s+/).filter(Boolean)
  if (!words.length) return safeText

  const pattern = new RegExp(`(${words.map(escapeRegExp).join('|')})`, 'gi')
  return safeText.replace(pattern, '<mark class="hl">$1</mark>')
}

// Parse multiple phone numbers separated by comma, slash, semicolon, or space-dot-space
function parsePhoneNumbers(phoneText: string): string[] {
  if (!phoneText) return []
  return phoneText
    .split(/[,;/]|\s+\.\s+/)
    .map((s) => s.trim())
    .filter((s) => Boolean(s) && /\d/.test(s))
}

// Copy phone number
async function copyPhone(phoneText: string, label = '', id?: number) {
  const cleanNumber = phoneText.trim()
  if (!cleanNumber) return
  try {
    await navigator.clipboard.writeText(cleanNumber)
    if ('vibrate' in navigator) {
      try { navigator.vibrate(35) } catch {}
    }
    if (id !== undefined) {
      copiedPhoneKey.value = `${id}::${cleanNumber}`
      window.clearTimeout(copiedTimer)
      copiedTimer = window.setTimeout(() => {
        if (copiedPhoneKey.value === `${id}::${cleanNumber}`) {
          copiedPhoneKey.value = null
        }
      }, 1800)
    }
    showToast(`คัดลอกเบอร์ ${cleanNumber} ${label ? `(${label})` : ''} แล้ว`)
  } catch {
    showToast('ไม่สามารถคัดลอกเบอร์โทรศัพท์ได้', 'error')
  }
}

// Background SWR Synchronizer
async function fetchAllEntries(silent = false) {
  if (!silent && entries.value.length === 0) {
    loading.value = true
  }
  error.value = ''
  try {
    const data = await api.list('', '', '')
    if (Array.isArray(data) && data.length > 0) {
      entries.value = data
      try {
        localStorage.setItem(CACHE_KEY, JSON.stringify(data))
      } catch {}
    }
  } catch (e) {
    console.error('Failed to fetch entries:', e)
    if (entries.value.length === 0) {
      error.value = e instanceof Error ? e.message : String(e)
    }
  } finally {
    loading.value = false
  }
}

async function refreshMeta() {
  await fetchAllEntries(true)
}

const matchedBuildings = computed(() => {
  const set = new Set<string>()
  for (const e of filteredEntries.value) {
    if (e.building) set.add(e.building)
  }
  return Array.from(set)
})

async function load() {
  await fetchAllEntries(true)
}

function selectBuilding(name: string) {
  activeBuilding.value = name
  activeFloor.value = ''
  if (!name) {
    hasRequestedAll.value = true
    searchScope.value = 'all'
  } else {
    hasRequestedAll.value = false
    if (q.value) searchScope.value = 'building'
  }
}

function selectFloor(floor: string) {
  activeFloor.value = activeFloor.value === floor ? '' : floor
}

// Search handler: Instant reactive 0ms search
function onSearchInput() {
  // Computed property 'filteredEntries' handles instant real-time filtering in 0ms!
}

function clearSearch() {
  q.value = ''
  searchInputRef.value?.focus()
}

// Add / Edit / Delete
function addNew() {
  editing.value = null
  modalOpen.value = true
}

function edit(e: Entry) {
  editing.value = { ...e }
  modalOpen.value = true
}

async function save(e: Entry) {
  try {
    if (e.id) {
      await api.update(e)
      showToast(`บันทึกข้อมูล “${e.department}” เรียบร้อย`)
    } else {
      await api.create({ ...e, id: undefined } as any)
      showToast(`เพิ่มหน่วยงาน “${e.department}” เรียบร้อย`)
    }
    modalOpen.value = false
    await fetchAllEntries(true)
  } catch (err) {
    showToast(err instanceof Error ? err.message : String(err), 'error')
  }
}

async function remove(e: Entry) {
  if (!confirm(`คุณต้องการลบข้อมูล “${e.department}” ใช่หรือไม่?`)) return
  try {
    await api.remove(e.id)
    showToast(`ลบข้อมูล “${e.department}” เรียบร้อย`)
    await fetchAllEntries(true)
  } catch (err) {
    showToast(err instanceof Error ? err.message : String(err), 'error')
  }
}

// Excel Export & Import
function handleExportExcel() {
  if (!filteredEntries.value.length) {
    showToast('ไม่มีข้อมูลสำหรับส่งออก', 'error')
    return
  }
  const title = activeBuilding.value
    ? `BGH-PhoneBook-${activeBuilding.value}.xlsx`
    : 'BGH-PhoneBook-เบอร์โทรภายในทั้งหมด.xlsx'
  exportToExcel(filteredEntries.value, title)
  showToast(`ส่งออกไฟล์ Excel สำเร็จ (${filteredEntries.value.length} รายการ)`)
}

async function handleImport(payload: ImportPayload) {
  try {
    const res = await api.importBatch(payload)
    importModalOpen.value = false
    showToast(`นำเข้าข้อมูลสำเร็จ ${res.imported} รายการ`)
    await fetchAllEntries(true)
  } catch (err) {
    showToast(err instanceof Error ? err.message : String(err), 'error')
  }
}

// Keyboard shortcuts
const searchInputRef = ref<HTMLInputElement | null>(null)

function onKeyDown(e: KeyboardEvent) {
  if (
    (e.key === '/' || ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K'))) &&
    document.activeElement !== searchInputRef.value
  ) {
    e.preventDefault()
    searchInputRef.value?.focus()
    searchInputRef.value?.select()
  } else if (e.key === 'Escape') {
    if (adminLoginModalOpen.value) adminLoginModalOpen.value = false
    else if (locationSettingsModalOpen.value) locationSettingsModalOpen.value = false
    else if (modalOpen.value) modalOpen.value = false
    else if (importModalOpen.value) importModalOpen.value = false
    else if (relocateModalOpen.value) relocateModalOpen.value = false
    else if (q.value) clearSearch()
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onKeyDown)
  await checkAdminStatus()
  await loadConfiguredLocations()
  // Background SWR sync with Supabase / SQLite
  await fetchAllEntries(entries.value.length > 0)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <div>
    <!-- Top Hero Header -->
    <header class="app-header">
      <div class="header-container">
        <div class="brand-section">
          <div class="brand-logo-badge">
            <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"/>
            </svg>
          </div>
          <div class="brand-text">
            <div class="brand-eyebrow-row">
              <span class="eyebrow">BGH · DIRECTORY INFRASTRUCTURE</span>
              <span class="live-status-pill">
                <span class="live-dot"></span>
                <span>SYSTEM LIVE</span>
              </span>
            </div>
            <h1>สมุดโทรศัพท์ภายใน</h1>
            <p class="subtitle">โรงพยาบาลบ้านแพ้ว (องค์การมหาชน)</p>
          </div>
        </div>

        <div class="header-right-tools">
          <!-- Stats Bar -->
          <div class="stats-bar">
            <div class="stat-pill">
              <span class="lbl">ENTRIES</span>
              <span class="num">{{ stats.total_entries || meta.reduce((s, x) => s + x.count, 0) }}</span>
            </div>
            <div class="stat-pill">
              <span class="lbl">BUILDINGS</span>
              <span class="num">{{ stats.total_buildings || meta.length }}</span>
            </div>
            <div class="stat-pill">
              <span class="lbl">FLOORS</span>
              <span class="num">{{ stats.total_floors || allFloors.length }}</span>
            </div>
          </div>

          <!-- Theme Toggle Button -->
          <button
            type="button"
            class="theme-btn"
            :title="isDark ? 'เปลี่ยนเป็นธีมสว่าง (Warm Paper)' : 'เปลี่ยนเป็นธีมมืด (Terminal)'"
            @click="toggleTheme"
          >
            <span v-if="isDark">☀️ LIGHT</span>
            <span v-else>🌙 DARK</span>
          </button>

          <!-- Admin Mode Indicator / Login Button -->
          <div v-if="isAdmin" class="admin-user-pill">
            <span class="admin-badge">👨‍💼 เจ้าหน้าที่ (Admin)</span>
            <button type="button" class="btn-logout-mini" title="ออกจากระบบผู้ดูแล" @click="handleAdminLogout">
              ออก
            </button>
          </div>
          <button
            v-else
            type="button"
            class="btn-admin-login-entry"
            title="สำหรับเจ้าหน้าที่ IT เข้าสู่ระบบเพื่อจัดการข้อมูล"
            @click="adminLoginModalOpen = true"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
            <span>เข้าสู่ระบบเจ้าหน้าที่</span>
          </button>
        </div>
      </div>
    </header>

    <!-- Hero Omni-Search Section (Primary Tool for Staff & IT) -->
    <section class="hero-search-section">
      <div class="hero-search-card" :class="{ 'has-query': Boolean(q) }">
        <div class="hero-search-bar-row">
          <div class="search-input-wrapper">
            <span class="search-hero-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5">
                <circle cx="11" cy="11" r="8"/>
                <line x1="21" y1="21" x2="16.65" y2="16.65"/>
              </svg>
            </span>
            <input
              ref="searchInputRef"
              v-model="q"
              class="hero-search-input"
              placeholder="ค้นหาชื่อหน่วยงาน, เบอร์ภายใน (เช่น 1041), เบอร์ตรง, ตึก, ชั้น..."
              @input="onSearchInput"
            />
            <div class="search-input-right-tools">
              <span v-if="loading" class="search-spinner" title="กำลังค้นหา..."></span>
              <span v-else-if="q" class="search-count-badge">{{ totalFilteredCount }} พบ</span>
              <button
                v-if="q"
                type="button"
                class="hero-search-clear-btn"
                title="ล้างคำค้นหา (Esc)"
                @click="clearSearch"
              >
                ✕
              </button>
              <div class="kbd-shortcut-pill desktop-only" title="กดปุ่มลัดเพื่อค้นหา" @click="searchInputRef?.focus()">
                <span class="kbd">Ctrl</span><span class="kbd-plus">+</span><span class="kbd">K</span>
              </div>
            </div>
          </div>

          <!-- Scope Switcher Buttons (Only shown when a specific building is selected) -->
          <div v-if="activeBuilding" class="search-scope-group">
            <button
              type="button"
              class="scope-btn"
              :class="{ active: searchScope === 'all' }"
              title="ค้นหาครอบคลุมทุกตึกทุกชั้นทั่วทั้งโรงพยาบาล"
              @click="setSearchScope('all')"
            >
              <span class="scope-icon">🌐</span>
              <span>ทุกตึก</span>
            </button>
            <button
              type="button"
              class="scope-btn"
              :class="{ active: searchScope === 'building' }"
              :title="`ค้นหาเฉพาะใน ${activeBuilding}`"
              @click="setSearchScope('building')"
            >
              <span class="scope-icon">🏢</span>
              <span>เฉพาะ {{ activeBuilding }}</span>
            </button>
          </div>
        </div>

        <!-- Active Search Live Feedback Strip (Desktop/Tablet) -->
        <div v-if="q" class="search-feedback-banner desktop-only">
          <div class="feedback-info">
            <span class="feedback-badge">ผลการค้นหา</span>
            <span>
              พบ <strong>{{ totalFilteredCount }}</strong> รายการ ใน <strong>{{ matchedBuildings.length }}</strong> ตึก จากคำค้นหา “<strong>{{ q }}</strong>”
            </span>
            <span v-if="searchScope === 'all'" class="scope-indicator-tag">🌐 ทั่วทั้งโรงพยาบาล</span>
            <span v-else class="scope-indicator-tag">🏢 เฉพาะ {{ activeBuilding }}</span>
          </div>
          <button type="button" class="btn-clear-inline" @click="clearSearch">
            ✕ ล้างคำค้นหา
          </button>
        </div>
      </div>
    </section>

    <!-- Mobile Horizontal Building Scroller (Phone / Tablet) - Only shown in browse mode when NOT searching -->
    <div v-if="!q" class="mobile-building-bar">
      <div class="mobile-building-scroller">
        <button
          type="button"
          class="mobile-b-pill"
          :class="{ active: hasRequestedAll && activeBuilding === '' }"
          @click="selectBuilding('')"
        >
          <span class="b-pill-icon">🏢</span>
          <span>ทุกอาคาร</span>
          <span class="b-pill-count">{{ meta.reduce((s, x) => s + x.count, 0) }}</span>
        </button>
        <button
          v-for="b in meta"
          :key="b.building"
          type="button"
          class="mobile-b-pill"
          :class="{ active: b.building === activeBuilding }"
          @click="selectBuilding(b.building)"
        >
          <span>{{ b.building }}</span>
          <span class="b-pill-count">{{ b.count }}</span>
        </button>
      </div>
    </div>

    <!-- Main Content Layout -->
    <div class="app-layout">
      <!-- Sidebar -->
      <aside class="sidebar-panel">

        <!-- Building Navigation -->
        <div class="sidebar-section-title">
          <span>BUILDING REGIONS</span>
          <span>{{ meta.length }} REGIONS</span>
        </div>

        <nav class="building-nav">
          <!-- All Buildings Option -->
          <button
            type="button"
            class="nav-item"
            :class="{ active: hasRequestedAll && activeBuilding === '' }"
            @click="selectBuilding('')"
          >
            <span class="nav-label-wrap">
              <span class="nav-idx mono">00</span>
              <span>🏢 ทุกอาคาร</span>
            </span>
            <span class="nav-badge">{{ meta.reduce((s, x) => s + x.count, 0) }}</span>
          </button>

          <!-- Individual Buildings -->
          <button
            v-for="(b, bIdx) in meta"
            :key="b.building"
            type="button"
            class="nav-item"
            :class="{ active: b.building === activeBuilding }"
            @click="selectBuilding(b.building)"
          >
            <span class="nav-label-wrap">
              <span class="nav-idx mono">{{ String(bIdx + 1).padStart(2, '0') }}</span>
              <span>{{ b.building }}</span>
            </span>
            <span class="nav-badge">{{ b.count }}</span>
          </button>
        </nav>

        <!-- Sidebar Actions (Only visible for logged-in Admin) -->
        <div v-if="isAdmin" class="sidebar-actions">
          <button type="button" class="btn btn-signal" @click="addNew">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.5">
              <line x1="12" y1="5" x2="12" y2="19"/>
              <line x1="5" y1="12" x2="19" y2="12"/>
            </svg>
            <span>+ เพิ่มหน่วยงานใหม่</span>
          </button>

          <button type="button" class="btn btn-outline btn-sm w-full btn-loc-manage" style="width: 100%; margin-top: 0.4rem; justify-content: center;" @click="locationSettingsModalOpen = true">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 21h18"/>
              <path d="M5 21V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16"/>
              <path d="M9 9h1"/>
              <path d="M9 13h1"/>
              <path d="M9 17h1"/>
            </svg>
            <span>⚙️ จัดการอาคารและชั้น</span>
          </button>

          <div class="action-row-2">
            <button type="button" class="btn btn-excel btn-sm" title="ส่งออก Excel" @click="handleExportExcel">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="7 10 12 15 17 10"/>
                <line x1="12" y1="15" x2="12" y2="3"/>
              </svg>
              <span>ส่งออก Excel</span>
            </button>
            <button type="button" class="btn btn-outline btn-sm" title="นำเข้า Excel" @click="importModalOpen = true">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="17 8 12 3 7 8"/>
                <line x1="12" y1="3" x2="12" y2="15"/>
              </svg>
              <span>นำเข้า Excel</span>
            </button>
          </div>

          <div class="action-row-2">
            <button type="button" class="btn btn-light btn-sm" title="สำรองไฟล์ JSON" @click="api.backupDownload">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
                <polyline points="17 21 17 13 7 13 7 21"/>
              </svg>
              <span>สำรอง JSON</span>
            </button>
            <button type="button" class="btn btn-light btn-sm" title="พิมพ์หน้านี้" onclick="window.print()">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="6 9 6 2 18 2 18 9"/>
                <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"/>
                <rect x="6" y="14" width="12" height="8"/>
              </svg>
              <span>พิมพ์</span>
            </button>
          </div>
        </div>
      </aside>

      <!-- Main Panel -->
      <main class="main-content">
        <!-- Building Hub Landing View (Zero-Egress on initial visit: select building first or search) -->
        <div v-if="!q && !activeBuilding && !hasRequestedAll" class="building-hub-view">
          <div class="hub-header">
            <div class="hub-badge">🏢 DIRECTORY BY BUILDING</div>
            <h2>เลือกอาคารที่ต้องการดูข้อมูล</h2>
            <p>กรุณาคลิกเลือกอาคารด้านล่างเพื่อดูรายชื่อแผนกและเบอร์โทรศัพท์เฉพาะอาคาร หรือพิมพ์ค้นหาในช่องด้านบน</p>
          </div>

          <div class="hub-building-grid">
            <button
              v-for="(b, bIdx) in meta"
              :key="b.building"
              type="button"
              class="hub-building-card"
              @click="selectBuilding(b.building)"
            >
              <div class="hub-card-icon-badge">🏢</div>
              <div class="hub-card-content">
                <span class="hub-card-idx mono">{{ String(bIdx + 1).padStart(2, '0') }}</span>
                <h3 class="hub-card-title">{{ b.building }}</h3>
                <span class="hub-card-count">{{ b.count }} หน่วยงาน</span>
              </div>
              <div class="hub-card-arrow">
                <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5">
                  <line x1="5" y1="12" x2="19" y2="12"/>
                  <polyline points="12 5 19 12 12 19"/>
                </svg>
              </div>
            </button>
          </div>

          <div class="hub-footer-row">
            <button type="button" class="btn-hub-all" @click="selectBuilding('')">
              <span class="hub-all-icon">🌐</span>
              <span>ดูข้อมูลทุกอาคารทั่วทั้งโรงพยาบาล ({{ meta.reduce((s, x) => s + x.count, 0) }} หน่วยงาน)</span>
            </button>
          </div>
        </div>

        <template v-else>
          <!-- Control Bar -->
          <section class="control-bar" :class="{ 'is-searching': Boolean(q) }">
          <div class="control-info">
            <h2>
              <span v-if="q">ผลการค้นหา: “{{ q }}”</span>
              <span v-else>{{ activeBuilding || 'แสดงข้อมูลทุกตึก / อาคาร' }}</span>
            </h2>
            <p>
              พบ <strong style="color: var(--brand-blue)">{{ totalFilteredCount }}</strong> หน่วยงาน
              <span v-if="activeFloor"> (กรองเฉพาะ {{ activeFloor }})</span>
              <span v-if="q && searchScope === 'all'"> (ค้นหาทุกตึกทั่วทั้งโรงพยาบาล)</span>
              <span v-else-if="q && activeBuilding"> (ค้นหาเฉพาะ {{ activeBuilding }})</span>
            </p>
          </div>

          <div class="control-actions">
            <!-- View Mode Switcher -->
            <div class="view-switch-group">
              <button
                type="button"
                class="view-btn"
                :class="{ active: viewMode === 'cards' }"
                title="มุมมองการ์ด (Card Grid View)"
                @click="setViewMode('cards')"
              >
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="3" y="3" width="7" height="7"/>
                  <rect x="14" y="3" width="7" height="7"/>
                  <rect x="14" y="14" width="7" height="7"/>
                  <rect x="3" y="14" width="7" height="7"/>
                </svg>
                <span>การ์ด</span>
              </button>
              <button
                type="button"
                class="view-btn"
                :class="{ active: viewMode === 'table' }"
                title="มุมมองตาราง (Compact Table View)"
                @click="setViewMode('table')"
              >
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="8" y1="6" x2="21" y2="6"/>
                  <line x1="8" y1="12" x2="21" y2="12"/>
                  <line x1="8" y1="18" x2="21" y2="18"/>
                  <line x1="3" y1="6" x2="3.01" y2="6"/>
                  <line x1="3" y1="12" x2="3.01" y2="12"/>
                  <line x1="3" y1="18" x2="3.01" y2="18"/>
                </svg>
                <span>ตาราง</span>
              </button>
            </div>

            <button v-if="isAdmin" type="button" class="btn btn-outline" @click="handleExportExcel">
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="7 10 12 15 17 10"/>
                <line x1="12" y1="15" x2="12" y2="3"/>
              </svg>
              <span>ส่งออก Excel</span>
            </button>
            <button v-if="isAdmin" type="button" class="btn btn-signal" @click="addNew">
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.5">
                <line x1="12" y1="5" x2="12" y2="19"/>
                <line x1="5" y1="12" x2="19" y2="12"/>
              </svg>
              <span>+ เพิ่มข้อมูล</span>
            </button>
          </div>
        </section>

        <!-- Floor Quick Filter Bar (Browse Mode Only) -->
        <div v-if="!q && allFloors.length > 1" class="floor-pills-bar">
          <span class="floor-pill-label">เลือกชั้น:</span>
          <button
            type="button"
            class="floor-pill-btn"
            :class="{ active: activeFloor === '' }"
            @click="activeFloor = ''"
          >
            ทุกชั้น
          </button>
          <button
            v-for="fl in allFloors"
            :key="fl"
            type="button"
            class="floor-pill-btn"
            :class="{ active: activeFloor === fl }"
            @click="selectFloor(fl)"
          >
            {{ fl }}
          </button>
        </div>

        <!-- Error & Loading States -->
        <div v-if="error" class="alert-box alert-danger">
          <strong>เกิดข้อผิดพลาด:</strong> {{ error }}
        </div>

        <div v-if="loading" class="empty-state">
          <div class="spinner"></div>
          <p>กำลังค้นหาข้อมูล...</p>
        </div>

        <!-- Content Display -->
        <template v-else>
          <!-- 1A. Search Mode (Direct Stream, No Group Dividers for Zero-Scroll Speed) -->
          <div v-if="q && viewMode === 'cards' && filteredEntries.length" class="search-stream-view">
            <div class="entries-grid">
              <article
                v-for="e in filteredEntries"
                :key="e.id"
                class="entry-card"
              >
                <!-- Card Header: Title + Actions -->
                <div class="card-header-row">
                  <div class="card-title-group">
                    <h3 class="entry-dept" v-html="highlight(e.department)"></h3>
                  </div>

                  <!-- Actions: Relocate, Edit, Delete -->
                  <div v-if="isAdmin" class="entry-actions">
                    <button
                      type="button"
                      class="btn-icon btn-relocate"
                      title="ย้ายสถานที่ (เปลี่ยนตึก/ชั้น)"
                      @click.stop="openRelocate(e)"
                    >
                      <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M12 2a8 8 0 0 0-8 8c0 5.25 8 12 8 12s8-6.75 8-12a8 8 0 0 0-8-8z"/>
                        <circle cx="12" cy="10" r="3"/>
                      </svg>
                    </button>
                    <button
                      type="button"
                      class="btn-icon"
                      title="แก้ไขข้อมูล"
                      @click.stop="edit(e)"
                    >
                      <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                        <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                      </svg>
                    </button>
                    <button
                      type="button"
                      class="btn-icon btn-danger"
                      title="ลบข้อมูล"
                      @click.stop="remove(e)"
                    >
                      <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="3 6 5 6 21 6"/>
                        <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                      </svg>
                    </button>
                  </div>
                </div>

                <!-- Card Footer: Meta Chips + Phone Badges -->
                <div class="card-footer-row">
                  <div class="entry-meta-tags">
                    <span class="bldg-chip" v-html="highlight(e.building)"></span>
                    <span v-if="e.floor" class="floor-chip-mini">📍 {{ e.floor }}</span>
                    <span v-if="e.external_phone" class="ext-phone-text">
                      <span class="ext-lbl">สายนอก:</span>
                      <span class="ext-phone-pills">
                        <span
                          v-for="(ep, epIdx) in parsePhoneNumbers(e.external_phone)"
                          :key="epIdx"
                          class="ext-phone-chip-wrap"
                        >
                          <button
                            type="button"
                            class="ext-phone-btn"
                            :class="{ copied: isPhoneCopied(e.id, ep) }"
                            :title="`คลิกเพื่อคัดลอกเบอร์สายนอก ${ep}`"
                            @click.stop="copyPhone(ep, e.department, e.id)"
                          >
                            <span v-if="isPhoneCopied(e.id, ep)">✓ {{ ep }}</span>
                            <span v-else v-html="highlight(ep)"></span>
                          </button>
                          <a
                            :href="'tel:' + ep.replace(/[^0-9]/g, '')"
                            class="ext-tel-link"
                            :title="`โทรออก ${ep}`"
                            @click.stop
                          >
                            📞
                          </a>
                        </span>
                      </span>
                    </span>
                  </div>

                  <!-- Internal Phone & Quick Copy -->
                  <div class="phone-badge-group">
                    <template v-if="parsePhoneNumbers(e.internal_phone).length">
                      <button
                        v-for="(p, pIdx) in parsePhoneNumbers(e.internal_phone)"
                        :key="pIdx"
                        type="button"
                        class="phone-badge"
                        :class="{ copied: isPhoneCopied(e.id, p) }"
                        :title="`คลิกเพื่อคัดลอกเบอร์ ${p}`"
                        @click.stop="copyPhone(p, e.department, e.id)"
                      >
                        <span v-if="isPhoneCopied(e.id, p)">✓ คัดลอกแล้ว</span>
                        <span v-else>
                          <span class="copy-icon">📞</span>
                          <span v-html="highlight(p)"></span>
                        </span>
                      </button>
                    </template>
                    <span v-else class="phone-badge phone-badge-empty" title="ไม่มีเบอร์ภายใน">
                      <span class="copy-icon">📞</span>
                      <span>—</span>
                    </span>
                  </div>
                </div>
              </article>
            </div>
          </div>

          <!-- 1B. Browse Mode (Grouped by Floor) -->
          <div v-else-if="!q && viewMode === 'cards' && grouped.length">
            <section v-for="[floor, items] in grouped" :key="floor" class="floor-group">
              <div class="floor-title-header">
                <div class="floor-tag">
                  <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5">
                    <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
                    <line x1="3" y1="9" x2="21" y2="9"/>
                    <line x1="9" y1="21" x2="9" y2="9"/>
                  </svg>
                  <span>{{ floor }}</span>
                </div>
                <span class="floor-dept-count">{{ items.length }} หน่วยงาน</span>
                <div class="floor-divider"></div>
              </div>

              <div class="entries-grid">
                <article
                  v-for="e in items"
                  :key="e.id"
                  class="entry-card"
                >
                  <!-- Card Header: Title + Actions -->
                  <div class="card-header-row">
                    <div class="card-title-group">
                      <h3 class="entry-dept" v-html="highlight(e.department)"></h3>
                    </div>

                    <!-- Actions: Relocate, Edit, Delete -->
                    <div v-if="isAdmin" class="entry-actions">
                      <button
                        type="button"
                        class="btn-icon btn-relocate"
                        title="ย้ายสถานที่ (เปลี่ยนตึก/ชั้น)"
                        @click.stop="openRelocate(e)"
                      >
                        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M12 2a8 8 0 0 0-8 8c0 5.25 8 12 8 12s8-6.75 8-12a8 8 0 0 0-8-8z"/>
                          <circle cx="12" cy="10" r="3"/>
                        </svg>
                      </button>
                      <button
                        type="button"
                        class="btn-icon"
                        title="แก้ไขข้อมูล"
                        @click.stop="edit(e)"
                      >
                        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                        </svg>
                      </button>
                      <button
                        type="button"
                        class="btn-icon btn-danger"
                        title="ลบข้อมูล"
                        @click.stop="remove(e)"
                      >
                        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                          <polyline points="3 6 5 6 21 6"/>
                          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                        </svg>
                      </button>
                    </div>
                  </div>

                  <!-- Card Footer: Meta Chips + Phone Badges -->
                  <div class="card-footer-row">
                    <div class="entry-meta-tags">
                      <span class="bldg-chip" v-html="highlight(e.building)"></span>
                      <span v-if="e.floor" class="floor-chip-mini">{{ e.floor }}</span>
                      <span v-if="e.external_phone" class="ext-phone-text">
                        <span class="ext-lbl">สายนอก:</span>
                        <span class="ext-phone-pills">
                          <span
                            v-for="(ep, epIdx) in parsePhoneNumbers(e.external_phone)"
                            :key="epIdx"
                            class="ext-phone-chip-wrap"
                          >
                            <button
                              type="button"
                              class="ext-phone-btn"
                              :class="{ copied: isPhoneCopied(e.id, ep) }"
                              :title="`คลิกเพื่อคัดลอกเบอร์สายนอก ${ep}`"
                              @click.stop="copyPhone(ep, e.department, e.id)"
                            >
                              <span v-if="isPhoneCopied(e.id, ep)">✓ {{ ep }}</span>
                              <span v-else v-html="highlight(ep)"></span>
                            </button>
                            <a
                              :href="'tel:' + ep.replace(/[^0-9]/g, '')"
                              class="ext-tel-link"
                              :title="`โทรออก ${ep}`"
                              @click.stop
                            >
                              📞
                            </a>
                          </span>
                        </span>
                      </span>
                    </div>

                    <!-- Internal Phone & Quick Copy -->
                    <div class="phone-badge-group">
                      <template v-if="parsePhoneNumbers(e.internal_phone).length">
                        <button
                          v-for="(p, pIdx) in parsePhoneNumbers(e.internal_phone)"
                          :key="pIdx"
                          type="button"
                          class="phone-badge"
                          :class="{ copied: isPhoneCopied(e.id, p) }"
                          :title="`คลิกเพื่อคัดลอกเบอร์ ${p}`"
                          @click.stop="copyPhone(p, e.department, e.id)"
                        >
                          <span v-if="isPhoneCopied(e.id, p)">✓ คัดลอกแล้ว</span>
                          <span v-else>
                            <span class="copy-icon">📞</span>
                            <span v-html="highlight(p)"></span>
                          </span>
                        </button>
                      </template>
                      <span v-else class="phone-badge phone-badge-empty" title="ไม่มีเบอร์ภายใน">
                        <span class="copy-icon">📞</span>
                        <span>—</span>
                      </span>
                    </div>
                  </div>
                </article>
              </div>
            </section>
          </div>

          <!-- 2. Compact Table View -->
          <div v-else-if="viewMode === 'table' && filteredEntries.length" class="table-view-container">
            <div class="data-table-scroll">
              <table class="bgh-table">
                <thead>
                  <tr>
                    <th style="width: 50px;">#</th>
                    <th>ตึก / อาคาร</th>
                    <th>ชั้น</th>
                    <th>ชื่อหน่วยงาน</th>
                    <th>เบอร์ภายใน</th>
                    <th>เบอร์สายนอก</th>
                    <th v-if="isAdmin" style="width: 120px; text-align: center;">จัดการ</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(e, idx) in filteredEntries"
                    :key="e.id"
                  >
                    <td class="table-num">{{ idx + 1 }}</td>
                    <td class="table-bldg" v-html="highlight(e.building)"></td>
                    <td><span class="floor-chip">{{ e.floor }}</span></td>
                    <td class="table-dept" v-html="highlight(e.department)"></td>
                    <td>
                      <div v-if="parsePhoneNumbers(e.internal_phone).length" class="table-phone-list">
                        <button
                          v-for="(p, pIdx) in parsePhoneNumbers(e.internal_phone)"
                          :key="pIdx"
                          type="button"
                          class="table-phone-btn"
                          :class="{ copied: isPhoneCopied(e.id, p) }"
                          :title="`คลิกเพื่อคัดลอกเบอร์ ${p}`"
                          @click="copyPhone(p, e.department, e.id)"
                        >
                          <span v-if="isPhoneCopied(e.id, p)">✓ คัดลอกแล้ว</span>
                          <span v-else>📞 <span v-html="highlight(p)"></span></span>
                        </button>
                      </div>
                      <span v-else style="color: var(--text-light);">-</span>
                    </td>
                    <td>
                      <div v-if="parsePhoneNumbers(e.external_phone).length" class="table-ext-list">
                        <button
                          v-for="(ep, epIdx) in parsePhoneNumbers(e.external_phone)"
                          :key="epIdx"
                          type="button"
                          class="table-ext-btn"
                          :class="{ copied: isPhoneCopied(e.id, ep) }"
                          :title="`คลิกเพื่อคัดลอกเบอร์สายนอก ${ep}`"
                          @click="copyPhone(ep, e.department, e.id)"
                        >
                          <span v-if="isPhoneCopied(e.id, ep)">✓ {{ ep }}</span>
                          <span v-else v-html="highlight(ep)"></span>
                        </button>
                      </div>
                      <span v-else style="color: var(--text-light);">-</span>
                    </td>
                    <td v-if="isAdmin">
                      <div class="entry-actions" style="justify-content: center;">
                        <button
                          type="button"
                          class="btn-icon btn-relocate"
                          title="ย้ายสถานที่ (เปลี่ยนตึก/ชั้น)"
                          @click="openRelocate(e)"
                        >
                          <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M12 2a8 8 0 0 0-8 8c0 5.25 8 12 8 12s8-6.75 8-12a8 8 0 0 0-8-8z"/>
                            <circle cx="12" cy="10" r="3"/>
                          </svg>
                        </button>
                        <button type="button" class="btn-icon" title="แก้ไขข้อมูล" @click="edit(e)">
                          <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                          </svg>
                        </button>
                        <button type="button" class="btn-icon btn-danger" title="ลบข้อมูล" @click="remove(e)">
                          <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                            <polyline points="3 6 5 6 21 6"/>
                            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                          </svg>
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Empty State -->
          <div v-else class="empty-state">
            <div class="empty-icon">🔍</div>
            <h3>ไม่พบข้อมูลเบอร์โทรศัพท์</h3>
            <p v-if="q">ไม่พบหน่วยงานหรือเบอร์โทรที่ตรงกับ “{{ q }}”</p>
            <!-- Scope Switch Suggestion -->
            <div v-if="q && activeBuilding && searchScope === 'building'" class="scope-switch-card">
              <p>คำค้นหานี้อาจจะอยู่ในตึกอื่นของโรงพยาบาล</p>
              <button type="button" class="btn btn-signal btn-sm" @click="setSearchScope('all')">
                🌐 สลับไปค้นหาทุกตึกทั่วทั้งโรงพยาบาล
              </button>
            </div>
            <p v-else-if="!q">ยังไม่มีข้อมูลในหมวดหมู่นี้</p>
            <button v-if="q" type="button" class="btn btn-outline" style="margin-top: 10px;" @click="clearSearch">ล้างการค้นหา</button>
            <button v-else-if="isAdmin" type="button" class="btn btn-primary" @click="addNew">+ เพิ่มหน่วยงานแรก</button>
          </div>
        </template>
        </template>
      </main>
    </div>

    <!-- Modals -->
    <EntryModal
      :open="modalOpen"
      :entry="editing"
      :buildings="buildings"
      :floors="allFloors"
      @close="modalOpen = false"
      @save="save"
    />

    <ImportModal
      :open="importModalOpen"
      @close="importModalOpen = false"
      @import="handleImport"
    />

    <!-- Dedicated Relocate Modal -->
    <RelocateModal
      :open="relocateModalOpen"
      :entries="relocatingEntries"
      :buildings="buildings"
      :floors="allFloors"
      @close="relocateModalOpen = false"
      @relocate="handleRelocate"
    />

    <!-- Admin Login Modal -->
    <AdminLoginModal
      :open="adminLoginModalOpen"
      @close="adminLoginModalOpen = false"
      @login-success="handleLoginSuccess"
    />

    <!-- Location Settings Modal -->
    <LocationSettingsModal
      :open="locationSettingsModalOpen"
      :buildings="buildings"
      :entries="entries"
      :meta="meta"
      @close="locationSettingsModalOpen = false"
      @updated="handleLocationsUpdated"
    />

    <!-- Toast Notification -->
    <div class="toast-container">
      <div
        v-if="toast.show"
        class="toast"
        :class="toast.type === 'success' ? 'toast-success' : 'toast-error'"
      >
        <span v-if="toast.type === 'success'">✅</span>
        <span v-else>⚠️</span>
        <span>{{ toast.msg }}</span>
      </div>
    </div>
  </div>
</template>
