<template>
  <div class="page">
    <h1>季节养护日历</h1>
    <el-row :gutter="16">
      <el-col :xs="24" :md="14">
        <el-timeline>
          <el-timeline-item v-for="t in seasonTasks" :key="t.month" :timestamp="t.label" :type="t.month === currentMonth ? 'primary' : ''">
            {{ t.task }}
            <el-tag v-if="t.month === currentMonth" size="small" type="success">本月</el-tag>
          </el-timeline-item>
        </el-timeline>
      </el-col>
      <el-col :xs="24" :md="10">
        <el-card>
          <template #header>本月养护提醒</template>
          <el-form inline>
            <el-form-item label="花盆">
              <el-select v-model="form.garden_id" placeholder="选择花盆" style="width: 220px">
                <el-option
                  v-for="g in activePots"
                  :key="g.id"
                  :label="`${g.pot_no} ${g.plant_name || g.nickname}`"
                  :value="g.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="任务"><el-input v-model="form.task_title" placeholder="如：给月季施肥" /></el-form-item>
            <el-form-item label="日期"><el-date-picker v-model="form.remind_date" type="date" value-format="YYYY-MM-DD" /></el-form-item>
            <el-form-item><el-button type="primary" @click="create">创建提醒</el-button></el-form-item>
          </el-form>
          <ReminderList :reminders="reminders" @done="markDone" @remove="remove" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import ReminderList from '@/components/common/ReminderList.vue'
import { useReminderStore } from '@/stores/reminderStore'
import { listGardens } from '@/api/garden'
import { getSeasonTasks } from '@/utils/season'
import type { CareReminder, UserGarden } from '@/types/api'

const store = useReminderStore()
const seasonTasks = getSeasonTasks()
const currentMonth = new Date().getMonth() + 1
const reminders = ref<CareReminder[]>([])
const gardenItems = ref<UserGarden[]>([])
const activePots = computed(() => gardenItems.value.filter((g) => g.status === 'active'))
const form = reactive({ garden_id: undefined as number | undefined, task_title: '', remind_date: '' })

onMounted(async () => {
  await Promise.all([store.load(), listGardens().then((g) => (gardenItems.value = g))])
  reminders.value = store.reminders
})

async function create() {
  if (!form.garden_id || !form.task_title || !form.remind_date) {
    ElMessage.warning('请选择花盆、填写任务与日期')
    return
  }
  const result = await store.create({ garden_id: form.garden_id, task_title: form.task_title, remind_date: form.remind_date })
  reminders.value = store.reminders
  form.task_title = ''
  form.remind_date = ''
  ElMessage.success(result.already_existed ? '该计划已存在，返回已有计划' : '养护提醒已创建')
}
async function markDone(id: number) {
  await store.setStatus(id, 'done')
  reminders.value = store.reminders
}
async function remove(id: number) {
  const { deleteReminder } = await import('@/api/reminder')
  await deleteReminder(id)
  await store.load()
  reminders.value = store.reminders
  ElMessage.success('已删除提醒')
}
</script>

<style scoped>
.page { max-width: 1200px; margin: 0 auto; }
</style>
