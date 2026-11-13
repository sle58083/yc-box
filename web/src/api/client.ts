import axios from 'axios'
import type {
  AxiosError,
  AxiosInstance,
  AxiosRequestConfig,
  InternalAxiosRequestConfig,
} from 'axios'

// 统一响应信封（system_design.md §9.1）：{code, message, data}
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

// 业务错误：后端 code !== 0 时抛出，携带业务码。
export class ApiError extends Error {
  readonly code: number

  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

const client: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 15000,
})

// 请求拦截：附加 JWT（Bearer）。
client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = localStorage.getItem('sbp_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：统一解包 {code,message,data}，并集中处理 401。
client.interceptors.response.use(
  (response) => {
    const body = response.data as ApiResponse | undefined
    if (body && typeof body.code === 'number' && body.code !== 0) {
      return Promise.reject(new ApiError(body.code, body.message))
    }
    return response
  },
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('sbp_token')
      if (!window.location.pathname.startsWith('/login')) {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  },
)

async function unwrap<T>(promise: Promise<{ data: ApiResponse<T> }>): Promise<T> {
  const res = await promise
  return res.data.data
}

export function get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return unwrap(client.get<ApiResponse<T>>(url, config))
}

export function post<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
  return unwrap(client.post<ApiResponse<T>>(url, data, config))
}

export function put<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
  return unwrap(client.put<ApiResponse<T>>(url, data, config))
}

export function del<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return unwrap(client.delete<ApiResponse<T>>(url, config))
}

export default client
