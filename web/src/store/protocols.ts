import { create } from 'zustand'
import { listProtocols, type ProtocolMeta } from '../api/protocols'
import { LOCAL_PROTOCOLS } from '../utils/protocols'

interface ProtocolsState {
  metas: ProtocolMeta[]
  loaded: boolean
  load: () => Promise<void>
}

// 协议元数据全局状态：优先使用后端 GET /protocols 下发结果，
// 失败或为空时回退到本地 LOCAL_PROTOCOLS，保证页面始终可渲染。
export const useProtocolsStore = create<ProtocolsState>((set, get) => ({
  metas: LOCAL_PROTOCOLS,
  loaded: false,
  load: async () => {
    if (get().loaded) return
    try {
      const metas = await listProtocols()
      if (metas && metas.length > 0) {
        set({ metas, loaded: true })
      } else {
        set({ loaded: true })
      }
    } catch {
      set({ loaded: true })
    }
  },
}))
