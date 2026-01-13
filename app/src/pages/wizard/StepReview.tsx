import { Info } from 'lucide-react'
import type { Region, Size } from '../../types'
import type { WizardState } from './types'

interface StepReviewProps {
    state: WizardState
    regions: Region[]
    sizes: Size[]
}

export function StepReview({ state, regions, sizes }: StepReviewProps) {
    const selectedSize = sizes.find(s => s.slug === state.size)
    const priceMonthly = selectedSize ? Number(selectedSize.price_monthly) : 0

    return (
        <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
            <h2 className="text-lg font-medium text-zinc-900">Review & Deploy</h2>

            <div className="bg-zinc-50 border border-zinc-200 rounded-xl p-6">
                <h3 className="text-sm font-medium text-zinc-500 uppercase tracking-wider mb-4">Deployment Configuration</h3>
                <dl className="grid grid-cols-2 gap-x-4 gap-y-4 text-sm">
                    <div className="col-span-1">
                        <dt className="text-zinc-500">Application</dt>
                        <dd className="text-zinc-900 font-medium">{state.appName}</dd>
                    </div>
                    <div className="col-span-1">
                        <dt className="text-zinc-500">Service Name</dt>
                        <dd className="text-zinc-900 font-medium">{state.serverName}</dd>
                    </div>
                    <div className="col-span-1">
                        <dt className="text-zinc-500">Provider</dt>
                        <dd className="text-zinc-900 font-medium">{state.providerName}</dd>
                    </div>
                    <div className="col-span-1">
                        <dt className="text-zinc-500">Region</dt>
                        <dd className="text-zinc-900 font-medium">{regions.find(r => r.slug === state.region)?.name || state.region}</dd>
                    </div>
                    <div className="col-span-1">
                        <dt className="text-zinc-500">Domain</dt>
                        <dd className="text-zinc-900 font-medium">
                            {state.dnsMode === 'auto' ? state.domainAuto : state.dnsMode === 'provider' ? state.domainProvider : 'None'}
                        </dd>
                    </div>
                    <div className="col-span-1">
                        <dt className="text-zinc-500">DNS Mode</dt>
                        <dd className="text-zinc-900 font-medium capitalize">{state.detectedDomainProvider === 'cloudflare' && state.dnsMode === 'auto' ? 'Cloudflare' : state.dnsMode}</dd>
                    </div>
                    {state.detectedDomainProvider === 'cloudflare' && state.dnsMode === 'auto' && (
                        <div className="col-span-1">
                            <dt className="text-zinc-500">Cloudflare Token</dt>
                            <dd className="text-zinc-900 font-medium">{state.cloudflareToken ? '✓ Configured' : '✗ Not set'}</dd>
                        </div>
                    )}
                    <div className="col-span-1">
                        <dt className="text-zinc-500">Plan</dt>
                        <dd className="text-zinc-900 font-medium">
                            {state.size} (${priceMonthly.toFixed(2)}/mo)
                        </dd>
                    </div>
                    <div className="col-span-2 pt-2 border-t border-zinc-200 mt-2">
                        <dt className="text-zinc-500">Total Monthly Cost</dt>
                        <dd className="text-green-600 font-bold text-lg">${priceMonthly.toFixed(2)}</dd>
                    </div>
                </dl>
            </div>

            <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 flex gap-3 text-sm text-blue-800">
                <div className="shrink-0 mt-0.5"><Info size={16} /></div>
                <p>
                    Upon deployment, we will provision a new server, install the runtime, and configure your application. This process typically takes 3-5 minutes.
                </p>
            </div>
        </div>
    )
}

