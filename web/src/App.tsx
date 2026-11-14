import { useEffect, useMemo } from 'react'
import { RouterProvider } from 'react-router-dom'
import { createAppRouter } from './router'
import { useProtocolsStore } from './store/protocols'

export default function App({ basename = '' }: { basename?: string }) {
  const loadProtocols = useProtocolsStore((s) => s.load)

  useEffect(() => {
    void loadProtocols()
  }, [loadProtocols])

  // 面板路径随机化：以 window.__PANEL_PATH__（经 main.tsx 传入）作为路由 basename
  const router = useMemo(() => createAppRouter(basename), [basename])

  return <RouterProvider router={router} />
}
