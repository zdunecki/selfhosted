import { Terminal as TerminalIcon } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

export function Deploying() {
    const terminalRef = useRef<HTMLDivElement>(null)
    const xtermRef = useRef<Terminal | null>(null)

    useEffect(() => {
        if (!terminalRef.current || xtermRef.current) return

        const term = new Terminal({
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            fontSize: 14,
            theme: {
                background: '#1e1e1e',
            }
        })
        term.open(terminalRef.current)
        term.writeln('Initializing deployment...')
        term.writeln('Connecting to server...')

        xtermRef.current = term

        // TODO: Connect to websocket

        return () => {
            term.dispose()
        }
    }, [])

    return (
        <div className="flex flex-col h-[calc(100vh-100px)]">
            <h1 className="text-2xl font-bold mb-4 flex items-center gap-2">
                <TerminalIcon /> Deploying Service
            </h1>
            <div className="flex-1 bg-[#1e1e1e] rounded-lg overflow-hidden p-4" ref={terminalRef} />
        </div>
    )
}
