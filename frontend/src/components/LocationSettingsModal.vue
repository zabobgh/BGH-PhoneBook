<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { LocationItem, Entry, BuildingMeta } from '../types'
import { api } from '../services/api'

const props = defineProps<{
  open: boolean
  buildings: string[]
  entries: Entry[]
  meta?: BuildingMeta[]
}>()

const emit = defineEmits<{
  close: []
  updated: [oldBuilding?: string, newBuilding?: string]
}>()

const locations = ref<LocationItem[]>([])
const loading = ref(false)
const selectedBuilding = ref('')
const newBuildingInput = ref('')
const isAddingNewBuilding = ref(false)

// Rename Building State
const isRenamingBuilding = ref(false)
const renameBuildingInput = ref('')

// Building entries cache for accurate floor count and checks
const buildingEntries = ref<Entry[]>([])
const loadingEntries = ref(false)

const newFloorInput = ref('')
const isSubmitting = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

// Predefined floor suggestions
const commonFloorSuggestions = [
  'ชั้น B2',
  'ชั้น B1',
  'ชั้นใต้ดิน',
  'ชั้น G',
  'ชั้น 1',
  'ชั้น 2',
  'ชั้น 3',
  'ชั้น 4',
  'ชั้น 5',
  'ชั้น 6',
  'ชั้น 7',
  'ชั้น 8',
  'ชั้น 9',
  'ชั้น 10',
  'ชั้น 11',
  'ชั้น 12',
  'ชั้น 13',
  'ชั้น 14',
  'ชั้น 15',
  'ชั้นดาดฟ้า',
  'ไม่ระบุชั้น',
]

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

// Department count per building
const buildingEntryCounts = computed(() => {
  const counts: Record<string, number> = {}
  if (props.meta) {
    for (const m of props.meta) {
      counts[m.building] = m.count
    }
  }
  for (const e of props.entries) {
    if (e.building && counts[e.building] === undefined) {
      counts[e.building] = (counts[e.building] || 0) + 1
    }
  }
  return counts
})

// All unique buildings (from locations table + meta + entries + props)
const allBuildingList = computed(() => {
  const set = new Set<string>([...props.buildings])
  if (props.meta) {
    for (const m of props.meta) {
      if (m.building) set.add(m.building)
    }
  }
  for (const loc of locations.value) {
    if (loc.building) set.add(loc.building)
  }
  for (const e of props.entries) {
    if (e.building) set.add(e.building)
  }
  return Array.from(set).filter(Boolean)
})

// Current floors for the selected building
const currentFloors = computed(() => {
  if (!selectedBuilding.value) return []
  const set = new Set<string>()
  for (const loc of locations.value) {
    if (loc.building === selectedBuilding.value && loc.floor) {
      set.add(loc.floor)
    }
  }
  for (const e of buildingEntries.value) {
    if (e.building === selectedBuilding.value && e.floor) {
      set.add(e.floor)
    }
  }
  for (const e of props.entries) {
    if (e.building === selectedBuilding.value && e.floor) {
      set.add(e.floor)
    }
  }
  return Array.from(set).sort(floorSort)
})

// Department count per floor in selected building
const floorEntryCounts = computed(() => {
  const counts: Record<string, number> = {}
  if (!selectedBuilding.value) return counts
  const source = buildingEntries.value.length > 0 ? buildingEntries.value : props.entries
  for (const e of source) {
    if (e.building === selectedBuilding.value && e.floor) {
      counts[e.floor] = (counts[e.floor] || 0) + 1
    }
  }
  return counts
})

// Available suggestions that haven't been added to this building yet
const availableSuggestions = computed(() => {
  const currentSet = new Set(currentFloors.value)
  return commonFloorSuggestions.filter((f) => !currentSet.has(f))
})

async function fetchLocations() {
  loading.value = true
  errorMsg.value = ''
  try {
    locations.value = await api.getLocations()
  } catch (err) {
    console.error('Failed to load locations', err)
  } finally {
    loading.value = false
  }
}

async function loadBuildingEntries(b: string) {
  if (!b) {
    buildingEntries.value = []
    return
  }
  loadingEntries.value = true
  try {
    buildingEntries.value = await api.list('', b)
  } catch (err) {
    console.warn('Failed to load building entries', err)
  } finally {
    loadingEntries.value = false
  }
}

watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen) {
      errorMsg.value = ''
      successMsg.value = ''
      newFloorInput.value = ''
      newBuildingInput.value = ''
      isAddingNewBuilding.value = false
      isRenamingBuilding.value = false
      renameBuildingInput.value = ''
      await fetchLocations()
      if (!selectedBuilding.value && allBuildingList.value.length > 0) {
        selectedBuilding.value = allBuildingList.value[0]
      }
      if (selectedBuilding.value) {
        await loadBuildingEntries(selectedBuilding.value)
      }
    }
  },
  { immediate: true }
)

watch(
  () => selectedBuilding.value,
  async (newB) => {
    isRenamingBuilding.value = false
    renameBuildingInput.value = ''
    if (newB) {
      await loadBuildingEntries(newB)
    } else {
      buildingEntries.value = []
    }
  }
)

function selectBuilding(b: string) {
  selectedBuilding.value = b
  isAddingNewBuilding.value = false
  newFloorInput.value = ''
  errorMsg.value = ''
  successMsg.value = ''
}

function startAddNewBuilding() {
  isAddingNewBuilding.value = true
  isRenamingBuilding.value = false
  newBuildingInput.value = ''
  newFloorInput.value = 'ชั้น 1'
}

function startRenameBuilding() {
  if (!selectedBuilding.value) return
  renameBuildingInput.value = selectedBuilding.value
  isRenamingBuilding.value = true
  errorMsg.value = ''
  successMsg.value = ''
}

function cancelRenameBuilding() {
  isRenamingBuilding.value = false
  renameBuildingInput.value = ''
}

async function handleRenameBuilding() {
  const oldName = selectedBuilding.value.trim()
  const newName = renameBuildingInput.value.trim()

  if (!newName) {
    errorMsg.value = 'กรุณาระบุชื่ออาคารใหม่'
    return
  }
  if (oldName === newName) {
    isRenamingBuilding.value = false
    return
  }

  isSubmitting.value = true
  errorMsg.value = ''
  successMsg.value = ''

  try {
    const res = await api.renameBuilding(oldName, newName)
    successMsg.value = `เปลี่ยนชื่ออาคารจาก "${oldName}" เป็น "${newName}" สำเร็จ (อัปเดต ${res.updatedEntries} หน่วยงาน)`
    isRenamingBuilding.value = false
    selectedBuilding.value = newName
    await fetchLocations()
    await loadBuildingEntries(newName)
    emit('updated', oldName, newName)
  } catch (err: any) {
    errorMsg.value = err.message || 'ไม่สามารถเปลี่ยนชื่ออาคารได้'
  } finally {
    isSubmitting.value = false
  }
}

async function handleDeleteBuilding() {
  const b = selectedBuilding.value.trim()
  if (!b) return
  const count = buildingEntryCounts.value[b] || 0
  if (count > 0) {
    errorMsg.value = `ไม่สามารถลบอาคาร "${b}" ได้ เนื่องจากมีหน่วยงานใช้งานอยู่ ${count} แห่ง (กรุณาย้ายหน่วยงานออกก่อน)`
    return
  }

  if (!confirm(`ยืนยันการลบอาคาร "${b}" และชั้นทั้งหมดของอาคารนี้ ใช่หรือไม่?`)) {
    return
  }

  isSubmitting.value = true
  errorMsg.value = ''
  successMsg.value = ''

  try {
    await api.deleteBuilding(b)
    successMsg.value = `ลบอาคาร "${b}" สำเร็จแล้ว`
    await fetchLocations()
    emit('updated', b, '')
    const remaining = allBuildingList.value.filter((x) => x !== b)
    if (remaining.length > 0) {
      selectBuilding(remaining[0])
    } else {
      selectedBuilding.value = ''
    }
  } catch (err: any) {
    errorMsg.value = err.message || 'ไม่สามารถลบอาคารได้'
  } finally {
    isSubmitting.value = false
  }
}

async function handleAddFloor(floorToAdd?: string) {
  const floorName = (floorToAdd || newFloorInput.value).trim()
  const buildingName = isAddingNewBuilding.value
    ? newBuildingInput.value.trim()
    : selectedBuilding.value.trim()

  if (!buildingName) {
    errorMsg.value = 'กรุณาระบุชื่ออาคาร'
    return
  }
  if (!floorName) {
    errorMsg.value = 'กรุณาระบุชื่อชั้น'
    return
  }

  isSubmitting.value = true
  errorMsg.value = ''
  successMsg.value = ''

  try {
    await api.addLocation(buildingName, floorName)
    successMsg.value = `เพิ่ม "${floorName}" ให้ "${buildingName}" สำเร็จแล้ว`
    newFloorInput.value = ''
    if (isAddingNewBuilding.value) {
      selectedBuilding.value = buildingName
      isAddingNewBuilding.value = false
      newBuildingInput.value = ''
    }
    await fetchLocations()
    await loadBuildingEntries(selectedBuilding.value)
    emit('updated')
  } catch (err: any) {
    errorMsg.value = err.message || 'ไม่สามารถเพิ่มชั้นได้'
  } finally {
    isSubmitting.value = false
  }
}

