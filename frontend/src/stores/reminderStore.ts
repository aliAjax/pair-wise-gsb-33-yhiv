import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listReminders, updateReminderStatus, createReminder, type ReminderPayload } from '@/api/reminder'
import type { CareReminder, ReminderCreateResult } from '@/types/api'

export const useReminderStore = defineStore('reminder', () => {
  const reminders = ref<CareReminder[]>([])

  async function load(status?: string) {
    reminders.value = await listReminders(status)
  }

  // Returns the creation result; when another device already submitted the
  // same plan, already_existed is true and callers should say "已有计划".
  async function create(payload: ReminderPayload): Promise<ReminderCreateResult> {
    const result = await createReminder(payload)
    await load()
    return result
  }

  async function setStatus(id: number, status: string) {
    await updateReminderStatus(id, status)
    await load()
  }

  return { reminders, load, create, setStatus }
})
