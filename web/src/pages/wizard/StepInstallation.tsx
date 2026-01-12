import { Server, Shield } from 'lucide-react'
import { useEffect } from 'react'
import { TerminalView } from '../../components/TerminalView'
import { TRexGame } from '../../components/TRexGame'
import { InteractiveTerminal } from '../../components/InteractiveTerminal'

interface StepInstallationProps {
    logs: string[]
    ptySessionId: string
    ptyChunksB64: string[]
    hasTTYAutomation: boolean
    ttyAutoAnswering: boolean
    onAutoAnswerTTY: () => Promise<void>
    deployError: string | null
    deployComplete: boolean
    deployedUrl: string
    showFullLogs: boolean
    setShowFullLogs: (show: boolean) => void
    setDeployError: (error: string | null) => void
    setCurrentStepIndex: (index: number) => void
    size: string
    region: string
}

export function StepInstallation({
    logs,
    ptySessionId,
    ptyChunksB64,
    hasTTYAutomation,
    ttyAutoAnswering,
    onAutoAnswerTTY,
    deployError,
    deployComplete,
    deployedUrl,
    showFullLogs,
    setShowFullLogs,
    setDeployError,
    setCurrentStepIndex,
    size,
    region
}: StepInstallationProps) {
    useEffect(() => {
        if (ptySessionId) setShowFullLogs(true)
    }, [ptySessionId, setShowFullLogs])

    return (
        <div className="h-full flex flex-col animate-in fade-in zoom-in-95 duration-500">
            <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-3">
                    <div className={`relative flex items-center justify-center w-8 h-8 rounded-full bg-zinc-100 border border-zinc-200
                        ${deployError ? 'border-red-200 bg-red-50' : deployComplete ? 'border-green-200 bg-green-50' : 'border-blue-200 bg-blue-50'}
                     `}>
                        {deployError ? (
                            <div className="w-3 h-3 bg-red-500 rounded-sm" />
                        ) : deployComplete ? (
                            <div className="w-3 h-3 bg-green-500 rounded-full shadow-sm" />
                        ) : (
                            <>
                                <div className="absolute w-full h-full rounded-full border border-blue-400/50 animate-ping" />
                                <div className="w-3 h-3 bg-blue-500 rounded-full shadow-sm" />
                            </>
                        )}
                    </div>
                    <div>
                        <h2 className="text-lg font-medium text-zinc-900">
                            {deployError ? 'Deployment Failed' : deployComplete ? 'Installation Complete' : 'Provisioning app'}
                        </h2>
                        <p className="text-sm text-zinc-500">
                            {deployError
                                ? 'Check the logs for details.'
                                : deployComplete
                                    ? 'Your app is ready. You can open it now.'
                                    : 'Please wait while we set up your app.'}
                        </p>
                        {deployComplete && deployedUrl ? (
                            <p className="text-sm text-zinc-600 mt-1">
                                Visit:{' '}
                                <a
                                    href={deployedUrl}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="font-mono text-[#F38020] hover:underline"
                                >
                                    {deployedUrl}
                                </a>
                            </p>
                        ) : null}
                    </div>
                </div>

                {deployError ? (
                    <button
                        onClick={() => {
                            setDeployError(null)
                            setCurrentStepIndex(2)
                        }}
                        className="px-4 py-2 bg-white hover:bg-zinc-50 text-zinc-700 text-sm font-medium rounded-lg transition-colors border border-zinc-200 shadow-sm"
                    >
                        Retry Configuration
                    </button>
                ) : (
                    <div className="flex items-center gap-3">
                        {deployComplete && deployedUrl ? (
                            <a
                                href={deployedUrl}
                                target="_blank"
                                rel="noreferrer"
                                className="px-4 py-2 bg-[#F38020] text-white hover:bg-[#F38020]/90 text-sm font-medium rounded-lg transition-colors shadow-sm"
                            >
                                Open App
                            </a>
                        ) : null}
                        <button
                            onClick={() => setShowFullLogs(!showFullLogs)}
                            className="text-sm text-zinc-500 hover:text-zinc-900 transition-colors flex items-center gap-2"
                        >
                            {showFullLogs ? 'Minimize Logs' : 'View Full Logs'}
                        </button>
                    </div>
                )}
            </div>

            <div className={`
                transition-all duration-500 ease-in-out overflow-hidden rounded-xl border border-zinc-200 bg-zinc-50 shadow-inner
                ${showFullLogs ? 'flex-1 opacity-100' : 'h-24 opacity-100 ring-1 ring-zinc-900/5'}
            `}>
                {showFullLogs ? (
                    ptySessionId ? (
                        <div className="h-full p-2 bg-[#0b0b0f]">
                            {hasTTYAutomation && (
                                <div className="mb-2 flex items-center justify-between rounded-lg border border-zinc-700/40 bg-zinc-900/40 px-3 py-2">
                                    <div className="text-xs text-zinc-200">
                                        Interactive setup detected. You can auto-answer from your selected options, or type manually.
                                    </div>
                                    <button
                                        onClick={onAutoAnswerTTY}
                                        disabled={ttyAutoAnswering}
                                        className={`text-xs px-3 py-1.5 rounded-md transition-colors border
                                            ${ttyAutoAnswering
                                                ? 'bg-zinc-800 text-zinc-400 border-zinc-700 cursor-not-allowed'
                                                : 'bg-[#F38020] text-white border-[#F38020] hover:bg-[#F38020]/90'}
                                        `}
                                    >
                                        {ttyAutoAnswering ? 'Auto-answering…' : 'Auto-answer setup'}
                                    </button>
                                </div>
                            )}
                            <InteractiveTerminal sessionId={ptySessionId} chunksB64={ptyChunksB64} />
                        </div>
                    ) : (
                        <TerminalView logs={logs} />
                    )
                ) : (
                    <div className="h-full flex flex-col justify-center px-6 cursor-pointer group" onClick={() => setShowFullLogs(true)}>
                        <div className="font-mono text-xs text-zinc-500 mb-2 uppercase tracking-wider group-hover:text-zinc-600 transition-colors">Latest Log Output</div>
                        <div className="font-mono text-sm text-zinc-600 truncate group-hover:text-zinc-900 transition-colors">
                            {logs.length > 0 ? logs[logs.length - 1] : 'Initializing deployment sequence...'}
                        </div>
                        {!deployError && !deployComplete && <div className="h-0.5 w-full bg-zinc-200 mt-4 overflow-hidden rounded-full">
                            <div className="h-full bg-blue-500 w-1/3 animate-[loading_2s_ease-in-out_infinite]" />
                        </div>}
                    </div>
                )}
            </div>

            {!showFullLogs && !ptySessionId && (
                <>
                    <div className="mt-8 grid grid-cols-1 md:grid-cols-2 gap-4 animate-in slide-in-from-bottom-2 duration-700 delay-200">
                        <div className="p-4 rounded-xl bg-white border border-zinc-200 shadow-sm flex gap-3">
                            <div className="p-2 bg-blue-50 text-blue-600 rounded-lg h-fit">
                                <Server size={18} />
                            </div>
                            <div>
                                <h3 className="text-sm font-medium text-zinc-900 mb-1">Provisioning Server</h3>
                                <p className="text-xs text-zinc-500 leading-relaxed">
                                    Allocating {size} instance in {region}...
                                </p>
                            </div>
                        </div>
                        <div className="p-4 rounded-xl bg-white border border-zinc-200 shadow-sm flex gap-3">
                            <div className="p-2 bg-green-50 text-green-600 rounded-lg h-fit">
                                <Shield size={18} />
                            </div>
                            <div>
                                <h3 className="text-sm font-medium text-zinc-900 mb-1">Security Hardening</h3>
                                <p className="text-xs text-zinc-500 leading-relaxed">
                                    Configuring UFW firewall, SSH keys, and fail2ban protection.
                                </p>
                            </div>
                        </div>
                    </div>

                    {/* T-Rex Game for entertainment during installation */}
                    {!deployError && (
                        <div className="mt-8 animate-in fade-in slide-in-from-bottom-4 duration-700 delay-300">
                            <div className="bg-white border border-zinc-200 rounded-xl p-4 shadow-sm">
                                <div className="mb-3 flex items-center justify-between">
                                    <div>
                                        <h3 className="text-sm font-medium text-zinc-900 mb-1">Waiting for installation?</h3>
                                        <p className="text-xs text-zinc-500">
                                            Pass the time with a quick game while we set everything up.
                                        </p>
                                    </div>
                                </div>
                                <div className="h-64 rounded-lg overflow-hidden border border-zinc-100 bg-zinc-50">
                                    <TRexGame />
                                </div>
                            </div>
                        </div>
                    )}
                </>
            )}
        </div>
    )
}

