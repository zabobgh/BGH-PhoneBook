<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Entry } from '../types'

const props = defineProps<{
  open: boolean
  entries: Entry[]
  buildings: string[]
  floors: string[]
}>()

const emit = defineEmits<{
  close: []
  relocate: [ids: number[], building: string, floor: string]
}>()

const targetBuilding = ref('')
const customBuilding = ref('')
const isCustomBuilding = ref(false)

const targetFloor = ref('')
const customFloor = ref('')
const isCustomFloor = ref(false)

const isSubmitting = ref(false)

// Common hospital floor list
const defaultFloors = [
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
]

const availableFloors = computed(() => {
  const set = new Set([...defaultFloors, ...props.floors])
  return Array.from(set).sort((a, b) => {
    const val = (s: string) => (s.includes('G') || s.includes('g') ? 0 : Number((s.match(/\d+/) || ['999'])[0]) + 1)
    return val(a) - val(b) || a.localeCompare(b, 'th')
  })
})

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) return
    isSubmitting.value = false
    isCustomBuilding.value = false
    isCustomFloor.value = false
    customBuilding.value = ''
    customFloor.value = ''

    if (props.entries.length === 1) {
      targetBuilding.value = props.entries[0].building
      targetFloor.value = props.entries[0].floor
    } else {
      targetBuilding.value = props.buildings[0] || ''
      targetFloor.value = 'ชั้น 1'
    }
  },
  { immediate: true }
)

const finalBuilding = computed(() => {
  return isCustomBuilding.value ? customBuilding.value.trim() : targetBuilding.value.trim()
})

const finalFloor = computed(() => {
  return isCustomFloor.value ? customFloor.value.trim() : targetFloor.value.trim()
})

const isValid = computed(() => {
  return !!finalBuilding.value && !!finalFloor.value && props.entries.length > 0
})

function submit() {
  if (!isValid.value) return
  isSubmitting.value = true
  const ids = props.entries.map((e) => e.id)
  emit('relocate', ids, finalBuilding.value, finalFloor.value)
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal modal-md" role="dialog" aria-modal="true">
      <!-- Modal Header -->
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-badge-icon relocate-icon">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 2a8 8 0 0 0-8 8c0 5.25 8 12 8 12s8-6.75 8-12a8 8 0 0 0-8-8z"/>
              <circle cx="12" cy="10" r="3"/>
            </svg>
          </div>
          <div>
            <h3>{{ entries.length > 1 ? `ย้ายสถานที่ (${entries.length} หน่วยงาน)` : 'ย้ายสถานที่หน่วยงาน' }}</h3>
            <p class="modal-sub">เปลี่ยนตึกหรือชั้นที่ตั้งของหน่วยงานในโรงพยาบาล</p>
          </div>
        </div>
        <button type="button" class="btn-close" aria-label="Close" @click="$emit('close')">✕</button>
      </div>

      <!-- Modal Body -->
      <form @submit.prevent="submit">
        <div class="modal-body">
          <!-- Department Information Summary -->
          <div class="relocate-info-box">
            <div v-if="entries.length === 1" class="single-entry-info">
              <div class="dept-title">{{ entries[0].department }}</div>
              <div class="current-loc">
                <span class="loc-label">สถานที่เดิม:</span>
                <span class="loc-pill">🏢 {{ entries[0].building }}</span>
                <span class="loc-pill">📍 {{ entries[0].floor }}</span>
                <span v-if="entries[0].internal_phone" class="loc-phone">📞 {{ entries[0].internal_phone }}</span>
              </div>
            </div>

            <div v-else class="multi-entry-info">
              <div class="multi-head">
                <strong>หน่วยงานที่เลือกย้าย ({{ entries.length }} รายการ):</strong>
              </div>
              <div class="multi-chips-list">
                <span v-for="e in entries.slice(0, 8)" :key="e.id" class="multi-dept-chip">
                  {{ e.department }}
                </span>
                <span v-if="entries.length > 8" class="multi-dept-chip more">
                  + อีก {{ entries.length - 8 }} หน่วยงาน
                </span>
              </div>
            </div>
          </div>

          <!-- Target Building & Floor Fields -->
          <div class="relocate-inputs">
            <!-- Target Building -->
            <div class="form-group">
              <div class="label-row">
                <label>ตึก / อาคารปลายทาง <span class="req">*</span></label>
                <button
                  type="button"
                  class="btn-text-toggle"
                  @click="isCustomBuilding = !isCustomBuilding"
                >
                  {{ isCustomBuilding ? '← เลือกจากรายการเดิม' : '+ พิมพ์ชื่อตึกใหม่' }}
                </button>
              </div>

              <input
                v-if="isCustomBuilding"
                v-model="customBuilding"
                placeholder="พิมพ์ชื่อตึกใหม่..."
                required
                autofocus
              />
              <select v-else v-model="targetBuilding" required>
                <option v-for="b in buildings" :key="b" :value="b">{{ b }}</option>
              </select>
            </div>

            <!-- Target Floor -->
            <div class="form-group">
              <div class="label-row">
                <label>ชั้นปลายทาง <span class="req">*</span></label>
                <button
                  type="button"
                  class="btn-text-toggle"
                  @click="isCustomFloor = !isCustomFloor"
                >
                  {{ isCustomFloor ? '← เลือกจากรายการเดิม' : '+ พิมพ์ชั้นใหม่' }}
                </button>
              </div>

              <input
                v-if="isCustomFloor"
                v-model="customFloor"
                placeholder="เช่น ชั้น 11, ชั้น B1..."
                required
              />
              <select v-else v-model="targetFloor" required>
                <option v-for="f in availableFloors" :key="f" :value="f">{{ f }}</option>
              </select>
            </div>
          </div>

          <!-- Preview Transition Box -->
          <div v-if="isValid" class="relocate-preview-box">
            <div class="preview-title">สรุปสถานที่ใหม่หลังการย้าย:</div>
            <div class="preview-route">
              <div class="preview-dest">
                <span class="dest-badge">ตึกใหม่</span>
                <strong>{{ finalBuilding }}</strong>
              </div>
              <div class="arrow-divider">➔</div>
              <div class="preview-dest">
                <span class="dest-badge">ชั้นใหม่</span>
                <strong>{{ finalFloor }}</strong>
              </div>
            </div>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="modal-footer">
          <button type="button" class="btn btn-light" @click="$emit('close')">ยกเลิก</button>
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="!isValid || isSubmitting"
          >
            <span v-if="isSubmitting" class="spinner-sm"></span>
            <span v-else>
              📍 บันทึกการย้ายสถานที่ ({{ entries.length }} รายการ)
            </span>
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.modal {
  background: var(--paper-card);
  border-radius: var(--radius-xl);
  overflow: hidden;
  box-shadow: 0 25px 60px -12px rgba(0, 0, 0, 0.35);
}

.modal-body {
  background: var(--paper-card);
}

.relocate-icon {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
}

.relocate-info-box {
  background: var(--paper-subtle);
  border: 1px solid var(--rule);
  border-radius: var(--radius-md);
  padding: 14px 18px;
}

.single-entry-info .dept-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--ink);
}

