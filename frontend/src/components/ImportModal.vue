<script setup lang="ts">
import { ref } from 'vue'
import { parseExcelFile, downloadExcelTemplate, type ParsedImportResult } from '../services/excel'
import type { ImportPayload } from '../types'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
  import: [ImportPayload]
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const fileName = ref('')
const isDragging = ref(false)
const isParsing = ref(false)
const parseError = ref('')
const parsedResult = ref<ParsedImportResult | null>(null)
const importMode = ref<'append' | 'replace'>('append')
const isSubmitting = ref(false)

function resetState() {
  fileName.value = ''
  parseError.value = ''
  parsedResult.value = null
  importMode.value = 'append'
  isSubmitting.value = false
  if (fileInput.value) fileInput.value.value = ''
}

function handleClose() {
  resetState()
  emit('close')
}

async function processFile(file: File) {
  if (!file) return
  fileName.value = file.name
  isParsing.value = true
  parseError.value = ''
  parsedResult.value = null

  try {
    const res = await parseExcelFile(file)
    parsedResult.value = res
  } catch (err) {
    parseError.value = err instanceof Error ? err.message : String(err)
  } finally {
    isParsing.value = false
  }
}

function onFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files[0]) {
    processFile(target.files[0])
  }
}

function onDrop(e: DragEvent) {
  isDragging.value = false
  if (e.dataTransfer?.files && e.dataTransfer.files[0]) {
    processFile(e.dataTransfer.files[0])
  }
}

