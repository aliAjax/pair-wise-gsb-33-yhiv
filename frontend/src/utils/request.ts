import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

request.interceptors.request.use((config) => {
  const auth = useAuthStore()
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  return config
})

request.interceptors.response.use(
  (res) => {
    const body = res.data
    if (body && typeof body.code === 'number' && body.code !== 0) {
      ElMessage.error(body.message || '请求失败')
      return Promise.reject(new Error(body.message))
    }
    return body?.data
  },
  (err) => {
    const status = err.response?.status
    const body = err.response?.data
    const msg = body?.message || '网络异常'
    if (status === 401) {
      const auth = useAuthStore()
      auth.logout()
    }
    // 409 冲突（如两台设备同时提交同一计划）时，后端在 data 里带回已有记录，
    // 调用方可以直接拿到已有计划而不是只拿到一个错误。
    if (status === 409 && body?.data) {
      ElMessage.warning(msg)
      return Promise.reject(Object.assign(new Error(msg), { existing: body.data, status }))
    }
    ElMessage.error(msg)
    return Promise.reject(err)
  },
)

export default request
