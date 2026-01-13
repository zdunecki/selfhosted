import { Cloud, Globe, Shield, Info, Server, Check, Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { getAssetUrl } from '../../utils/api'
import type { WizardState, WizardActions } from './types'

interface StepDNSProps {
    state: WizardState
    actions: WizardActions
    providerName: string
}

export function StepDNS({ state, actions, providerName }: StepDNSProps) {
    const [showToken, setShowToken] = useState(false)
    return (
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div>
                <h2 className="text-lg font-medium text-zinc-900 mb-4">Domain Configuration</h2>
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-zinc-500 mb-3">DNS Strategy</label>
                        <div className="grid grid-cols-1 gap-3">
                            {/* Strategy 1: Automatic (Domain Based) */}
                            <div
                                onClick={(e) => {
                                    if ((e.target as HTMLElement).closest('input')) return
                                    actions.setDnsMode('auto')
                                }}
                                className={`p-4 rounded-xl border transition-all relative overflow-hidden cursor-pointer
                                    ${state.dnsMode === 'auto'
                                        ? 'bg-[#F38020]/10 border-[#F38020]/50 ring-1 ring-[#F38020]/50'
                                        : 'bg-white border-zinc-200 hover:border-[#F38020]/30'}
                                `}
                            >
                                <div className="flex items-start gap-4">
                                    <div className={`mt-1 p-1.5 rounded-lg flex items-center justify-center ${state.dnsMode === 'auto' ? 'bg-[#F38020] text-white' : 'bg-zinc-200 text-zinc-400'}`}>
                                        {state.detectedDomainProvider === 'cloudflare' && state.dnsMode === 'auto' ? (
                                            <img src={getAssetUrl('/cloudflare.svg')} alt="Cloudflare" className="w-4 h-4" />
                                        ) : (
                                            <Cloud size={16} />
                                        )}
                                    </div>
                                    <div className="flex-1">
                                        <div className="flex items-center gap-2 mb-1">
                                            <div className="font-medium text-zinc-900">Automatic (Domain Identity)</div>
                                            {state.detectedDomainProvider === 'cloudflare' && state.dnsMode === 'auto' && (
                                                <div className="flex items-center gap-1.5">
                                                    <img src={getAssetUrl('/cloudflare.svg')} alt="Cloudflare" className="w-4 h-4" />
                                                    <span className="text-[10px] bg-[#F38020] text-white px-1.5 py-0.5 rounded-full font-bold">CLOUDFLARE DETECTED</span>
                                                </div>
                                            )}
                                        </div>
                                        <div className="text-xs text-zinc-500 mb-3">
                                            {state.detectedDomainProvider === 'cloudflare'
                                                ? "We detected you are using Cloudflare. We will use the Cloudflare API to automatically configure DNS records."
                                                : state.detectedDomainProvider === 'other'
                                                    ? "We detected your DNS provider, but automatic configuration is not supported for this provider."
                                                    : "Enter your domain name below. We will check if you're using a supported DNS provider (like Cloudflare) and automatically configure DNS records if available."}
                                        </div>
                                        {state.dnsMode === 'auto' && (
                                            <div className="mt-3">
                                                <label className="block text-xs font-medium text-zinc-500 mb-2">Domain Name</label>
                                                <div className={`flex bg-white border rounded-lg overflow-hidden transition-all
                                                    ${!state.domainAuto && state.dnsMode === 'auto' ? 'border-yellow-300 ring-2 ring-yellow-100' : 'border-zinc-200 focus-within:ring-2 focus-within:ring-[#F38020]/20 focus-within:border-[#F38020]'}
                                                `}>
                                                    <div className="px-3 py-3 border-r border-zinc-200 bg-zinc-50 text-zinc-400">
                                                        <Globe size={18} />
                                                    </div>
                                                    <input
                                                        type="text"
                                                        className="flex-1 bg-transparent px-4 text-zinc-900 outline-none placeholder:text-zinc-400"
                                                        placeholder="app.example.com"
                                                        value={state.domainAuto}
                                                        onChange={e => actions.setDomainAuto(e.target.value)}
                                                        onClick={(e) => e.stopPropagation()}
                                                    />
                                                    {state.isCheckingDomain && (
                                                        <div className="px-3 py-3 text-zinc-400">
                                                            <div className="w-4 h-4 rounded-full border-2 border-zinc-300 border-t-zinc-600 animate-spin" />
                                                        </div>
                                                    )}
                                                </div>
                                                <p className="text-xs text-zinc-500 mt-2">
                                                    Required. We will check your nameservers to determine available automation.
                                                </p>
                                                {state.detectedDomainProvider === 'cloudflare' && (
                                                    <div className="mt-3">
                                                        <label className="block text-xs font-medium text-zinc-500 mb-2">Cloudflare API Token</label>
                                                        <div className="flex gap-2">
                                                            <div className={`flex-1 flex bg-white border rounded-lg overflow-hidden transition-all
                                                                ${!state.cloudflareToken && state.dnsMode === 'auto' ? 'border-yellow-300 ring-2 ring-yellow-100' : state.cloudflareTokenVerified ? 'border-green-300 ring-2 ring-green-100' : 'border-zinc-200 focus-within:ring-2 focus-within:ring-[#F38020]/20 focus-within:border-[#F38020]'}
                                                            `}>
                                                                <div className="px-3 py-3 border-r border-zinc-200 bg-zinc-50 text-zinc-400">
                                                                    <Shield size={18} />
                                                                </div>
                                                                <input
                                                                    type={showToken ? "text" : "password"}
                                                                    autoComplete="off"
                                                                    autoCorrect="off"
                                                                    autoCapitalize="off"
                                                                    spellCheck="false"
                                                                    name="cf-token"
                                                                    id="cf-token"
                                                                    data-form-type="other"
                                                                    data-1p-ignore
                                                                    data-lpignore="true"
                                                                    data-bwignore
                                                                    className="flex-1 bg-transparent px-4 text-zinc-900 outline-none placeholder:text-zinc-400 font-mono text-sm"
                                                                    placeholder="Enter your Cloudflare API token"
                                                                    value={state.cloudflareToken}
                                                                    onChange={e => actions.setCloudflareToken(e.target.value)}
                                                                    onClick={(e) => e.stopPropagation()}
                                                                    disabled={state.isVerifyingCloudflareToken}
                                                                />
                                                                {state.cloudflareToken && (
                                                                    <button
                                                                        type="button"
                                                                        onClick={(e) => {
                                                                            e.stopPropagation()
                                                                            setShowToken(!showToken)
                                                                        }}
                                                                        className="px-3 py-3 text-zinc-400 hover:text-zinc-600 transition-colors"
                                                                        tabIndex={-1}
                                                                    >
                                                                        {showToken ? <EyeOff size={18} /> : <Eye size={18} />}
                                                                    </button>
                                                                )}
                                                                {state.cloudflareTokenVerified && (
                                                                    <div className="px-3 py-3 text-green-600">
                                                                        <Check size={18} />
                                                                    </div>
                                                                )}
                                                            </div>
                                                            <button
                                                                onClick={(e) => {
                                                                    e.stopPropagation()
                                                                    actions.handleVerifyCloudflareToken()
                                                                }}
                                                                disabled={!state.cloudflareToken.trim() || state.isVerifyingCloudflareToken || state.cloudflareTokenVerified}
                                                                className={`
                                                                    px-4 py-3 rounded-lg text-sm font-medium transition-colors whitespace-nowrap
                                                                    ${state.cloudflareTokenVerified
                                                                        ? 'bg-green-50 text-green-700 border border-green-200 cursor-not-allowed'
                                                                        : state.isVerifyingCloudflareToken
                                                                            ? 'bg-zinc-100 text-zinc-400 cursor-not-allowed'
                                                                            : state.cloudflareToken.trim()
                                                                                ? 'bg-[#F38020] text-white hover:bg-[#F38020]/90'
                                                                                : 'bg-zinc-100 text-zinc-400 cursor-not-allowed'
                                                                    }
                                                                `}
                                                            >
                                                                {state.isVerifyingCloudflareToken ? (
                                                                    <div className="flex items-center gap-2">
                                                                        <div className="w-4 h-4 rounded-full border-2 border-current border-t-transparent animate-spin" />
                                                                        Verifying...
                                                                    </div>
                                                                ) : state.cloudflareTokenVerified ? (
                                                                    <div className="flex items-center gap-2">
                                                                        <Check size={16} />
                                                                        Verified
                                                                    </div>
                                                                ) : (
                                                                    'Verify'
                                                                )}
                                                            </button>
                                                        </div>
                                                        <div className="mt-3">
                                                            <label className="block text-xs font-medium text-zinc-500 mb-2">
                                                                Account ID <span className="text-zinc-400 font-normal">(Optional)</span>
                                                            </label>
                                                            <div className="flex bg-white border border-zinc-200 rounded-lg overflow-hidden transition-all focus-within:ring-2 focus-within:ring-[#F38020]/20 focus-within:border-[#F38020]">
                                                                <div className="px-3 py-3 border-r border-zinc-200 bg-zinc-50 text-zinc-400">
                                                                    <Globe size={18} />
                                                                </div>
                                                                <input
                                                                    type="text"
                                                                    className="flex-1 bg-transparent px-4 text-zinc-900 outline-none placeholder:text-zinc-400"
                                                                    placeholder="e.g., 38506b7d3508e99f1165509b2344237b"
                                                                    value={state.cloudflareAccountId}
                                                                    onChange={e => actions.setCloudflareAccountId(e.target.value)}
                                                                    onClick={(e) => e.stopPropagation()}
                                                                    disabled={state.isVerifyingCloudflareToken}
                                                                />
                                                            </div>
                                                            <p className="text-xs text-zinc-500 mt-2">
                                                                Optional. If your token is scoped to a specific account, enter the account ID here.
                                                            </p>
                                                        </div>
                                                        {state.cloudflareTokenError && (
                                                            <div className="mt-2 bg-red-50 border border-red-200 rounded-lg p-3 flex gap-2">
                                                                <div className="shrink-0 mt-0.5">
                                                                    <Info size={14} className="text-red-600" />
                                                                </div>
                                                                <p className="text-xs text-red-700 flex-1">
                                                                    {state.cloudflareTokenError}
                                                                </p>
                                                            </div>
                                                        )}
                                                        {state.cloudflareTokenVerified ? (
                                                            <p className="text-xs text-green-600 mt-2 flex items-center gap-1">
                                                                <Check size={12} />
                                                                Token verified successfully
                                                            </p>
                                                        ) : !state.cloudflareTokenError && (
                                                            <p className="text-xs text-zinc-500 mt-2">
                                                                Required. Create a token with <span className="font-medium">Zone.DNS</span> permissions at{' '}
                                                                <a 
                                                                    href="https://dash.cloudflare.com/profile/api-tokens" 
                                                                    target="_blank" 
                                                                    rel="noopener noreferrer"
                                                                    className="text-[#F38020] hover:underline"
                                                                    onClick={(e) => e.stopPropagation()}
                                                                >
                                                                    Cloudflare Dashboard
                                                                </a>
                                                            </p>
                                                        )}
                                                    </div>
                                                )}
                                                {state.detectedDomainProvider === 'other' && (
                                                    <div className="mt-3 bg-yellow-50 border border-yellow-200 rounded-lg p-3 flex gap-3">
                                                        <div className="shrink-0 mt-0.5">
                                                            <Info size={16} className="text-yellow-600" />
                                                        </div>
                                                        <div className="flex-1">
                                                            <p className="text-xs font-medium text-yellow-800 mb-1">
                                                                DNS Provider Not Supported
                                                            </p>
                                                            <p className="text-xs text-yellow-700">
                                                                We don't support automatic configuration for your DNS provider. Supported providers for automatic mode: <span className="font-medium">Cloudflare</span>.
                                                            </p>
                                                        </div>
                                                    </div>
                                                )}
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </div>

                            {/* Strategy 2: Cloud Provider */}
                            <div
                                onClick={(e) => {
                                    if ((e.target as HTMLElement).closest('input')) return
                                    actions.setDnsMode('provider')
                                }}
                                className={`p-4 rounded-xl border transition-all
                                    ${state.dnsMode === 'provider'
                                        ? 'bg-blue-50 border-blue-200 ring-1 ring-blue-200'
                                        : 'bg-white border-zinc-200 hover:border-zinc-300 hover:bg-zinc-50 cursor-pointer'}
                                `}
                            >
                                <div className="flex items-start gap-4">
                                    <div className={`mt-1 p-1.5 rounded-lg ${state.dnsMode === 'provider' ? 'bg-blue-500 text-white' : 'bg-zinc-100 text-zinc-400'}`}>
                                        <Server size={16} />
                                    </div>
                                    <div className="flex-1">
                                        <div className="font-medium text-zinc-900 mb-1">{providerName} DNS</div>
                                        <div className="text-xs text-zinc-500 mb-3">
                                            Use {providerName}'s managed DNS service. You will need to update your domain's nameservers to point to {providerName}.
                                        </div>
                                        {state.dnsMode === 'provider' && (
                                            <div className="mt-3">
                                                <label className="block text-xs font-medium text-zinc-500 mb-2">Domain Name</label>
                                                <div className={`flex bg-white border rounded-lg overflow-hidden transition-all
                                                    ${!state.domainProvider && state.dnsMode === 'provider' ? 'border-yellow-300 ring-2 ring-yellow-100' : 'border-zinc-200 focus-within:ring-2 focus-within:ring-blue-500/20 focus-within:border-blue-500'}
                                                `}>
                                                    <div className="px-3 py-3 border-r border-zinc-200 bg-zinc-50 text-zinc-400">
                                                        <Globe size={18} />
                                                    </div>
                                                    <input
                                                        type="text"
                                                        className="flex-1 bg-transparent px-4 text-zinc-900 outline-none placeholder:text-zinc-400"
                                                        placeholder="app.example.com"
                                                        value={state.domainProvider}
                                                        onChange={e => actions.setDomainProvider(e.target.value)}
                                                        onClick={(e) => e.stopPropagation()}
                                                    />
                                                </div>
                                                <p className="text-xs text-zinc-500 mt-2">
                                                    Required. We will check your nameservers to determine available automation.
                                                </p>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </div>

                            {/* Strategy 3: Manual */}
                            <div
                                onClick={() => actions.setDnsMode('manual')}
                                className={`p-4 rounded-xl border cursor-pointer transition-all flex items-start gap-4
                                    ${state.dnsMode === 'manual'
                                        ? 'bg-purple-50 border-purple-200 ring-1 ring-purple-200'
                                        : 'bg-white border-zinc-200 hover:border-zinc-300 hover:bg-zinc-50'}
                                `}
                            >
                                <div className={`mt-1 p-1.5 rounded-lg ${state.dnsMode === 'manual' ? 'bg-purple-500 text-white' : 'bg-zinc-100 text-zinc-400'}`}>
                                    <Shield size={16} />
                                </div>
                                <div>
                                    <div className="font-medium text-zinc-900 mb-1">Manual Configuration</div>
                                    <div className="text-xs text-zinc-500">
                                        You will be given the IP address and required records to configure in your DNS provider's dashboard manually.
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}

