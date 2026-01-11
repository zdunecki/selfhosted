import { Server } from 'lucide-react'
import { SelectCard } from '../../components/SelectCard'
import type { App, WizardState, WizardActions } from './types'

interface StepApplicationProps {
    apps: App[]
    state: WizardState
    actions: WizardActions
    getAppLogo: (name: string) => string | undefined
}

export function StepApplication({ apps, state, actions, getAppLogo }: StepApplicationProps) {
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
        </div>
    )
}

