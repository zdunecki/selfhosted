import { Terminal as TerminalIcon } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

interface TerminalViewProps {
    logs: string[]
}

export function TerminalView({ logs }: TerminalViewProps) {
    const terminalRef = useRef<HTMLDivElement>(null)
    const xtermRef = useRef<Terminal | null>(null)
    const fitAddonRef = useRef<FitAddon | null>(null)

    useEffect(() => {
        if (!terminalRef.current || xtermRef.current) return

        const term = new Terminal({
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            fontSize: 14,
            theme: {
                background: '#ffffff',
                foreground: '#18181b', // zinc-900
                cursor: '#18181b',
                selectionBackground: '#bfdbfe', // blue-200
            },
            cursorBlink: true,
            convertEol: true, // Important for processing \n correctly
        })

        const fitAddon = new FitAddon()
        term.loadAddon(fitAddon)
        fitAddonRef.current = fitAddon

        term.open(terminalRef.current)
        fitAddon.fit()

        xtermRef.current = term

        // Handle window resize
        const handleResize = () => fitAddon.fit()
        window.addEventListener('resize', handleResize)

        return () => {
            window.removeEventListener('resize', handleResize)
            term.dispose()
        }
    }, [])

    // Update logs
    useEffect(() => {
        if (xtermRef.current && logs.length > 0) {
            // Write only new logs or clear and write all?
            // Since props.logs is the full array, we should probably keep track of what we wrote.
            // But xterm is stateful.
            // Better approach: expose a method to write to it, or just write the last log if we assume append?
            // Actually, React paradigm with xterm is tricky. 
            // Let's just assume we receive the *latest* chunk or we manage the diff.
            // Simplest for now: write the last log entry if it changed.
            // BUT props.logs is likely the full history or accumulated chunks.
            // Let's reconstruct for simplicity or just clear and rewrite (bad performance).

            // Optimization: The parent should probably just pass the *new* content. 
            // But let's stick to the prop interface.
            // We'll trust the parent wraps this or we just write the last element if we change the interface.
        }
    }, [logs])

    // Better approach: Let's make this component responsible for the terminal instance 
    // and expose a way to write to it, OR just have it accept the *full content* and we write the diff.
    // Actually, xterm handle \r\n well.
    // Let's try to just write the new lines.

    // We will use a ref to track the number of lines processed
    const processedCount = useRef(0)

    useEffect(() => {
        if (!xtermRef.current) return

        const newLogs = logs.slice(processedCount.current)
        if (newLogs.length > 0) {
            newLogs.forEach(log => {
                // Each SSE `data:` line is one message; render as a new terminal line.
                // Use CRLF for xterm; also normalize any embedded newlines just in case.
                const normalized = log.replace(/\r?\n/g, '\r\n')
                xtermRef.current?.write(normalized + '\r\n')
            })
            processedCount.current = logs.length
        }
    }, [logs])


    return (
        <div className="flex flex-col h-full bg-white rounded-lg overflow-hidden border border-zinc-200 shadow-inner">
            <div className="bg-zinc-50 px-4 py-2 flex items-center gap-2 border-b border-zinc-200">
                <div className="flex gap-1.5">
                    <div className="w-3 h-3 rounded-full bg-red-400/80"></div>
                    <div className="w-3 h-3 rounded-full bg-yellow-400/80"></div>
                    <div className="w-3 h-3 rounded-full bg-green-400/80"></div>
                </div>
                <div className="flex items-center gap-2 ml-4 text-zinc-500 text-xs font-mono">
                    <TerminalIcon size={12} />
                    <span>deploy.log</span>
                </div>
            </div>
            <div className="flex-1 p-2" ref={terminalRef} />
        </div>
    )
}
