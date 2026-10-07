<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { Entry } from '../types'

const props = defineProps<{
  open: boolean
  entry: Entry | null
  buildings: string[]
  floors: string[]
}>()

const emit = defineEmits<{
  close: []
  save: [Entry]
}>()

const form = reactive<Entry>({
  id: 0,
  building: '',
  floor: '',
  department: '',
  internal_phone: '',
  external_phone: '',
  sort_order: 0,
})

watch(
  () => [props.open, props.entry] as const,
  () => {
    if (!props.open) return
    Object.assign(
      form,
      props.entry ?? {
        id: 0,
        building: props.buildings[0] || '',
        floor: 'ชั้น 1',
        department: '',
        internal_phone: '',
        external_phone: '',
        sort_order: 0,
      }
    )
  },
  { immediate: true }
)

function submit() {
  if (!form.building.trim() || !form.floor.trim() || !form.department.trim()) return
  emit('save', { ...form })
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal modal-md" role="dialog" aria-modal="true">
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-badge-icon">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
            </svg>
          </div>
          <div>
            <h3>{{ form.id ? 'แก้ไขข้อมูลหน่วยงาน / ย้ายสถานที่' : 'เพิ่มหน่วยงานใหม่' }}</h3>
            <p class="modal-sub">กำหนดข้อมูลตึก ชั้น ชื่อหน่วยงาน และเบอร์โทร</p>
          </div>
        </div>
        <button type="button" class="btn-close" aria-label="Close" @click="$emit('close')">✕</button>
      </div>

      <form @submit.prevent="submit">
        <div class="modal-body">
          <div v-if="form.id" class="relocate-callout">
            <span class="callout-icon">📍</span>
            <span>หากหน่วยงานมีการ<b>ย้ายสถานที่</b> สามารถเปลี่ยนตึกหรือชั้นด้านล่างนี้ได้ทันที ข้อมูลจะถูกย้ายไปยังหมวดหมู่ใหม่อัตโนมัติ</span>
          </div>

          <div class="form-row form-grid-2">
            <div class="form-group">
              <label>ตึก / อาคาร <span class="req">*</span></label>
              <input
                v-model="form.building"
                list="building-list"
                placeholder="เช่น ตึกเฉลิมพระเกียรติ"
                required
                autocomplete="off"
              />
              <datalist id="building-list">
                <option v-for="b in buildings" :key="b" :value="b" />
              </datalist>
            </div>
            <div class="form-group">
              <label>ชั้น <span class="req">*</span></label>
              <input
                v-model="form.floor"
                list="floor-list"
                placeholder="เช่น ชั้น 1, ชั้น G"
                required
                autocomplete="off"
              />
              <datalist id="floor-list">
                <option v-for="f in floors" :key="f" :value="f" />
              </datalist>
            </div>
          </div>

          <div class="form-group">
            <label>ชื่อหน่วยงาน / แผนก <span class="req">*</span></label>
            <input
              v-model="form.department"
              placeholder="เช่น จุดประชาสัมพันธ์, OPD อายุรกรรม, ห้องยา"
              required
              autofocus
            />
          </div>

          <div class="form-row form-grid-2">
            <div class="form-group">
              <label>เบอร์ภายใน</label>
              <div class="input-icon-wrap">
                <span class="input-icon">📞</span>
                <input
                  v-model="form.internal_phone"
                  placeholder="เช่น 1001, 1002 หรือ 9508"
                />
              </div>
              <small class="hint">กรณีมีหลายเบอร์ ให้คั่นด้วยเครื่องหมายจุลภาค (,)</small>
            </div>
            <div class="form-group">
              <label>เบอร์สายนอก (ถ้ามี)</label>
              <div class="input-icon-wrap">
                <span class="input-icon">🌐</span>
                <input
                  v-model="form.external_phone"
                  placeholder="เช่น 034-419555"
                />
              </div>
            </div>
          </div>

          <div class="form-group">
            <label>ลำดับการแสดงผล (Sort Order)</label>
            <input
              v-model.number="form.sort_order"
              type="number"
              placeholder="0 (แสดงผลตามลำดับอัตโนมัติ)"
            />
          </div>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-light" @click="$emit('close')">ยกเลิก</button>
          <button type="submit" class="btn btn-primary">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
              <polyline points="17 21 17 13 7 13 7 21"/>
              <polyline points="7 3 7 8 15 8"/>
            </svg>
            <span>บันทึกข้อมูล</span>
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

.relocate-callout {
  background: var(--paper-subtle);
  border: 1px solid var(--rule);
  border-radius: var(--radius-md);
  padding: 12px 16px;
  font-size: 13px;
  color: var(--ink);
  display: flex;
  align-items: flex-start;
  gap: 10px;
  line-height: 1.45;
}

.callout-icon {
  font-size: 16px;
  flex-shrink: 0;
  margin-top: 1px;
}
</style>
