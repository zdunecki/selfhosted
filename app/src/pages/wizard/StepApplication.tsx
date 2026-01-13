import { Server } from 'lucide-react'
import { SelectCard } from '../../components/SelectCard'
import type { App, WizardQuestion } from '../../types'
import type { WizardState, WizardActions } from './types'

interface StepApplicationProps {
    apps: App[]
    state: WizardState
    actions: WizardActions
    getAppLogo: (name: string) => string | undefined
}

export function StepApplication({ apps, state, actions, getAppLogo }: StepApplicationProps) {
    const questions = state.selectedApp?.wizard?.application?.custom_questions || []

    const renderQuestion = (q: WizardQuestion) => {
        const value = state.appWizardAnswers[q.id]

        if (q.type === 'boolean') {
            return (
                <label className="flex items-center gap-3 p-3 rounded-lg border border-zinc-200 bg-white">
                    <input
                        type="checkbox"
                        className="h-4 w-4 accent-[#F38020]"
                        checked={Boolean(value)}
                        onChange={(e) => actions.setAppWizardAnswer(q.id, e.target.checked)}
                    />
                    <div className="flex-1">
                        <div className="text-sm font-medium text-zinc-900">{q.name}</div>
                        <div className="text-xs text-zinc-500">Used to auto-answer interactive setup.</div>
                    </div>
                </label>
            )
        }

        if (q.type === 'text') {
            return (
                <div className="p-3 rounded-lg border border-zinc-200 bg-white">
                    <label className="block text-sm font-medium text-zinc-900 mb-2">{q.name}</label>
                    <input
                        type="text"
                        className="w-full bg-white border border-zinc-200 rounded-lg px-3 py-2 text-zinc-900 focus:ring-2 focus:ring-[#F38020]/20 focus:border-[#F38020] outline-none transition-all placeholder:text-zinc-400"
                        value={typeof value === 'string' ? value : ''}
                        onChange={(e) => actions.setAppWizardAnswer(q.id, e.target.value)}
                        placeholder="Optional"
                    />
                </div>
            )
        }

        if (q.type === 'choice') {
            const choices = q.choices || []
            const defaults = new Set(choices.filter(c => c.default === true).map(c => c.name))
            const isMulti = defaults.size > 1

            if (isMulti) {
                const selected = new Set<string>(Array.isArray(value) ? value : Array.from(defaults))
                return (
                    <div className="p-3 rounded-lg border border-zinc-200 bg-white">
                        <div className="text-sm font-medium text-zinc-900 mb-2">{q.name}</div>
                        <div className="space-y-2">
                            {choices.map(c => (
                                <label key={c.name} className="flex items-center gap-3 text-sm text-zinc-700">
                                    <input
                                        type="checkbox"
                                        className="h-4 w-4 accent-[#F38020]"
                                        checked={selected.has(c.name)}
                                        onChange={(e) => {
                                            const next = new Set(selected)
                                            if (e.target.checked) next.add(c.name)
                                            else next.delete(c.name)
                                            actions.setAppWizardAnswer(q.id, Array.from(next))
                                        }}
                                    />
                                    {c.name}
                                </label>
                            ))}
                        </div>
                    </div>
                )
            }

            const current = typeof value === 'string' ? value : (choices.find(c => c.default === true)?.name ?? choices[0]?.name ?? '')
            return (
                <div className="p-3 rounded-lg border border-zinc-200 bg-white">
                    <div className="text-sm font-medium text-zinc-900 mb-2">{q.name}</div>
                    <div className="space-y-2">
                        {choices.map(c => (
                            <label key={c.name} className="flex items-center gap-3 text-sm text-zinc-700">
                                <input
                                    type="radio"
                                    name={q.id}
                                    className="h-4 w-4 accent-[#F38020]"
                                    checked={current === c.name}
                                    onChange={() => actions.setAppWizardAnswer(q.id, c.name)}
                                />
                                {c.name}
                            </label>
                        ))}
                    </div>
                </div>
            )
        }

        // Unknown type: ignore for now
        return null
    }

    return (
        <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div>
                <h2 className="text-lg font-medium text-zinc-900 mb-4">Select Application</h2>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {apps.map(app => {
                        const appLogo = getAppLogo(app.name)
                        return (
                            <SelectCard
                                key={app.name}
                                title={app.name}
                                description={app.description}
                                selected={state.appName === app.name}
                                onClick={() => actions.setAppName(app.name)}
                                icon={appLogo ? (
                                    <img src={appLogo} alt={app.name} className="w-8 h-8 object-contain" />
                                ) : (
                                    <Server size={20} />
                                )}
                            />
                        )
                    })}
                </div>
            </div>

            {state.appName && (
                <div className="animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <label className="block text-sm font-medium text-zinc-500 mb-2">Service Name</label>
                    <input
                        type="text"
                        className="w-full bg-white border border-zinc-200 rounded-lg px-4 py-3 text-zinc-900 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 outline-none transition-all placeholder:text-zinc-400"
                        placeholder="my-awesome-app"
                        value={state.serverName}
                        onChange={e => actions.setServerName(e.target.value)}
                    />
                </div>
            )}

            {state.appName && questions.length > 0 && (
                <div className="animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <h3 className="text-sm font-medium text-zinc-900 mb-2">Setup options</h3>
                    <p className="text-xs text-zinc-500 mb-3">
                        These will be used to help auto-answer the interactive installer (TTY) later. You can still type manually.
                    </p>
                    <div className="space-y-3">
                        {questions.map((q: WizardQuestion) => (
                            <div key={q.id}>{renderQuestion(q)}</div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    )
}

