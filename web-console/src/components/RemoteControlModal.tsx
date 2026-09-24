import React, { useEffect, useRef, useState } from 'react'
import { Monitor, X, Eye, MousePointer, Maximize2, Minimize2, AlertCircle } from 'lucide-react'
import { getStoredToken } from '../services/api'
import type { DeviceDTO } from '../types/api'

interface RemoteControlModalProps {
  device: DeviceDTO | null
  initialMode?: 'full_control' | 'view_only'
  onClose: () => void
}

export const RemoteControlModal: React.FC<RemoteControlModalProps> = ({
  device,
  initialMode = 'full_control',
  onClose,
}) => {
  const [mode, setMode] = useState<'full_control' | 'view_only'>(initialMode)
  const [status, setStatus] = useState<'initiating' | 'connecting' | 'active' | 'ended' | 'error'>('initiating')
  const [errorMessage, setErrorMessage] = useState<string>('')
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [fps, setFps] = useState<number>(0)
  const [bytesReceived, setBytesReceived] = useState<number>(0)
  const [resolution, setResolution] = useState<{ width: number; height: number }>({ width: 0, height: 0 })
  const [isFullscreen, setIsFullscreen] = useState<boolean>(false)

  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)
  const frameCountRef = useRef<number>(0)
  const lastFpsTimeRef = useRef<number>(Date.now())
  const lastImgRef = useRef<HTMLImageElement | null>(null)

  useEffect(() => {
    if (!device) return

    const token = getStoredToken()
    if (!token) {
      setStatus('error')
      setErrorMessage('Authentication token missing. Please log in again.')
      return
    }

    let isCancelled = false
    let currentSessionId = ''

    // 1. Initialize session via REST API
    const initSession = async () => {
      try {
        const resp = await fetch(`/api/devices/${device.id}/remotecontrol/session`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ mode }),
        })

        if (!resp.ok) {
          const errData = await resp.json().catch(() => ({}))
          throw new Error(errData.error || `Failed to start session (HTTP ${resp.status})`)
        }

        const data = await resp.json()
        currentSessionId = data.id
        if (isCancelled) return

        setSessionId(currentSessionId)
        setStatus('connecting')

        // 2. Open operator WebSocket relay
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
        const wsUrl = `${protocol}//${window.location.host}/api/devices/${device.id}/remotecontrol/ws?token=${encodeURIComponent(token)}&session=${encodeURIComponent(currentSessionId)}`

        const ws = new WebSocket(wsUrl)
        ws.binaryType = 'arraybuffer'
        wsRef.current = ws

        ws.onopen = () => {
          if (!isCancelled) {
            setStatus('active')
          }
        }

        ws.onmessage = (event) => {
          if (typeof event.data === 'string') return

          const buffer = event.data as ArrayBuffer
          if (buffer.byteLength < 4) return

          setBytesReceived((prev) => prev + buffer.byteLength)
          frameCountRef.current++

          // Calculate FPS every 1 second
          const now = Date.now()
          if (now - lastFpsTimeRef.current >= 1000) {
            setFps(frameCountRef.current)
            frameCountRef.current = 0
            lastFpsTimeRef.current = now
          }

          const view = new DataView(buffer)
          const width = view.getUint16(0)
          const height = view.getUint16(2)
          setResolution({ width, height })

          const jpegBytes = buffer.slice(4)
          const blob = new Blob([jpegBytes], { type: 'image/jpeg' })
          const url = URL.createObjectURL(blob)

          const img = new Image()
          img.onload = () => {
            const canvas = canvasRef.current
            if (canvas) {
              if (canvas.width !== width || canvas.height !== height) {
                canvas.width = width
                canvas.height = height
              }
              const ctx = canvas.getContext('2d')
              if (ctx) {
                ctx.drawImage(img, 0, 0)
              }
            }
            URL.revokeObjectURL(url)
          }
          img.src = url
          lastImgRef.current = img
        }

        ws.onerror = () => {
          if (!isCancelled) {
            setStatus('error')
            setErrorMessage('Remote desktop relay connection error.')
          }
        }

        ws.onclose = () => {
          if (!isCancelled && status !== 'error') {
            setStatus('ended')
          }
        }
      } catch (err: unknown) {
        if (!isCancelled) {
          setStatus('error')
          setErrorMessage(err instanceof Error ? err.message : 'Unknown initialization error')
        }
      }
    }

    initSession()

    return () => {
      isCancelled = true
      if (wsRef.current) {
        try {
          wsRef.current.close()
        } catch {
          // ignore
        }
      }
      if (currentSessionId && device) {
        fetch(`/api/devices/${device.id}/remotecontrol/sessions/${currentSessionId}/stop`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
        }).catch(() => {})
      }
    }
  }, [device, mode])

  const sendInput = (msg: Record<string, unknown>) => {
    if (mode === 'view_only' || status !== 'active' || !wsRef.current) return
    if (wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg))
    }
  }

  const getCanvasCoords = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current
    if (!canvas) return { x: 0, y: 0 }
    const rect = canvas.getBoundingClientRect()
    const scaleX = canvas.width / rect.width
    const scaleY = canvas.height / rect.height
    const x = Math.round((e.clientX - rect.left) * scaleX)
    const y = Math.round((e.clientY - rect.top) * scaleY)
    return { x, y }
  }

  const handleMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const { x, y } = getCanvasCoords(e)
    sendInput({ type: 'mouse', action: 'move', x, y })
  }

  const handleMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const { x, y } = getCanvasCoords(e)
    let button = 'left'
    if (e.button === 2) button = 'right'
    else if (e.button === 1) button = 'middle'
    sendInput({ type: 'mouse', action: 'down', x, y, button })
  }

  const handleMouseUp = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const { x, y } = getCanvasCoords(e)
    let button = 'left'
    if (e.button === 2) button = 'right'
    else if (e.button === 1) button = 'middle'
    sendInput({ type: 'mouse', action: 'up', x, y, button })
  }

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault()
  }

  const handleWheel = (e: React.WheelEvent<HTMLCanvasElement>) => {
    e.preventDefault()
    const { x, y } = getCanvasCoords(e)
    const delta = e.deltaY < 0 ? 120 : -120
    sendInput({ type: 'mouse', action: 'wheel', x, y, delta })
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (mode === 'view_only') return
    e.preventDefault()
    sendInput({ type: 'keyboard', action: 'down', key: e.key, code: e.keyCode })
  }

  const handleKeyUp = (e: React.KeyboardEvent) => {
    if (mode === 'view_only') return
    e.preventDefault()
    sendInput({ type: 'keyboard', action: 'up', key: e.key, code: e.keyCode })
  }

  const toggleFullscreen = () => {
    if (!containerRef.current) return
    if (!isFullscreen) {
      if (containerRef.current.requestFullscreen) {
        containerRef.current.requestFullscreen().catch(() => {})
      }
      setIsFullscreen(true)
    } else {
      if (document.exitFullscreen) {
        document.exitFullscreen().catch(() => {})
      }
      setIsFullscreen(false)
    }
  }

  if (!device) return null

  const formatMB = (bytes: number) => (bytes / (1024 * 1024)).toFixed(1) + ' MB'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/85 backdrop-blur-sm p-4">
      <div
        ref={containerRef}
        tabIndex={0}
        onKeyDown={handleKeyDown}
        onKeyUp={handleKeyUp}
        className="flex flex-col w-full max-w-6xl h-[85vh] bg-slate-900 border border-slate-700 rounded-xl shadow-2xl overflow-hidden focus:outline-none"
      >
        {/* Top Control Bar */}
        <div className="flex items-center justify-between px-4 py-2.5 bg-slate-950 border-b border-slate-800 text-slate-200">
          <div className="flex items-center space-x-3">
            <Monitor className="w-5 h-5 text-indigo-400" />
            <div className="flex flex-col">
              <div className="flex items-center space-x-2">
                <span className="font-semibold text-sm">{device.hostname}</span>
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 font-mono">
                  {device.site || 'Default'}
                </span>
                <span
                  className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                    status === 'active'
                      ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                      : status === 'connecting' || status === 'initiating'
                      ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                      : status === 'ended'
                      ? 'bg-slate-700 text-slate-300'
                      : 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
                  }`}
                >
                  {status === 'active' ? 'Live Stream' : status}
                </span>
                {sessionId && (
                  <span className="text-xs text-slate-500 font-mono hidden md:inline">
                    #{sessionId.substring(0, 8)}
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Telemetry info */}
          <div className="flex items-center space-x-4 text-xs font-mono text-slate-400">
            {resolution.width > 0 && (
              <span>
                {resolution.width}x{resolution.height}
              </span>
            )}
            <span>{fps} FPS</span>
            <span>{formatMB(bytesReceived)}</span>
          </div>

          {/* Actions & Mode switch */}
          <div className="flex items-center space-x-2">
            <div className="flex items-center bg-slate-800 p-0.5 rounded-lg border border-slate-700 text-xs">
              <button
                type="button"
                onClick={() => setMode('full_control')}
                className={`flex items-center space-x-1 px-2.5 py-1 rounded-md transition-colors ${
                  mode === 'full_control' ? 'bg-indigo-600 text-white font-medium' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <MousePointer className="w-3.5 h-3.5" />
                <span>Control</span>
              </button>
              <button
                type="button"
                onClick={() => setMode('view_only')}
                className={`flex items-center space-x-1 px-2.5 py-1 rounded-md transition-colors ${
                  mode === 'view_only' ? 'bg-indigo-600 text-white font-medium' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Eye className="w-3.5 h-3.5" />
                <span>View</span>
              </button>
            </div>

            <button
              type="button"
              onClick={toggleFullscreen}
              className="p-1.5 text-slate-400 hover:text-slate-200 rounded-lg hover:bg-slate-800 transition-colors"
              title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
            >
              {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
            </button>

            <button
              type="button"
              onClick={onClose}
              className="p-1.5 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800 transition-colors"
              title="Close Session"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Viewport Canvas */}
        <div className="relative flex-1 flex items-center justify-center bg-black overflow-hidden select-none">
          {status === 'active' ? (
            <canvas
              ref={canvasRef}
              onMouseMove={handleMouseMove}
              onMouseDown={handleMouseDown}
              onMouseUp={handleMouseUp}
              onContextMenu={handleContextMenu}
              onWheel={handleWheel}
              className={`max-w-full max-h-full object-contain ${
                mode === 'full_control' ? 'cursor-crosshair' : 'cursor-default'
              }`}
            />
          ) : status === 'error' ? (
            <div className="flex flex-col items-center justify-center text-center p-6 max-w-md">
              <AlertCircle className="w-12 h-12 text-rose-500 mb-3" />
              <h3 className="text-base font-semibold text-slate-100 mb-1">Session Failed</h3>
              <p className="text-sm text-slate-400 mb-4">{errorMessage}</p>
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-sm transition-colors"
              >
                Close Window
              </button>
            </div>
          ) : status === 'ended' ? (
            <div className="flex flex-col items-center justify-center text-center p-6">
              <Monitor className="w-12 h-12 text-slate-600 mb-3" />
              <h3 className="text-base font-semibold text-slate-200 mb-1">Remote Session Ended</h3>
              <p className="text-sm text-slate-400 mb-4">
                The desktop streaming session was closed by the operator or endpoint.
              </p>
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg text-sm transition-colors"
              >
                Close Window
              </button>
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center space-y-3">
              <div className="w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
              <span className="text-sm text-slate-400 font-mono">
                {status === 'initiating' ? 'Requesting desktop tunnel...' : 'Connecting to remote desktop relay...'}
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