.current-loc {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  flex-wrap: wrap;
}

.loc-label {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  color: var(--mute);
  text-transform: uppercase;
}

.loc-pill {
  background: var(--paper-card);
  border: 1px solid var(--rule);
  padding: 2px 8px;
  border-radius: var(--radius-xs);
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
  color: var(--ink);
}

.loc-phone {
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 700;
  color: var(--ok);
}

.multi-head {
  font-size: 13px;
  color: var(--ink);
  margin-bottom: 8px;
}

.multi-chips-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.multi-dept-chip {
  background: var(--paper-card);
  border: 1px solid var(--rule);
  padding: 3px 8px;
  border-radius: var(--radius-xs);
  font-size: 12px;
  font-weight: 600;
  color: var(--ink);
}

.multi-dept-chip.more {
  background: var(--signal-light);
  color: var(--signal);
  border-color: var(--signal-border);
}

.relocate-inputs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.label-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.btn-text-toggle {
  background: none;
  border: none;
  color: var(--signal);
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
}

.btn-text-toggle:hover {
  text-decoration: underline;
}

.relocate-preview-box {
  background: var(--paper-card);
  border: 1px dashed var(--signal);
  border-radius: var(--radius-sm);
  padding: 12px 16px;
}

.preview-title {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--mute);
  margin-bottom: 8px;
}

.preview-route {
  display: flex;
  align-items: center;
  gap: 14px;
}

.preview-dest {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dest-badge {
  background: var(--signal-light);
  color: var(--signal);
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: var(--radius-xs);
}

.preview-dest strong {
  font-size: 14px;
  color: var(--ink);
}

.arrow-divider {
  color: var(--signal);
  font-weight: 800;
  font-size: 16px;
}

@media (max-width: 600px) {
  .relocate-inputs {
    grid-template-columns: 1fr;
  }
}
</style>
