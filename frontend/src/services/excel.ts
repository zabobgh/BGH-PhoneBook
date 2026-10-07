import * as XLSX from 'xlsx'
import type { Entry } from '../types'

export interface ParsedImportResult {
  entries: Array<Omit<Entry, 'id'>>
  totalRows: number
  validRows: number
  invalidRows: number
  sample: Array<Omit<Entry, 'id'>>
  errors: string[]
}

export function exportToExcel(entries: Entry[], filename = 'BGH-PhoneBook-เบอร์โทรภายใน.xlsx') {
  const data = entries.map((e, index) => ({
    'ลำดับ': e.sort_order || index + 1,
    'ตึก / อาคาร': e.building,
    'ชั้น': e.floor,
    'หน่วยงาน / แผนก': e.department,
    'เบอร์ภายใน': e.internal_phone || '',
    'เบอร์สายนอก': e.external_phone || '',
  }))

  const worksheet = XLSX.utils.json_to_sheet(data)

  // Configure column widths
  worksheet['!cols'] = [
    { wch: 10 }, // ลำดับ
    { wch: 26 }, // ตึก / อาคาร
    { wch: 14 }, // ชั้น
    { wch: 38 }, // หน่วยงาน
    { wch: 24 }, // เบอร์ภายใน
    { wch: 22 }, // เบอร์สายนอก
  ]

  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, worksheet, 'สมุดโทรศัพท์ BGH')
  XLSX.writeFile(workbook, filename)
}

export function downloadExcelTemplate() {
  const sampleData = [
    {
      'ลำดับ': 1,
      'ตึก / อาคาร': 'ตึกเฉลิมพระเกียรติ',
      'ชั้น': 'ชั้น G',
      'หน่วยงาน / แผนก': 'Call Center',
      'เบอร์ภายใน': '6',
      'เบอร์สายนอก': '034-419555',
    },
    {
      'ลำดับ': 2,
      'ตึก / อาคาร': 'ตึกเฉลิมพระเกียรติ',
      'ชั้น': 'ชั้น 1',
      'หน่วยงาน / แผนก': 'จุดประชาสัมพันธ์',
      'เบอร์ภายใน': '1001, 1002',
      'เบอร์สายนอก': '',
    },
    {
      'ลำดับ': 3,
      'ตึก / อาคาร': 'อาคารเฉลิมพระเกียรติสมเด็จพระเทพฯ',
      'ชั้น': 'ชั้น 2',
      'หน่วยงาน / แผนก': 'ห้องตรวจศัลยกรรม',
      'เบอร์ภายใน': '9590',
      'เบอร์สายนอก': '',
    },
  ]

  const worksheet = XLSX.utils.json_to_sheet(sampleData)
  worksheet['!cols'] = [
    { wch: 10 },
    { wch: 32 },
    { wch: 14 },
    { wch: 34 },
    { wch: 20 },
    { wch: 20 },
  ]
  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, worksheet, 'Template')
  XLSX.writeFile(workbook, 'BGH-PhoneBook-Template.xlsx')
}

export async function parseExcelFile(file: File): Promise<ParsedImportResult> {
  const buffer = await file.arrayBuffer()
  const workbook = XLSX.read(buffer, { type: 'array' })

  if (!workbook.SheetNames.length) {
    throw new Error('ไม่พบแผ่นงาน (Sheet) ในไฟล์ Excel นี้')
  }

  const firstSheet = workbook.Sheets[workbook.SheetNames[0]]
  const rows = XLSX.utils.sheet_to_json<Record<string, any>>(firstSheet, { defval: '' })

  if (!rows.length) {
    throw new Error('ไม่พบข้อมูลในไฟล์ Excel ที่เลือก')
  }

  const entries: Array<Omit<Entry, 'id'>> = []
  const errors: string[] = []
  let totalRows = rows.length
  let validRows = 0
  let invalidRows = 0

  for (let i = 0; i < rows.length; i++) {
    const row = rows[i]
    const rowNum = i + 2 // 1-indexed, header is row 1

    // Fuzzy header matching
    let building = ''
    let floor = ''
    let department = ''
    let internalPhone = ''
    let externalPhone = ''
    let sortOrder = 0

    for (const [key, rawVal] of Object.entries(row)) {
      const cleanKey = key.trim().toLowerCase().replace(/[\s\-_/\\()]/g, '')
      const val = String(rawVal ?? '').trim()

      if (/^(ตึก|อาคาร|ตึกอาคาร|building)/i.test(cleanKey)) {
        building = val
      } else if (/^(ชั้น|floor|level)/i.test(cleanKey)) {
        floor = val
      } else if (/^(หน่วยงาน|แผนก|ชื่อหน่วยงาน|หน่วยงานแผนก|department|dept|name)/i.test(cleanKey)) {
        department = val
      } else if (/^(เบอร์ใน|เบอร์ภายใน|internal|internalphone|ext|extension|เบอร์)/i.test(cleanKey)) {
        internalPhone = val
      } else if (/^(เบอร์นอก|เบอร์สายนอก|เบอร์ภายนอก|external|externalphone|direct)/i.test(cleanKey)) {
        externalPhone = val
      } else if (/^(ลำดับ|ลำดับที่|sort|sortorder|order|no)/i.test(cleanKey)) {
        sortOrder = parseInt(val, 10) || 0
      }
    }

    // Skip entirely empty row
    if (!building && !floor && !department && !internalPhone && !externalPhone) {
      totalRows--
      continue
    }

    if (!building || !floor || !department) {
      invalidRows++
      errors.push(`แถวที่ ${rowNum}: ขาดข้อมูลจำเป็น (${!building ? 'ตึก/อาคาร ' : ''}${!floor ? 'ชั้น ' : ''}${!department ? 'หน่วยงาน' : ''})`)
      continue
    }

    validRows++
    entries.push({
      building,
      floor,
      department,
      internal_phone: internalPhone,
      external_phone: externalPhone,
      sort_order: sortOrder || validRows,
    })
  }

  return {
    entries,
    totalRows,
    validRows,
    invalidRows,
    sample: entries.slice(0, 5),
    errors: errors.slice(0, 8),
  }
}
