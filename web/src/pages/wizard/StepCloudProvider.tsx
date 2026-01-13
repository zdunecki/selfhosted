import { Cloud } from 'lucide-react'
import { SelectCard } from '../../components/SelectCard'
import type { Provider, WizardState, WizardActions } from './types'
import { useEffect, useRef, useState } from 'react'

interface StepCloudProviderProps {
    providers: Provider[]
    state: WizardState
    actions: WizardActions
    getProviderLogo: (name: string) => string | undefined
}

export function StepCloudProvider({ providers, state, actions, getProviderLogo }: StepCloudProviderProps) {
    const regionSectionRef = useRef<HTMLDivElement>(null)
    const sizeSectionRef = useRef<HTMLDivElement>(null)
    const prevProviderRef = useRef<string>('')
    const prevRegionRef = useRef<string>('')
    const [showAllSizes, setShowAllSizes] = useState(true)

    // UX: auto-scroll to newly revealed sections inside the fixed-height installer
    useEffect(() => {
        if (state.providerName && state.providerName !== prevProviderRef.current && !state.showConfig) {
            prevProviderRef.current = state.providerName
            setTimeout(() => regionSectionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 50)
        }
    }, [state.providerName, state.showConfig])

    useEffect(() => {
        if (state.region && state.region !== prevRegionRef.current && !state.showConfig) {
            prevRegionRef.current = state.region
            setShowAllSizes(true) // keep "show all" by default
            setTimeout(() => sizeSectionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 50)
        }
    }, [state.region, state.showConfig])

    return (
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
            {/* Provider */}
            <div>
                <h2 className="text-lg font-medium text-zinc-900 mb-4">Cloud Infrastructure</h2>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {providers.map(provider => {
                        const providerLogo = getProviderLogo(provider.name)
                        return (
                            <SelectCard
                                key={provider.name}
                                title={provider.name}
                                description={provider.description}
                                selected={state.providerName === provider.name}
                                onClick={() => {
                                    actions.setProviderName(provider.name)
                                    actions.setRegion('')
                                    actions.setSize('')
                                    actions.setConfigToken('')
                                    actions.setShowConfig(provider.needs_config || false)
                                }}
                                icon={providerLogo ? (
                                    <img src={providerLogo} alt={provider.name} className="w-8 h-8 object-contain" />
                                ) : (
                                    <Cloud size={20} />
                                )}
                            />
                        )
                    })}
                </div>
            </div>

            {/* API Token Config */}
            {state.providerName && state.showConfig && (
                <div className="bg-yellow-50 border border-yellow-200 rounded-xl p-6">
                    <h3 className="text-yellow-800 font-medium mb-2">Authentication Required</h3>
                    <p className="text-sm text-yellow-700 mb-4">
                        {state.providerName.toLowerCase() === 'scaleway'
                            ? 'Paste your Scaleway JSON config to continue (access_key, secret_key, project_id, optional organization_id, optional zone).'
                            : state.providerName.toLowerCase() === 'gcp'
                                ? 'Use ADC on the machine running SelfHosted (run: gcloud auth application-default login) OR paste your GCP Service Account JSON. (Enterprise: you may also need billing_account + parent.)'
                            : `Enter your ${state.providerName} API token to continue.`}
                    </p>
                    <div className="flex gap-3">
                        {state.providerName.toLowerCase() === 'scaleway' || state.providerName.toLowerCase() === 'gcp' ? (
                            <textarea
                                className="flex-1 bg-white border border-yellow-300 rounded-lg px-4 py-2 text-zinc-900 focus:ring-2 focus:ring-yellow-500/20 outline-none font-mono text-xs min-h-[90px]"
                                placeholder={state.providerName.toLowerCase() === 'gcp'
                                    ? '{\n  "type": "service_account",\n  "project_id": "...",\n  "private_key_id": "...",\n  "private_key": "-----BEGIN PRIVATE KEY-----\\n...\\n-----END PRIVATE KEY-----\\n",\n  "client_email": "...",\n  "client_id": "..." \n}'
                                    : '{"access_key":"SCW...","secret_key":"...","project_id":"...","zone":"fr-par-1"}'}
                                value={state.configToken}
                                onChange={e => actions.setConfigToken(e.target.value)}
                            />
                        ) : (
                            <input
                                type="password"
                                className="flex-1 bg-white border border-yellow-300 rounded-lg px-4 py-2 text-zinc-900 focus:ring-2 focus:ring-yellow-500/20 outline-none"
                                placeholder="API Token"
                                value={state.configToken}
                                onChange={e => actions.setConfigToken(e.target.value)}
                            />
                        )}
                        <button
                            onClick={actions.handleSaveToken}
                            className="px-4 py-2 bg-yellow-600 hover:bg-yellow-500 text-white rounded-lg text-sm font-medium transition-colors"
                        >
                            Verify
                        </button>
                    </div>
                </div>
            )}

            {/* Region */}
            {state.providerName && !state.showConfig && (
                <div ref={regionSectionRef} className="animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <h2 className="text-lg font-medium text-zinc-900 mb-4">Region</h2>

                    {/* GCP project mode + billing/project selection */}
                    {state.providerName.toLowerCase() === 'gcp' && (
                        <div className="mb-6 bg-white border border-zinc-200 rounded-xl p-4">
                            <div className="flex items-start justify-between gap-4">
                                <div>
                                    <div className="text-sm font-medium text-zinc-900">GCP project</div>
                                    <div className="text-xs text-zinc-600 mt-1">
                                        Choose whether to deploy into an <span className="font-medium">existing project</span> (recommended) or create a <span className="font-medium">new project</span>.
                                    </div>
                                </div>
                                <button
                                    onClick={actions.handleSaveGcpProjectSelection}
                                    className="px-3 py-2 bg-zinc-900 hover:bg-zinc-800 text-white rounded-lg text-xs font-medium transition-colors"
                                >
                                    Save
                                </button>
                            </div>

                            <div className="mt-3 flex gap-3">
                                <button
                                    onClick={() => actions.setGcpProjectMode('existing')}
                                    className={`px-3 py-2 rounded-lg text-xs font-medium border transition-colors ${
                                        state.gcpProjectMode === 'existing'
                                            ? 'bg-zinc-900 text-white border-zinc-900'
                                            : 'bg-white text-zinc-900 border-zinc-300 hover:bg-zinc-50'
                                    }`}
                                >
                                    Use existing
                                </button>
                                <button
                                    onClick={() => actions.setGcpProjectMode('new')}
                                    className={`px-3 py-2 rounded-lg text-xs font-medium border transition-colors ${
                                        state.gcpProjectMode === 'new'
                                            ? 'bg-zinc-900 text-white border-zinc-900'
                                            : 'bg-white text-zinc-900 border-zinc-300 hover:bg-zinc-50'
                                    }`}
                                >
                                    Create new
                                </button>
                            </div>

                            {state.gcpProjectMode === 'existing' ? (
                                state.gcpProjectsLoading ? (
                                    <div className="mt-3 text-zinc-500 text-sm flex items-center gap-2">
                                        <div className="w-4 h-4 rounded-full border-2 border-zinc-300 border-t-zinc-600 animate-spin" />
                                        Fetching projects...
                                    </div>
                                ) : state.gcpProjectsError ? (
                                    <div className="mt-3 text-sm text-red-700">
                                        Failed to load projects: {state.gcpProjectsError}
                                    </div>
                                ) : (
                                    <div className="mt-3 flex gap-3">
                                        <div className="w-full">
                                            <div className="text-xs font-medium text-zinc-700 mb-1">Project (ACTIVE only)</div>
                                        <select
                                            className="flex-1 bg-white border border-zinc-300 rounded-lg px-3 py-2 text-zinc-900 focus:ring-2 focus:ring-zinc-500/20 outline-none text-sm"
                                            value={state.gcpProjectId}
                                            onChange={e => actions.setGcpProjectId(e.target.value)}
                                        >
                                            <option value="">Select project…</option>
                                            {state.gcpProjects.map(p => (
                                                <option key={p.projectID} value={p.projectID}>
                                                    {p.displayName || p.projectID} ({p.projectID})
                                                </option>
                                            ))}
                                        </select>
                                        </div>
                                    </div>
                                )
                            ) : (
                                state.gcpBillingAccountsLoading ? (
                                    <div className="mt-3 text-zinc-500 text-sm flex items-center gap-2">
                                        <div className="w-4 h-4 rounded-full border-2 border-zinc-300 border-t-zinc-600 animate-spin" />
                                        Fetching billing accounts...
                                    </div>
                                ) : state.gcpBillingAccountsError ? (
                                    <div className="mt-3 text-sm text-red-700">
                                        Failed to load billing accounts: {state.gcpBillingAccountsError}
                                    </div>
                                ) : (
                                    <div className="mt-3 flex gap-3">
                                        <div className="w-full">
                                            <div className="text-xs font-medium text-zinc-700 mb-1">Billing account (for new project)</div>
                                        <select
                                            className="flex-1 bg-white border border-zinc-300 rounded-lg px-3 py-2 text-zinc-900 focus:ring-2 focus:ring-zinc-500/20 outline-none text-sm"
                                            value={state.gcpBillingAccount}
                                            onChange={e => actions.setGcpBillingAccount(e.target.value)}
                                        >
                                            <option value="">Select billing account…</option>
                                            {state.gcpBillingAccounts.map(a => (
                                                <option key={a.name} value={a.name}>
                                                    {a.display_name || a.name} {a.open ? '' : '(closed)'}
                                                </option>
                                            ))}
                                        </select>
                                        </div>
                                    </div>
                                )
                            )}
                        </div>
                    )}

                    {state.regionsLoading ? (
                        <div className="text-zinc-500 text-sm flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full border-2 border-zinc-300 border-t-zinc-600 animate-spin" />
                            Fetching regions...
                        </div>
                    ) : (
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                            {state.regions.map(r => {
                                const slug = (r.slug || '').toLowerCase()
                                let flag = '🌍'

                                // UpCloud/Scaleway are <cc>-<city><n> or <cc>-<city>-<n>, e.g. nl-ams1, pl-waw-1, fr-par-1
                                if (slug.startsWith('nl-') || slug.includes('nl-ams')) flag = '🇳🇱'
                                else if (slug.startsWith('pl-') || slug.includes('pl-waw')) flag = '🇵🇱'
                                else if (slug.startsWith('fr-') || slug.includes('fr-par')) flag = '🇫🇷'
                                else if (slug.startsWith('de-') || slug.includes('de-fra')) flag = '🇩🇪'
                                else if (slug.startsWith('fi-') || slug.includes('fi-hel')) flag = '🇫🇮'
                                else if (slug.startsWith('uk-') || slug.includes('uk-lon') || slug.includes('gb-lon')) flag = '🇬🇧'
                                else if (slug.startsWith('us-') || slug.includes('us-')) flag = '🇺🇸'

                                // DigitalOcean style (kept for backward compatibility): ams3, fra1, nyc3...
                                else if (slug.startsWith('nyc') || slug.startsWith('sfo')) flag = '🇺🇸'
                                else if (slug.startsWith('ams')) flag = '🇳🇱'
                                else if (slug.startsWith('lon')) flag = '🇬🇧'
                                else if (slug.startsWith('fra')) flag = '🇩🇪'
                                else if (slug.startsWith('tor')) flag = '🇨🇦'
                                else if (slug.startsWith('sgp')) flag = '🇸🇬'
                                else if (slug.startsWith('blr')) flag = '🇮🇳'
                                else if (slug.startsWith('syd')) flag = '🇦🇺'

                                return (
                                    <div
                                        key={r.slug}
                                        onClick={() => actions.setRegion(r.slug)}
                                        className={`
                                            cursor-pointer p-3 rounded-lg border transition-all
                                            ${state.region === r.slug
                                                ? 'bg-blue-50 border-blue-200 ring-1 ring-blue-200 shadow-sm'
                                                : 'bg-white border-zinc-200 hover:border-zinc-300 hover:bg-zinc-50'}
                                        `}
                                    >
                                        <div className="text-xl mb-1">{flag}</div>
                                        <div className={`text-sm font-medium ${state.region === r.slug ? 'text-blue-700' : 'text-zinc-700'}`}>
                                            {r.name.split(' ')[0]}
                                        </div>
                                        <div className="text-xs text-zinc-500 truncate">{r.slug}</div>
                                    </div>
                                )
                            })}
                        </div>
                    )}
                </div>
            )}

            {/* Size */}
            {state.region && !state.showConfig && (
                <div ref={sizeSectionRef} className="animate-in fade-in slide-in-from-bottom-2 duration-300">
                    <div className="flex items-center justify-between gap-3 mb-4">
                        <h2 className="text-lg font-medium text-zinc-900">Hardware Config</h2>
                        {state.sizes.length > 9 && (
                            <button
                                type="button"
                                onClick={() => setShowAllSizes(v => !v)}
                                className="text-xs font-medium text-zinc-600 hover:text-zinc-900 transition-colors"
                            >
                                {showAllSizes ? 'Show top 9' : `Show all (${state.sizes.length})`}
                            </button>
                        )}
                    </div>
                    {state.sizesLoading ? (
                        <div className="text-zinc-500 text-sm flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full border-2 border-zinc-300 border-t-zinc-600 animate-spin" />
                            Loading sizes...
                        </div>
                    ) : (
                        <div className={`
                            ${showAllSizes ? 'max-h-[420px] pr-1' : ''}
                        `}>
                            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            {(() => {
                                const recommendedSize = state.selectedApp
                                    ? state.sizes.find(s => s.vcpus >= (state.selectedApp?.min_cpus || 1) && s.memory >= (state.selectedApp?.min_memory || 512))
                                    : undefined;

                                const sortedSizes = [...state.sizes].sort((a, b) => {
                                    if (recommendedSize && a.slug === recommendedSize.slug) return -1;
                                    if (recommendedSize && b.slug === recommendedSize.slug) return 1;
                                    return a.price_monthly - b.price_monthly;
                                });

                                const visibleSizes = showAllSizes ? sortedSizes : sortedSizes.slice(0, 9);

                                return visibleSizes.map(s => {
                                    const isRecommended = recommendedSize && s.slug === recommendedSize.slug;
                                    return (
                                        <div
                                            key={s.slug}
                                            onClick={() => actions.setSize(s.slug)}
                                            className={`
                                                relative p-4 rounded-xl border cursor-pointer transition-all
                                                ${state.size === s.slug
                                                    ? 'bg-blue-50 border-blue-200 ring-1 ring-blue-200 shadow-sm'
                                                    : 'bg-white border-zinc-200 hover:border-zinc-300 hover:bg-zinc-50'}
                                            `}
                                        >
                                            {isRecommended && (
                                                <div className="absolute -top-2.5 left-4 bg-green-100 text-green-700 text-[10px] uppercase font-bold px-2 py-0.5 rounded-full border border-green-200">
                                                    Minimum
                                                </div>
                                            )}
                                            <div className="flex justify-between items-start mb-2">
                                                <div className="text-sm font-mono text-zinc-500">{s.slug}</div>
                                                <div className="text-zinc-900 font-medium">
                                                    ${Number(s.price_monthly).toFixed(2)}
                                                    <span className="text-zinc-500 text-xs">/mo</span>
                                                </div>
                                            </div>
                                            <div className="text-xs text-zinc-500 flex gap-3">
                                                <span>{s.vcpus} vCPU</span>
                                                <span>{s.memory}MB RAM</span>
                                                <span>{s.disk}GB SSD</span>
                                            </div>
                                        </div>
                                    )
                                })
                            })()}
                            </div>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}