function doImport() {
  if (!parsedResult.value || !parsedResult.value.validRows) return
  isSubmitting.value = true
  emit('import', {
    mode: importMode.value,
    items: parsedResult.value.entries,
  })
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="handleClose">
    <div class="modal modal-lg" role="dialog" aria-modal="true">
      <div class="modal-header">
        <div class="modal-title-group">
          <div class="modal-badge-icon excel-icon">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
              <polyline points="14 2 14 8 20 8"></polyline>
              <line x1="8" y1="13" x2="16" y2="13"></line>
              <line x1="8" y1="17" x2="16" y2="17"></line>
              <line x1="10" y1="9" x2="8" y2="9"></line>
            </svg>
          </div>
          <div>
            <h3>นำเข้าข้อมูลจาก Excel / CSV</h3>
            <p class="modal-sub">รองรับไฟล์นามสกุล .xlsx, .xls และ .csv</p>
          </div>
        </div>
        <button type="button" class="btn-close" aria-label="Close" @click="handleClose">✕</button>
      </div>

      <div class="modal-body">
        <!-- Template Download Banner -->
        <div class="template-box">
          <div class="template-info">
            <strong>ต้องการรูปแบบไฟล์ตัวอย่าง?</strong>
            <span>ดาวน์โหลดเทมเพลต Excel สำหรับกรอกข้อมูลเบอร์โทรศัพท์</span>
          </div>
          <button type="button" class="btn btn-outline" @click="downloadExcelTemplate">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="7 10 12 15 17 10"/>
              <line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
            <span>ดาวน์โหลด Template (.xlsx)</span>
          </button>
        </div>

        <!-- Drag & Drop Zone -->
        <div
          class="dropzone"
          :class="{ 'is-dragover': isDragging, 'has-file': !!parsedResult }"
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="onDrop"
          @click="fileInput?.click()"
        >
          <input
            ref="fileInput"
            type="file"
            accept=".xlsx, .xls, .csv"
            style="display: none"
            @change="onFileChange"
          />

          <div v-if="isParsing" class="dropzone-center">
            <div class="spinner"></div>
            <p>กำลังอ่านไฟล์ข้อมูล...</p>
          </div>

          <div v-else-if="parsedResult" class="dropzone-center file-loaded">
            <div class="file-icon-success">📊</div>
            <h4>{{ fileName }}</h4>
            <p class="text-success">อ่านข้อมูลสำเร็จ พบทั้งหมด {{ parsedResult.totalRows }} แถว (ใช้ได้ {{ parsedResult.validRows }} รายการ)</p>
            <span class="btn-link">คลิกเพื่อเลือกไฟล์ใหม่</span>
          </div>

          <div v-else class="dropzone-center">
            <div class="drop-icon">
              <svg viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="17 8 12 3 7 8"/>
                <line x1="12" y1="3" x2="12" y2="15"/>
              </svg>
            </div>
            <h4>ลากไฟล์ Excel มาวางที่นี่ หรือคลิกเพื่อเลือกไฟล์</h4>
            <p>รองรับไฟล์ Excel (.xlsx, .xls) หรือ CSV</p>
          </div>
        </div>

        <div v-if="parseError" class="alert-box alert-danger">
          <strong>เกิดข้อผิดพลาดในการอ่านไฟล์:</strong>
          <p>{{ parseError }}</p>
        </div>

        <!-- Parsed Results & Settings -->
        <template v-if="parsedResult && parsedResult.validRows > 0">
          <div class="import-mode-selection">
            <label class="mode-title">รูปแบบการนำเข้าข้อมูล:</label>
            <div class="mode-options">
              <label class="mode-card" :class="{ selected: importMode === 'append' }">
                <input type="radio" v-model="importMode" value="append" />
                <div class="mode-content">
                  <div class="mode-head">
                    <span class="mode-badge badge-blue">เพิ่มต่อท้าย (Append)</span>
                  </div>
                  <p>นำข้อมูลใหม่เข้าไปเพิ่มในระบบ โดยยังคงเก็บข้อมูลเดิมที่มีอยู่ไว้</p>
                </div>
              </label>

              <label class="mode-card" :class="{ selected: importMode === 'replace' }">
                <input type="radio" v-model="importMode" value="replace" />
                <div class="mode-content">
                  <div class="mode-head">
                    <span class="mode-badge badge-red">เขียนทับทั้งหมด (Replace)</span>
                  </div>
                  <p>ล้างข้อมูลเดิมในระบบทั้งหมด แล้วแทนที่ด้วยข้อมูลจากไฟล์นี้ (เหมาะกับการ Re-seed)</p>
                </div>
              </label>
            </div>
          </div>

          <!-- Preview Table -->
          <div class="preview-section">
            <div class="preview-head">
              <strong>ตัวอย่างข้อมูลที่จะนำเข้า (5 รายการแรก)</strong>
              <span>รวม {{ parsedResult.validRows }} รายการ</span>
            </div>
            <div class="table-responsive">
              <table class="preview-table">
                <thead>
                  <tr>
                    <th>ตึก / อาคาร</th>
                    <th>ชั้น</th>
                    <th>หน่วยงาน</th>
                    <th>เบอร์ภายใน</th>
                    <th>เบอร์สายนอก</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(item, idx) in parsedResult.sample" :key="idx">
                    <td>{{ item.building }}</td>
                    <td><span class="floor-chip">{{ item.floor }}</span></td>
                    <td class="font-medium">{{ item.department }}</td>
                    <td class="text-phone">{{ item.internal_phone || '-' }}</td>
                    <td>{{ item.external_phone || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Warning for invalid rows if any -->
          <div v-if="parsedResult.invalidRows > 0" class="alert-box alert-warning">
            <strong>พบแถวที่ไม่สมบูรณ์ {{ parsedResult.invalidRows }} แถว (จะถูกข้ามอัตโนมัติ):</strong>
            <ul>
              <li v-for="(err, idx) in parsedResult.errors" :key="idx">{{ err }}</li>
            </ul>
          </div>
        </template>
      </div>

      <div class="modal-footer">
        <button type="button" class="btn btn-light" @click="handleClose">ยกเลิก</button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!parsedResult || parsedResult.validRows === 0 || isSubmitting"
          @click="doImport"
        >
          <span v-if="isSubmitting" class="spinner-sm"></span>
          <span v-else>ยืนยันการนำเข้า ({{ parsedResult?.validRows || 0 }} รายการ)</span>
        </button>
      </div>
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
</style>
