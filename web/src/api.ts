export type Role = 'admin' | 'member'
export type User = { id: number; username: string; display_name: string; role: Role }
export type Member = User & { disabled: boolean }
export type Session = { user: User; csrf: string }
export type Species = { id: number; name: string }
export type Plant = {
  id: number; name: string; species_id: number; species_name: string;
  planted_on: string; location: string; notes: string; status: 'active' | 'archived';
  photo_count: number; latest_photo_id: number; created_at: string
}
export type PlantFields = Pick<Plant, 'name' | 'species_id' | 'planted_on' | 'location' | 'notes' | 'status'>
export type Photo = { id: number; name: string; mime: string; size: number }
export type Entry = {
  id: number; plant_id: number; author: string; observed_at: string;
  note: string; photos: Photo[]
}
export type EntryPage = { items: Entry[]; has_more: boolean }
export type CalendarDay = { day: string; count: number }
export type ApiError = Error & { status?: number }
let csrf = ''
export function setCsrf(token: string) { csrf = token }
export async function api<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const headers = new Headers(opts.headers)
  if (opts.body && !(opts.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  if (opts.method && !['GET', 'HEAD'].includes(opts.method.toUpperCase()) && csrf) headers.set('X-CSRF-Token', csrf)
  const response = await fetch('/api/v1' + path, { ...opts, credentials: 'same-origin', headers })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    const error: ApiError = new Error(body.error || '请求失败（HTTP ' + response.status + '）')
    error.status = response.status
    throw error
  }
  return response.json() as Promise<T>
}
export const json = (body: unknown, method = 'POST'): RequestInit => ({method, body: JSON.stringify(body)})
export const thumb = (id: number) => '/api/v1/photos/' + id + '/thumb'
export const preview = (id: number) => '/api/v1/photos/' + id + '/preview'
export const original = (id: number) => '/api/v1/photos/' + id + '/original'
export function localDateTime(date?: string) {
  const d = date ? new Date(date) : new Date()
  const local = new Date(d.getTime() - d.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}
export function withLocalOffset(value: string) {
  const d = new Date(value)
  const offset = -d.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const hh = String(Math.floor(Math.abs(offset) / 60)).padStart(2, '0')
  const mm = String(Math.abs(offset) % 60).padStart(2, '0')
  return value + ':00' + sign + hh + ':' + mm
}
export function displayDate(date: string) {
  return new Date(date).toLocaleString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}
