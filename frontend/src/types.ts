export interface Entry {
  id: number
  building: string
  floor: string
  department: string
  internal_phone: string
  external_phone: string
  sort_order: number
}

export interface BuildingMeta {
  building: string
  count: number
}

export interface Stats {
  total_entries: number
  total_buildings: number
  total_floors: number
}

export interface ImportPayload {
  mode: 'append' | 'replace'
  items: Array<Omit<Entry, 'id'>>
}
