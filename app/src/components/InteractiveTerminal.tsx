import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

interface InteractiveTerminalProps {
    sessionId: string
    chunksB64: string[]
}

function bytesToBase64(bytes: Uint8Array): string {
    // Convert to base64 without blowing the stack
    let binary = ''
    const chunkSize = 0x8000
    for (let i = 0; i < bytes.length; i += chunkSize) {
        binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize))
    }
    return btoa(binary)
}

function base64ToBytes(b64: string): Uint8Array {
    const binary = atob(b64)
    const bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
    return bytes
}

export function InteractiveTerminal({ sessionId, chunksB64 }: InteractiveTerminalProps) {
    const terminalRef = useRef<HTMLDivElement>(null)
    const xtermRef = useRef<Terminal | null>(null)
    const fitAddonRef = useRef<FitAddon | null>(null)
    const processedRef = useRef(0)
    const textDecoderRef = useRef<TextDecoder | null>(null)

    useEffect(() => {
        if (!terminalRef.current || xtermRef.current) return

        const term = new Terminal({
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            fontSize: 14,
            theme: {
                background: '#0b0b0f',
                foreground: '#e4e4e7',
            },
            cursorBlink: true,
            convertEol: false,
        })

        const fitAddon = new FitAddon()
        term.loadAddon(fitAddon)
        fitAddonRef.current = fitAddon

        term.open(terminalRef.current)
        fitAddon.fit()

        xtermRef.current = term
        textDecoderRef.current = new TextDecoder()

        const handleResize = () => fitAddon.fit()
        window.addEventListener('resize', handleResize)

        // Send keystrokes to backend
        const disposable = term.onData(async (data) => {
            try {
                const bytes = new TextEncoder().encode(data)
                const dataB64 = bytesToBase64(bytes)
                const { getApiBaseUrl } = await import('../utils/api')
                const baseUrl = await getApiBaseUrl()
                const url = `${baseUrl}/api/pty/input`
                await fetch(url, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ sessionId, dataB64 })
                })
            } catch (e) {
                // best-effort; don't crash UI
                console.error('PTY input failed:', e)
            }
        })

        return () => {
            window.removeEventListener('resize', handleResize)
            disposable.dispose()
            term.dispose()
        }
    }, [sessionId])

    // Append new output chunks
    useEffect(() => {
        if (!xtermRef.current || !textDecoderRef.current) return
        const newChunks = chunksB64.slice(processedRef.current)
        if (newChunks.length === 0) return

        for (const b64 of newChunks) {
            try {
                const bytes = base64ToBytes(b64)
                const text = textDecoderRef.current.decode(bytes, { stream: true })
                xtermRef.current.write(text)
            } catch (e) {
                console.error('PTY decode/write failed:', e)
            }
        }
        processedRef.current = chunksB64.length
    }, [chunksB64])

    return <div className="h-full w-full" ref={terminalRef} />
}


