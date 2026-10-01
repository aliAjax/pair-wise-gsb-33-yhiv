import request from '@/utils/request'
import type { CareReminder, ReminderCreateResult } from '@/types/api'

export function listReminders(status?: string) {
  return request.get<never, CareReminder[]>('/reminders', { params: { status } })
}

export function listRemindersByMonth(year: number, month: number) {
  return request.get<never, CareReminder[]>('/reminders/calendar', { params: { year, month } })
}

export interface ReminderPayload {
  garden_id?: number
  plant_species_id?: number
  task_title: string
  remind_date: string
  frequency?: string
}

// Two devices submitting the same plan get the existing plan back; the result
// carries already_existed so callers can show "已有计划" instead of "已创建".
export function createReminder(payload: ReminderPayload) {
  return request.post<never, ReminderCreateResult>('/reminders', payload)
}

export function updateReminderStatus(id: number, status: string) {
  return request.put<never, CareReminder>(`/reminders/${id}/status`, { status })
}

export function deleteReminder(id: number) {
  return request.delete<never, { deleted: boolean }>(`/reminders/${id}`)
}
