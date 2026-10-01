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
          <el-form label-width="64px">
            <el-form-item label="花盆">
              <el-select v-model="form.user_garden_id" placeholder="选择具体花盆" style="width: 100%">
                <el-option
                  v-for="g in pots"
                  :key="g.id"
                  :label="`#${g.id} ${g.nickname || ('品种#' + g.plant_species_id)}（${g.location || '未填位置'}）`"
                  :value="g.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="任务"><el-input v-model="form.task_title" placeholder="如：给月季施肥" /></el-form-item>
            <el-form-item label="日期"><el-date-picker v-model="form.remind_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" /></el-form-item>
            <el-form-item label="频率">
              <el-select v-model="form.frequency" style="width: 100%">
                <el-option label="单次" value="" />
                <el-option label="每日" value="daily" />
                <el-option label="每周" value="weekly" />
                <el-option label="每月" value="monthly" />
                <el-option label="每年" value="yearly" />
              </el-select>
            </el-form-item>
            <el-form-item><el-button type="primary" @click="create">创建提醒</el-button></el-form-item>
          </el-form>
          <ReminderList :reminders="reminders" @done="markDone" @remove="remove" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import ReminderList from '@/components/common/ReminderList.vue'
import { useReminderStore } from '@/stores/reminderStore'
import { getSeasonTasks } from '@/utils/season'
import { listGardens } from '@/api/garden'
import type { CareReminder, GardenView } from '@/types/api'

const store = useReminderStore()
const seasonTasks = getSeasonTasks()
const currentMonth = new Date().getMonth() + 1
const reminders = ref<CareReminder[]>([])
const pots = ref<GardenView[]>([])
const form = reactive({ user_garden_id: 0, task_title: '', remind_date: '', frequency: '' })

onMounted(async () => {
  await store.load()
  reminders.value = store.reminders
  pots.value = (await listGardens()).filter((g) => g.id > 0)
})

async function create() {
  if (!form.user_garden_id) {
    ElMessage.warning('请选择花盆')
    return
  }
  if (!form.task_title || !form.remind_date) {
    ElMessage.warning('请填写任务与日期')
    return
  }
  let duplicated = false
  try {
    await store.create({
      user_garden_id: form.user_garden_id,
      task_title: form.task_title,
      remind_date: form.remind_date,
      frequency: form.frequency,
    })
  } catch (e: any) {
    if (e?.existing) {
      duplicated = true
    } else {
      throw e
    }
  }
  reminders.value = store.reminders
  form.task_title = ''
  form.remind_date = ''
  form.frequency = ''
  if (duplicated) {
    ElMessage.warning('已有相同计划，已返回已有计划')
  } else {
    ElMessage.success('养护提醒已创建')
  }
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