async function handleDeleteFloor(floorName: string) {
  const count = floorEntryCounts.value[floorName] || 0
  if (count > 0) {
    errorMsg.value = `ไม่สามารถลบ "${floorName}" ได้ เนื่องจากมีหน่วยงานใช้งานอยู่ ${count} แห่ง (กรุณาย้ายหน่วยงานออกก่อน)`
    return
  }

  if (!confirm(`ยืนยันการลบ "${floorName}" ออกจาก "${selectedBuilding.value}" ใช่หรือไม่?`)) {
    return
  }

  isSubmitting.value = true
  errorMsg.value = ''
  successMsg.value = ''

  try {
    await api.deleteLocation(selectedBuilding.value, floorName)
    successMsg.value = `ลบ "${floorName}" สำเร็จแล้ว`
    await fetchLocations()
    await loadBuildingEntries(selectedBuilding.value)
    emit('updated')
  } catch (err: any) {
    errorMsg.value = err.message || 'ไม่สามารถลบชั้นได้'
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal modal-lg location-modal" role="dialog" aria-modal="true">
      <!-- Modal Header -->
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-badge-icon location-icon">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 21h18"/>
              <path d="M5 21V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16"/>
              <path d="M9 9h1"/>
              <path d="M9 13h1"/>
              <path d="M9 17h1"/>
              <path d="M14 9h1"/>
              <path d="M14 13h1"/>
              <path d="M14 17h1"/>
            </svg>
          </div>
          <div>
            <h3>⚙️ จัดการสถานที่ (อาคารและชั้น)</h3>
            <p class="modal-sub">เพิ่มชั้นใหม่ หรือเพิ่มอาคารสำหรับระบบค้นหาและจัดเก็บสมุดโทรศัพท์</p>
          </div>
        </div>
        <button type="button" class="btn-close" aria-label="Close" @click="$emit('close')">✕</button>
      </div>

      <div class="modal-body loc-modal-body">
        <!-- Messages -->
        <div v-if="successMsg" class="alert-box alert-success mb-3">
          ✓ {{ successMsg }}
        </div>
        <div v-if="errorMsg" class="alert-box alert-danger mb-3">
          ⚠️ {{ errorMsg }}
        </div>

        <div class="loc-split-layout">
          <!-- Left: Buildings List -->
          <div class="loc-sidebar">
            <div class="loc-sidebar-header">
              <span class="loc-col-title">🏢 รายชื่ออาคาร ({{ allBuildingList.length }})</span>
              <button
                type="button"
                class="btn-sm btn-signal-subtle"
                @click="startAddNewBuilding"
                title="สร้างอาคารใหม่"
              >
                + เพิ่มอาคาร
              </button>
            </div>

            <!-- Building Selector Buttons -->
            <div class="loc-building-list">
              <button
                v-if="isAddingNewBuilding"
                type="button"
                class="loc-b-item active-new"
              >
                <span>➕ อาคารใหม่</span>
              </button>

              <button
                v-for="b in allBuildingList"
                :key="b"
                type="button"
                class="loc-b-item"
                :class="{ active: selectedBuilding === b && !isAddingNewBuilding }"
                @click="selectBuilding(b)"
              >
                <span class="loc-b-name">{{ b }}</span>
                <span class="loc-b-badge">
                  {{ buildingEntryCounts[b] || 0 }} หน่วยงาน
                </span>
              </button>
            </div>
          </div>

          <!-- Right: Floors for selected building -->
          <div class="loc-content">
            <!-- Header of content pane -->
            <div class="loc-content-header">
              <div v-if="isAddingNewBuilding" class="new-building-form">
                <label>ชื่ออาคารใหม่ที่ต้องการเพิ่ม <span class="req">*</span></label>
                <div class="flex-row gap-2">
                  <input
                    v-model="newBuildingInput"
                    placeholder="เช่น อาคารศูนย์ความเป็นเลิศทางการแพทย์"
                    class="input-building"
                    autofocus
                  />
                </div>
              </div>
              <div v-else class="selected-b-info">
                <!-- Normal view: Title + Action buttons -->
                <div v-if="!isRenamingBuilding" class="b-info-title-row">
                  <div class="b-info-title-group">
                    <h4>🏢 {{ selectedBuilding }}</h4>
                    <p class="text-muted">
                      มีทั้งหมด {{ currentFloors.length }} ชั้น | {{ buildingEntryCounts[selectedBuilding] || 0 }} หน่วยงาน
                    </p>
                  </div>
                  <div class="b-header-actions">
                    <button
                      type="button"
                      class="btn btn-outline btn-sm btn-rename"
                      title="เปลี่ยนชื่ออาคารนี้"
                      @click="startRenameBuilding"
                    >
                      ✏️ เปลี่ยนชื่ออาคาร
                    </button>
                    <button
                      v-if="(buildingEntryCounts[selectedBuilding] || 0) === 0"
                      type="button"
                      class="btn btn-outline btn-sm btn-delete-bldg"
                      title="ลบอาคารที่ไม่มีหน่วยงาน"
                      @click="handleDeleteBuilding"
                    >
                      🗑️ ลบอาคาร
                    </button>
                  </div>
                </div>

                <!-- Renaming view: Inline edit form -->
                <div v-else class="rename-building-card">
                  <div class="rename-label">
                    <span>✏️ <strong>เปลี่ยนชื่ออาคาร</strong></span>
                    <span class="text-muted"> (ชื่อเดิม: {{ selectedBuilding }})</span>
                  </div>
                  <div class="rename-input-row">
                    <input
                      v-model="renameBuildingInput"
                      class="input-building"
                      placeholder="ระบุชื่ออาคารใหม่"
                      @keydown.enter.prevent="handleRenameBuilding"
                      @keydown.esc="cancelRenameBuilding"
                      autofocus
                    />
                    <button
                      type="button"
                      class="btn btn-signal btn-sm"
                      :disabled="isSubmitting || !renameBuildingInput.trim() || renameBuildingInput.trim() === selectedBuilding"
                      @click="handleRenameBuilding"
                    >
                      <span v-if="isSubmitting">กำลังบันทึก...</span>
                      <span v-else>บันทึกชื่อใหม่</span>
                    </button>
                    <button
                      type="button"
                      class="btn btn-outline btn-sm"
                      :disabled="isSubmitting"
                      @click="cancelRenameBuilding"
                    >
                      ยกเลิก
                    </button>
                  </div>
                  <div class="rename-help-note">
                    ℹ️ เมื่อเปลี่ยนชื่อ ระบบจะอัปเดตชื่ออาคารให้กับทุกหน่วยงาน ({{ buildingEntryCounts[selectedBuilding] || 0 }} แห่ง) และทุกชั้นในระบบให้ทันที
                  </div>
                </div>
              </div>
            </div>

            <!-- Add Floor Section -->
            <div class="add-floor-box">
              <label class="form-label">
                <strong>+ เพิ่มชั้นในอาคารนี้</strong>
                <span class="sub-label"> (พิมพ์ชื่อชั้น หรือกดเลือกจากชั้นแนะนำด้านล่าง)</span>
              </label>
              
              <div class="add-floor-input-row">
                <input
                  v-model="newFloorInput"
                  placeholder="เช่น ชั้น 11, ชั้น 12, ชั้น B1"
                  class="input-floor"
                  @keydown.enter.prevent="handleAddFloor()"
                />
                <button
                  type="button"
                  class="btn btn-signal"
                  :disabled="isSubmitting || !newFloorInput.trim()"
                  @click="handleAddFloor()"
                >
                  <span v-if="isSubmitting">กำลังบันทึก...</span>
                  <span v-else>+ เพิ่มชั้นนี้</span>
                </button>
              </div>

              <!-- Quick suggestion chips -->
              <div v-if="availableSuggestions.length > 0" class="suggestions-section">
                <span class="sugg-label">ชั้นแนะนำที่ยังไม่มี:</span>
                <div class="sugg-chips-wrap">
                  <button
                    v-for="sugg in availableSuggestions.slice(0, 10)"
                    :key="sugg"
                    type="button"
                    class="sugg-chip"
                    :disabled="isSubmitting"
                    @click="handleAddFloor(sugg)"
                  >
                    + {{ sugg }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Existing Floors List -->
            <div class="existing-floors-container">
              <div class="existing-floors-title">
                <span>ชั้นที่มีในอาคารนี้ ({{ currentFloors.length }})</span>
              </div>

              <div v-if="currentFloors.length === 0" class="empty-floors">
                ยังไม่มีข้อมูลชั้นในอาคารนี้ กรุณาพิมพ์เพิ่มชั้นด้านบน
              </div>

              <div v-else class="floor-items-grid">
                <div
                  v-for="fl in currentFloors"
                  :key="fl"
                  class="floor-card"
                >
                  <div class="floor-card-left">
                    <span class="floor-card-icon">📍</span>
                    <span class="floor-card-name">{{ fl }}</span>
                  </div>

                  <div class="floor-card-right">
                    <span
                      class="floor-count-pill"
                      :class="{ 'has-entries': (floorEntryCounts[fl] || 0) > 0 }"
                    >
                      {{ floorEntryCounts[fl] || 0 }} หน่วยงาน
                    </span>

                    <button
                      type="button"
                      class="btn-del-floor"
                      :disabled="(floorEntryCounts[fl] || 0) > 0"
                      :title="(floorEntryCounts[fl] || 0) > 0 ? 'มีหน่วยงานอยู่ ลบไม่ได้' : 'ลบชั้นนี้'"
                      @click="handleDeleteFloor(fl)"
                    >
                      🗑️
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <button type="button" class="btn btn-light" @click="$emit('close')">
          ปิดหน้าต่าง
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.location-modal {
  max-width: 860px;
  width: 95vw;
}

.location-icon {
  background: rgba(37, 99, 235, 0.12);
  color: var(--brand-blue, #2563eb);
}

.loc-modal-body {
  padding: 1.25rem 1.5rem;
}

.loc-split-layout {
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 1.5rem;
  min-height: 440px;
}

@media (max-width: 768px) {
  .loc-split-layout {
    grid-template-columns: 1fr;
  }
}

.loc-sidebar {
  border-right: 1px solid var(--border-color, #e2e8f0);
  padding-right: 1.25rem;
  display: flex;
  flex-direction: column;
}

.loc-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}

.loc-col-title {
  font-size: 0.8rem;
  font-weight: 700;
  color: var(--text-muted, #64748b);
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.btn-signal-subtle {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.6rem;
  background: var(--brand-blue-subtle, rgba(37, 99, 235, 0.1));
  color: var(--brand-blue, #2563eb);
  border: 1px solid rgba(37, 99, 235, 0.25);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-signal-subtle:hover {
  background: var(--brand-blue, #2563eb);
  color: #fff;
}

.loc-building-list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  overflow-y: auto;
  max-height: 380px;
  padding-right: 0.25rem;
}

.loc-b-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  text-align: left;
  padding: 0.6rem 0.75rem;
  background: var(--card-bg, #f8fafc);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-color, #1e293b);
  cursor: pointer;
  transition: all 0.15s ease;
}

.loc-b-item:hover {
  border-color: var(--brand-blue, #2563eb);
  background: rgba(37, 99, 235, 0.04);
}

.loc-b-item.active {
  border-color: var(--brand-blue, #2563eb);
  background: rgba(37, 99, 235, 0.1);
  color: var(--brand-blue, #2563eb);
  font-weight: 600;
}

.loc-b-item.active-new {
  border-color: #10b981;
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
  font-weight: 600;
}

.loc-b-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}

.loc-b-badge {
  font-size: 0.7rem;
  background: rgba(0, 0, 0, 0.05);
  padding: 0.15rem 0.4rem;
  border-radius: 4px;
  color: var(--text-muted, #64748b);
}

.loc-content {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.loc-content-header {
  border-bottom: 1px solid var(--border-color, #e2e8f0);
  padding-bottom: 0.75rem;
}

.b-info-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.b-info-title-group h4 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-color, #1e293b);
}

.b-info-title-group p {
  margin: 0.2rem 0 0;
  font-size: 0.8rem;
  color: var(--text-muted, #64748b);
}

.b-header-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.btn-rename {
  font-size: 0.8rem;
  padding: 0.35rem 0.75rem;
  border-radius: 6px;
  cursor: pointer;
  background: var(--paper-2, #f1f5f9);
  border: 1px solid var(--border-color, #cbd5e1);
  color: var(--text-color, #334155);
  font-weight: 600;
  transition: all 0.15s ease;
}

.btn-rename:hover {
  border-color: var(--signal, #f2551d);
  color: var(--signal, #f2551d);
}

.btn-delete-bldg {
  font-size: 0.8rem;
  padding: 0.35rem 0.75rem;
  border-radius: 6px;
  cursor: pointer;
  background: rgba(220, 38, 38, 0.04);
  border: 1px solid rgba(220, 38, 38, 0.25);
  color: var(--danger, #dc2626);
  font-weight: 600;
  transition: all 0.15s ease;
}

.btn-delete-bldg:hover {
  background: rgba(220, 38, 38, 0.1);
  border-color: var(--danger, #dc2626);
}

.rename-building-card {
  background: var(--card-bg, #f8fafc);
  border: 1px solid var(--signal, #f2551d);
  border-radius: 8px;
  padding: 0.75rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.rename-label {
  font-size: 0.85rem;
  color: var(--text-color, #1e293b);
}

.rename-input-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.rename-input-row input {
  flex: 1;
}

.rename-help-note {
  font-size: 0.75rem;
  color: var(--text-muted, #64748b);
  line-height: 1.4;
}

.new-building-form label {
  font-size: 0.85rem;
  font-weight: 600;
  display: block;
  margin-bottom: 0.4rem;
}

.input-building {
  width: 100%;
  padding: 0.55rem 0.8rem;
  border-radius: 6px;
  border: 1px solid var(--border-color, #e2e8f0);
  background: var(--input-bg, #fff);
  color: var(--text-color, #1e293b);
  font-size: 0.95rem;
}

.add-floor-box {
  background: var(--card-bg, #f8fafc);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 10px;
  padding: 1rem;
}

.form-label {
  display: block;
  margin-bottom: 0.5rem;
  font-size: 0.875rem;
}

.sub-label {
  font-size: 0.75rem;
  color: var(--text-muted, #64748b);
  font-weight: normal;
}

.add-floor-input-row {
  display: flex;
  gap: 0.6rem;
  margin-bottom: 0.75rem;
}

.input-floor {
  flex: 1;
  padding: 0.55rem 0.8rem;
  border-radius: 6px;
  border: 1px solid var(--border-color, #e2e8f0);
  background: var(--input-bg, #fff);
  color: var(--text-color, #1e293b);
  font-size: 0.95rem;
}

.suggestions-section {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.sugg-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted, #64748b);
}

.sugg-chips-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.sugg-chip {
  font-size: 0.75rem;
  padding: 0.25rem 0.55rem;
  background: rgba(37, 99, 235, 0.08);
  color: var(--brand-blue, #2563eb);
  border: 1px solid rgba(37, 99, 235, 0.2);
  border-radius: 20px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.sugg-chip:hover {
  background: var(--brand-blue, #2563eb);
  color: #fff;
}

.existing-floors-container {
  flex: 1;
}

.existing-floors-title {
  font-size: 0.8rem;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--text-muted, #64748b);
  margin-bottom: 0.6rem;
}

.empty-floors {
  padding: 1.5rem;
  text-align: center;
  color: var(--text-muted, #64748b);
  font-size: 0.875rem;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 8px;
}

.floor-items-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 0.6rem;
  max-height: 220px;
  overflow-y: auto;
  padding-right: 0.25rem;
}

.floor-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.55rem 0.75rem;
  background: var(--card-bg, #fff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  font-size: 0.875rem;
}

.floor-card-left {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.floor-card-name {
  font-weight: 600;
  color: var(--text-color, #1e293b);
}

.floor-card-right {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.floor-count-pill {
  font-size: 0.7rem;
  padding: 0.15rem 0.45rem;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.06);
  color: var(--text-muted, #64748b);
}

.floor-count-pill.has-entries {
  background: rgba(37, 99, 235, 0.1);
  color: var(--brand-blue, #2563eb);
  font-weight: 600;
}

.btn-del-floor {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0.2rem;
  border-radius: 4px;
  opacity: 0.7;
  transition: opacity 0.15s ease;
}

.btn-del-floor:hover:not(:disabled) {
  opacity: 1;
  background: rgba(239, 68, 68, 0.1);
}

.btn-del-floor:disabled {
  opacity: 0.2;
  cursor: not-allowed;
}

.alert-box {
  padding: 0.6rem 0.9rem;
  border-radius: 6px;
  font-size: 0.85rem;
}

.alert-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #065f46;
}

.alert-danger {
  background: rgba(239, 68, 68, 0.12);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #991b1b;
}

.mb-3 {
  margin-bottom: 0.75rem;
}
</style>
