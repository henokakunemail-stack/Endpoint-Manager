import React, { useEffect, useRef, useState } from 'react'
import { AlertCircle, Terminal, X } from 'lucide-react'
import { getStoredToken } from '../services/api'
import type { DeviceDTO } from '../types/api'

interface InteractiveTerminalModalProps {
  device: DeviceDTO | null
  shell: string
  onClose: () => void
}

interface TerminalMessage {
  type: string
  session_id: string
  data: string
}

export const InteractiveTerminalModal: React.FC<InteractiveTerminalModalProps> = ({
  device,
  shell,
  onClose,
}) => {
  const [sessionID, setSessionID] = useState<string | null>(null)
  const [status, setStatus] = useState<'connecting' | 'active' | 'closed' | 'error'>('connecting')
  const [errorMessage, setErrorMessage] = useState<string>('')
  const wsRef = useRef<WebSocket | null>(null)
  const outputRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const bufferRef = useRef<string>('')

  useEffect(() => {
    if (!device) return

    const token = getStoredToken()
    if (!token) {
      setStatus('error')
      setErrorMessage('Authentication token missing. Please log in again.')
      return
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/devices/${device.id}/terminal/ws?token=${encodeURIComponent(token)}&shell=${encodeURIComponent(shell)}`

    let ws: WebSocket
    try {
      ws = new WebSocket(wsUrl)
    } catch (err: unknown) {
      setStatus('error')
      setErrorMessage('Failed to open terminal connection.')
      return
    }
    wsRef.current = ws

    ws.onopen = () => {
      setStatus('active')
      inputRef.current?.focus()
    }

    ws.onmessage = (event) => {
      try {
        const msg: TerminalMessage = JSON.parse(event.data)
        if (msg.type === 'term.open') {
          if (msg.session_id) setSessionID(msg.session_id)
          appendOutput(msg.data + '\r\n', 'system')
        } else if (msg.type === 'term.data') {
          if (msg.session_id) setSessionID(msg.session_id)
          appendOutput(msg.data)
        } else if (msg.type === 'term.close') {
          setStatus('closed')
          appendOutput('\r\n*** Remote shell session terminated ***\r\n', 'system')
        }
      } catch {
        // Ignore malformed frames
      }
    }

    ws.onerror = () => {
      setStatus('error')
      setErrorMessage('Terminal connection error. The endpoint may be offline.')
    }

    ws.onclose = () => {
      if (status !== 'error' && status !== 'closed') {
        setStatus('closed')
      }
    }

    return () => {
      try {
        ws.close()
      } catch {
        // ignore
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [device, shell])

  const appendOutput = (text: string, kind: 'stdout' | 'system' = 'stdout') => {
    bufferRef.current += text
    if (outputRef.current) {
      const el = outputRef.current
      el.textContent = bufferRef.current
      el.className = `terminal-output ${kind === 'system' ? 'terminal-system' : ''}`
      el.scrollTop = el.scrollHeight
    }
  }

  const handleSendInput = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return
    if (status !== 'active' || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return

    const input = inputRef.current?.value ?? ''
    // Echo locally so the operator sees what they typed (remote echo not guaranteed)
    appendOutput(input + '\r\n')
    if (inputRef.current) inputRef.current.value = ''

    wsRef.current.send(
      JSON.stringify({
        type: 'term.data',
        session_id: sessionID,
        data: input + '\r\n',
      })
    )
  }

  const handleClose = () => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      try {
        wsRef.current.send(JSON.stringify({ type: 'term.close', session_id: sessionID }))
      } catch {
        // ignore
      }
    }
    try {
      wsRef.current?.close()
    } catch {
      // ignore
    }
    onClose()
  }

  if (!device) return null

  return (
    <div className="modal-backdrop" onClick={handleClose}>
      <div
        className="modal modal-wide"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: '900px' }}
      >
        <div className="modal-header">
          <div>
            <h2 className="modal-title">
              <Terminal size={20} /> Live Interactive Terminal
            </h2>
            <p className="modal-subtitle">
              {device.hostname} · {shell} shell ·{' '}
              <span className="font-mono">{device.id.substring(0, 16)}</span>
            </p>
          </div>
          <div className="terminal-status-group">
            <span className={`status-pill ${status === 'active' ? 'status-success' : status === 'connecting' ? 'status-running' : 'status-failed'}`}>
              {status.toUpperCase()}
            </span>
            <button
              type="button"
              className="btn btn-icon"
              onClick={handleClose}
              title="Disconnect and Close"
            >
              <X size={18} />
            </button>
          </div>
        </div>

        <div className="modal-body">
          {status === 'error' && (
            <div className="notification-banner error">
              <AlertCircle size={18} />
              <span>{errorMessage}</span>
            </div>
          )}

          <div className="live-terminal">
            <div className="terminal-output" ref={outputRef} />
            <div className="terminal-input-row">
              <span className="terminal-prompt">$</span>
              <input
                ref={inputRef}
                type="text"
                className="terminal-input"
                placeholder={status === 'active' ? 'Type a command and press Enter...' : 'Terminal not active'}
                disabled={status !== 'active'}
                onKeyDown={handleSendInput}
                spellCheck={false}
                autoComplete="off"
              />
            </div>
          </div>

          <p className="form-hint">
            Interactive session streams bidirectionally over the agent's outbound WebSocket.
            Closing this window terminates the shell process on the endpoint.
          </p>
        </div>
      </div>
    </div>
  )
}
