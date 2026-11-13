import { useEffect } from 'react'
import { RouterProvider } from 'react-router-dom'
import { router } from './router'
import { useProtocolsStore } from './store/protocols'

export default function App() {
  const loadProtocols = useProtocolsStore((s) => s.load)

  useEffect(() => {
    void loadProtocols()
  }, [loadProtocols])

  return <RouterProvider router={router} />
}
