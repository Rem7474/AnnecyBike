import { useEffect, useRef, useState } from 'react'
import type { BikeLive, Station, WSMessage } from '../types'

export function useLiveMap() {
  const [bikes, setBikes] = useState<BikeLive[]>([])
  const [stations, setStations] = useState<Station[]>([])
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    let destroyed = false

    function connect() {
      if (destroyed) return
      const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${protocol}//${location.host}/ws/live`)
      wsRef.current = ws

      ws.onopen = () => setConnected(true)

      ws.onmessage = (evt) => {
        try {
          const msg: WSMessage = JSON.parse(evt.data)
          if (msg.type === 'snapshot') {
            setBikes(msg.bikes)
            setStations(msg.stations)
          }
        } catch {}
      }

      ws.onclose = () => {
        setConnected(false)
        if (!destroyed) {
          retryRef.current = setTimeout(connect, 5000)
        }
      }

      ws.onerror = () => ws.close()
    }

    connect()

    return () => {
      destroyed = true
      if (retryRef.current) clearTimeout(retryRef.current)
      wsRef.current?.close()
    }
  }, [])

  return { bikes, stations, connected }
}
