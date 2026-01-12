import { useState, useEffect } from 'react'
import { useWizardData } from '../hooks/useWizardData'
import { InstallerLayout, type Step } from '../components/InstallerLayout'
import { StepApplication } from './wizard/StepApplication'
import { StepCloudProvider } from './wizard/StepCloudProvider'
import { StepDNS } from './wizard/StepDNS'
import { StepReview } from './wizard/StepReview'
import { StepInstallation } from './wizard/StepInstallation'
import type { WizardState, WizardActions, Region, Size } from './wizard/types'
import { encryptForServer } from '../utils/crypto'

// Step definitions
const STEPS: Step[] = [
    { id: 'app', title: 'Application', description: 'Choose software' },
    { id: 'cloud', title: 'Cloud Provider', description: 'Select infrastructure' },
    { id: 'dns', title: 'Network & DNS', description: 'Configure domain' },
    { id: 'review', title: 'Review', description: 'Confirm your choices' },
    { id: 'install', title: 'Installation', description: 'Deploy service' },
]

export function Wizard() {
    const { apps, providers } = useWizardData()

    // Form State
    const [appName, setAppName] = useState<string>('')
    const [serverName, setServerName] = useState<string>('')
    const [providerName, setProviderName] = useState<string>('')
    const [configToken, setConfigToken] = useState('')
    const [showConfig, setShowConfig] = useState(false) // New state for showing config
    const [region, setRegion] = useState<string>('')
    const [size, setSize] = useState<string>('')
    const [domainAuto, setDomainAuto] = useState<string>('')
    const [domainProvider, setDomainProvider] = useState<string>('')
    const [dnsMode, setDnsMode] = useState<'auto' | 'provider' | 'manual'>('auto')
    const [isCheckingDomain, setIsCheckingDomain] = useState(false)
    const [detectedDomainProvider, setDetectedDomainProvider] = useState<string | null>(null)
    const [cloudflareToken, setCloudflareToken] = useState<string>('')
    const [cloudflareAccountId, setCloudflareAccountId] = useState<string>('')
    const [isVerifyingCloudflareToken, setIsVerifyingCloudflareToken] = useState(false)
    const [cloudflareTokenVerified, setCloudflareTokenVerified] = useState(false)
    const [cloudflareTokenError, setCloudflareTokenError] = useState<string | null>(null)
    const [appWizardAnswers, setAppWizardAnswers] = useState<Record<string, any>>({})
    const [, setProviderHasCreds] = useState<boolean | null>(null)

    // Wizard State
    const [currentStepIndex, setCurrentStepIndex] = useState(0)

    // Deployment State
    const [deploying, setDeploying] = useState(false)
    const [logs, setLogs] = useState<string[]>([])
    const [deployError, setDeployError] = useState<string | null>(null)
    const [showFullLogs, setShowFullLogs] = useState(false)
    const [deployComplete, setDeployComplete] = useState(false)
    const [deployedUrl, setDeployedUrl] = useState<string>('')
    const [ptySessionId, setPtySessionId] = useState<string>('')
    const [ptyChunksB64, setPtyChunksB64] = useState<string[]>([])
    const [ttyAutoAnswering, setTtyAutoAnswering] = useState(false)

    // Data Hooks (now local state)
    const [regions, setRegions] = useState<Region[]>([])
    const [sizes, setSizes] = useState<Size[]>([])
    const [regionsLoading, setRegionsLoading] = useState(false)
    const [sizesLoading, setSizesLoading] = useState(false)

    // Derived State
    const selectedApp = apps.find(a => a.name === appName)
    const selectedProvider = providers.find(p => p.name === providerName)

    // Initialize/refresh default answers when app changes
    useEffect(() => {
        const qs = selectedApp?.wizard?.application?.custom_questions || []
        if (!selectedApp || qs.length === 0) {
            setAppWizardAnswers({})
            return
        }

        const defaults: Record<string, any> = {}
        for (const q of qs) {
            if (!q?.id) continue
            if (q.type === 'boolean') {
                defaults[q.id] = typeof q.default === 'boolean' ? q.default : true
            } else if (q.type === 'text') {
                defaults[q.id] = typeof q.default === 'string' ? q.default : ''
            } else if (q.type === 'choice') {
                const choices = q.choices || []
                const trueDefaults = choices.filter(c => c.default === true).map(c => c.name)
                if (trueDefaults.length > 1) {
                    defaults[q.id] = trueDefaults
                } else {
                    const def = choices.find(c => c.default === true)?.name
                    defaults[q.id] = def ?? (choices[0]?.name ?? '')
                }
            }
        }
        setAppWizardAnswers(prev => ({ ...defaults, ...prev }))
    }, [selectedApp])
    
    // Helper function to get app logo
    const getAppLogo = (name: string): string | undefined => {
        const logoMap: Record<string, string> = {
            'openreplay': '/openreplay.svg',
            'openpanel': '/openpanel.svg',
            'plausible': '/plausible.svg',
            'umami': '/umami.svg',
            'swetrix': '/swetrix.png',
            'rybbit': '/rybbit.svg',
        }
        return logoMap[name.toLowerCase()]
    }
    
    // Helper function to get provider logo
    const getProviderLogo = (name: string): string | undefined => {
        const logoMap: Record<string, string> = {
            'digitalocean': '/digitalocean.svg',
            'scaleway': '/scaleway.svg',
            'upcloud': '/upcloud.svg',
            'vultr': '/vultr.svg',
            'gcp': '/gcloud.svg',
        }
        return logoMap[name.toLowerCase()]
    }

    // Effects for fetching regions/sizes
    useEffect(() => {
        if (providerName) {
            // Ask backend if credentials already exist locally (so UI doesn't force auth).
            setProviderHasCreds(null)
            fetch(`/api/providers/check?provider=${providerName}`)
                .then(res => (res.ok ? res.json() : null))
                .then(data => {
                    const hasCreds = Boolean(data?.hasCredentials)
                    setProviderHasCreds(hasCreds)
                    if (!hasCreds && !configToken) {
                        setShowConfig(true)
                        setRegions([])
                        setSizes([])
                        return
                    }
                    setShowConfig(false)
                })
                .catch(() => {
                    // Fallback to previous behavior if check endpoint fails
                    const providerNeedsConfig = selectedProvider?.needs_config
                    if (providerNeedsConfig && !configToken) {
                        setShowConfig(true)
                        setRegions([])
                        setSizes([])
                        return
                    }
                    setShowConfig(false)
                })

            setRegionsLoading(true)
            fetch(`/api/regions?provider=${providerName}`)
                .then(res => {
                    if (!res.ok) {
                        if (res.status === 401) { // Unauthorized, likely token issue
                            setShowConfig(true)
                            throw new Error('Authentication required for this provider.')
                        }
                        throw new Error('Failed to fetch regions')
                    }
                    return res.json()
                })
                .then(data => setRegions(data || []))
                .catch(err => {
                    console.error(err)
                    setRegions([])
                })
                .finally(() => setRegionsLoading(false))
        } else {
            setRegions([])
            setRegion('')
        }
    }, [providerName, configToken, selectedProvider]) // Added configToken and selectedProvider to dependencies

    useEffect(() => {
        if (providerName && region && !showConfig) {
            // Region/zone affects available sizes for some providers (e.g. Scaleway).
            setSizes([])
            setSize('')
            setSizesLoading(true)
            fetch(`/api/sizes?provider=${providerName}&region=${encodeURIComponent(region)}`)
                .then(res => {
                    if (!res.ok) {
                        if (res.status === 401) { // Unauthorized, likely token issue
                            setShowConfig(true)
                            throw new Error('Authentication required for this provider.')
                        }
                        throw new Error('Failed to fetch sizes')
                    }
                    return res.json()
                })
                .then(data => setSizes(data || []))
                .catch(err => {
                    console.error(err)
                    setSizes([])
                })
                .finally(() => setSizesLoading(false))
        } else {
            setSizes([])
            setSize('')
        }
    }, [providerName, region, showConfig])

    // Domain Check Logic for Auto mode
    useEffect(() => {
        const checkDomain = async () => {
            if (!domainAuto || !domainAuto.includes('.')) {
                setDetectedDomainProvider(null)
                return
            }

            setIsCheckingDomain(true)
            try {
                const res = await fetch(`/api/domains/check?domain=${domainAuto}`)
                const data = await res.json()
                if (data.provider === 'cloudflare') {
                    setDetectedDomainProvider('cloudflare')
                } else if (data.provider === 'other') {
                    setDetectedDomainProvider('other')
                } else {
                    setDetectedDomainProvider(null)
                }
            } catch (err) {
                console.error("Domain check failed", err)
                setDetectedDomainProvider(null)
            } finally {
                setIsCheckingDomain(false)
            }
        }

        const timeoutId = setTimeout(checkDomain, 1000)
        return () => clearTimeout(timeoutId)
    }, [domainAuto])

    // Auto-fill server name when app changes
    useEffect(() => {
        if (appName && !serverName) {
            setServerName(`${appName}-server`)
        }
    }, [appName, serverName])


    // Step Validation
    const canProceed = () => {
        switch (currentStepIndex) {
            case 0: // App
                return !!(appName && serverName)
            case 1: // Cloud
                return !!(providerName && region && size && !showConfig)
            case 2: // DNS
                if (dnsMode === 'auto') {
                    return !!(domainAuto && detectedDomainProvider === 'cloudflare' && cloudflareToken && cloudflareTokenVerified)
                } else if (dnsMode === 'provider') {
                    return !!domainProvider
                } else {
                    return true // Manual doesn't need domain
                }
            case 3: // Review
                return true // Review step is always valid
            case 4: // Install
                return false // Final step, no "next" button
            default:
                return false
        }
    }

    // Handlers
    const handleNext = () => {
        if (currentStepIndex < STEPS.length - 1) {
            setCurrentStepIndex(prev => prev + 1)
        }
    }

    const handleBack = () => {
        if (currentStepIndex > 0 && !deploying && !isVerifyingCloudflareToken) {
            setCurrentStepIndex(prev => prev - 1)
        }
    }

    const handleSaveToken = async () => {
        try {
            let providerConfig: Record<string, string> = { token: configToken }

            if (providerName.toLowerCase() === 'scaleway') {
                // For Scaleway we need multiple fields; keep UI minimal by accepting JSON.
                // Example:
                // {"access_key":"SCW...","secret_key":"...","project_id":"...","organization_id":"...","zone":"fr-par-1"}
                let parsed: unknown
                try {
                    parsed = JSON.parse(configToken)
                } catch {
                    alert('For Scaleway, paste JSON config (access_key, secret_key, project_id, optional organization_id, optional zone).')
                    return
                }
                if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                    alert('For Scaleway, config must be a JSON object.')
                    return
                }
                providerConfig = {}
                for (const [k, v] of Object.entries(parsed as Record<string, any>)) {
                    if (v === undefined || v === null) continue
                    providerConfig[k] = typeof v === 'string' ? v : String(v)
                }
            } else if (providerName.toLowerCase() === 'gcp') {
                // For GCP we accept either:
                // - raw Service Account JSON (paste directly)
                // - or a small JSON wrapper like:
                //   {"credentials_json":"{...}","billing_account":"billingAccounts/...","parent":"folders/...","project_id":"optional","create_project":"true"}
                const raw = (configToken || '').trim()
                if (!raw) {
                    alert('For GCP, paste Service Account JSON (or use ADC on the server).')
                    return
                }
                // If user pasted wrapper JSON, pass through fields.
                if (raw.startsWith('{') && raw.endsWith('}')) {
                    try {
                        const parsed = JSON.parse(raw) as Record<string, any>
                        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
                            // If it's a full SA JSON, it will have "type": "service_account"
                            if (String(parsed.type || '') === 'service_account') {
                                providerConfig = { credentials_json: raw }
                            } else if (parsed.credentials_json) {
                                providerConfig = {}
                                for (const [k, v] of Object.entries(parsed)) {
                                    if (v === undefined || v === null) continue
                                    providerConfig[k] = typeof v === 'string' ? v : String(v)
                                }
                            } else {
                                providerConfig = { credentials_json: raw }
                            }
                        } else {
                            providerConfig = { credentials_json: raw }
                        }
                    } catch {
                        providerConfig = { credentials_json: raw }
                    }
                } else {
                    providerConfig = { credentials_json: raw }
                }
            }

            const res = await fetch('/api/providers/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    provider: providerName,
                    config: providerConfig
                })
            })

            if (!res.ok) throw new Error('Failed to verify token')
            setShowConfig(false)
            // Re-fetch regions and sizes after successful token save
            // This will be triggered by the useEffects due to providerName/configToken change
        } catch (err) {
            alert('Failed to verify provider configuration. Please check your input.')
            console.error('Token save failed:', err)
        }
    }

    const handleVerifyCloudflareToken = async () => {
        if (!cloudflareToken.trim()) {
            setCloudflareTokenError('Please enter a Cloudflare API token')
            return
        }

        setIsVerifyingCloudflareToken(true)
        setCloudflareTokenError(null)
        try {
            const enc = await encryptForServer(cloudflareToken)
            // Verify the token through our backend proxy
            const verifyRes = await fetch('/api/cloudflare/verify', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    token: enc.ciphertextB64,
                    keyId: enc.keyId,
                    accountId: cloudflareAccountId.trim() || undefined
                })
            })

            if (!verifyRes.ok) {
                const errorData = await verifyRes.json().catch(() => ({}))
                const errorMessage = errorData.errors?.[0]?.message || errorData.message || 'Failed to verify token'
                setCloudflareTokenError(errorMessage)
                setCloudflareTokenVerified(false)
                return
            }

            const data = await verifyRes.json()
            if (data.success) {
                setCloudflareTokenVerified(true)
                setCloudflareTokenError(null)
            } else {
                setCloudflareTokenError('Token verification failed')
                setCloudflareTokenVerified(false)
            }
        } catch (err) {
            setCloudflareTokenVerified(false)
            const errorMessage = err instanceof Error ? err.message : 'Failed to verify Cloudflare token. Please check your token.'
            setCloudflareTokenError(errorMessage)
            console.error('Cloudflare token verification failed:', err)
        } finally {
            setIsVerifyingCloudflareToken(false)
        }
    }

    // Reset verification when token or account ID changes
    useEffect(() => {
        if (cloudflareToken || cloudflareAccountId) {
            setCloudflareTokenVerified(false)
            setCloudflareTokenError(null)
        }
    }, [cloudflareToken, cloudflareAccountId])

    // Wizard State Object
    const wizardState: WizardState = {
        appName,
        serverName,
        providerName,
        configToken,
        showConfig,
        region,
        size,
        domainAuto,
        domainProvider,
        dnsMode,
        isCheckingDomain,
        detectedDomainProvider,
        cloudflareToken,
        cloudflareAccountId,
        isVerifyingCloudflareToken,
        cloudflareTokenVerified,
        cloudflareTokenError,
        regions,
        sizes,
        regionsLoading,
        sizesLoading,
        selectedApp,
        selectedProvider,
        appWizardAnswers,
    }

    // Wizard Actions Object
    const wizardActions: WizardActions = {
        setAppName,
        setServerName,
        setProviderName,
        setConfigToken,
        setShowConfig,
        setRegion,
        setSize,
        setDomainAuto,
        setDomainProvider,
        setDnsMode,
        setCloudflareToken,
        setCloudflareAccountId,
        handleSaveToken,
        handleVerifyCloudflareToken,
        setAppWizardAnswer: (id: string, value: any) => {
            setAppWizardAnswers(prev => ({ ...prev, [id]: value }))
        },
    }

    const handleDeploy = async () => {
        setDeploying(true)
        setDeployComplete(false)
        setDeployedUrl('')
        setPtySessionId('')
        setPtyChunksB64([])
        setTtyAutoAnswering(false)
        setLogs([])
        setDeployError(null)
        setCurrentStepIndex(4) // Move to install step
        setShowFullLogs(false)

        try {
            const domain = dnsMode === 'auto' ? domainAuto : dnsMode === 'provider' ? domainProvider : ''
            setDeployedUrl(domain ? `https://${domain}` : '')
            const cfEnc = dnsMode === 'auto' && detectedDomainProvider === 'cloudflare' && cloudflareToken
                ? await encryptForServer(cloudflareToken)
                : null
            const response = await fetch('/api/deploy', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    app: appName,
                    provider: providerName,
                    region,
                    size,
                    domain,
                    serverName,
                    dnsMode,
                    cloudflareToken: cfEnc ? cfEnc.ciphertextB64 : undefined,
                    cloudflareTokenKeyId: cfEnc ? cfEnc.keyId : undefined,
                    cloudflareAccountId: dnsMode === 'auto' && detectedDomainProvider === 'cloudflare' ? cloudflareAccountId : undefined,
                    wizardAnswers: appWizardAnswers,
                })
            })

            const reader = response.body?.getReader()
            if (!reader) throw new Error('ReadableStream not supported')

            const decoder = new TextDecoder()
            let deploymentComplete = false
            let buffer = ''
            let lastMessageTime = Date.now()
            const connectionTimeout = 120000 // 2 minutes - if no messages for 2 min, connection might be dead
            
            // Monitor connection health
            const healthCheckInterval = setInterval(() => {
                const timeSinceLastMessage = Date.now() - lastMessageTime
                if (timeSinceLastMessage > connectionTimeout && !deploymentComplete) {
                    console.warn(`No messages received for ${Math.round(timeSinceLastMessage / 1000)}s. Connection may be stale.`)
                    // Don't close the connection, just log - keep-alive should prevent this
                }
            }, 30000) // Check every 30 seconds

            try {
            while (true) {
                const { done, value } = await reader.read()
                    if (done) {
                        clearInterval(healthCheckInterval)
                        // Process any remaining buffer
                        if (buffer.trim()) {
                            const lines = buffer.split('\n')
                            for (const line of lines) {
                                if (line.startsWith('data: ')) {
                                    const msg = line.slice(6).trim()
                                    if (msg) {
                                        if (msg.startsWith('[SELFHOSTED::PTY_SESSION]')) {
                                            const id = msg.replace('[SELFHOSTED::PTY_SESSION]', '').trim()
                                            if (id) setPtySessionId(id)
                                            continue
                                        }
                                        if (msg.startsWith('[SELFHOSTED::PTY]')) {
                                            const b64 = msg.replace('[SELFHOSTED::PTY]', '').trim()
                                            if (b64) setPtyChunksB64(prev => [...prev, b64])
                                            continue
                                        }
                                        if (msg.startsWith('[SELFHOSTED::PTY_END]')) {
                                            const endedId = msg.replace('[SELFHOSTED::PTY_END]', '').trim()
                                            // Only clear if this is the current session (or if server didn't include id)
                                            setPtySessionId(prev => (endedId === '' || prev === endedId ? '' : prev))
                                            setPtyChunksB64([])
                                            continue
                                        }

                                        if (msg === '[SELFHOSTED::DONE]') {
                                            deploymentComplete = true
                                            setDeploying(false)
                                            setDeployComplete(true)
                                            break
                                        }
                                        if (msg.startsWith('[SELFHOSTED::ERROR]')) {
                                            const cleaned = msg.replace(/^\[SELFHOSTED::ERROR\]\s*/, '')
                                            setDeployError(cleaned || msg)
                                            setDeploying(false)
                                            setDeployComplete(false)
                                            deploymentComplete = true
                                            break
                                        } else {
                                            setLogs(prev => [...prev, msg])
                                        }
                                    }
                                }
                            }
                        }
                        if (!deploymentComplete) {
                            console.warn('Stream ended without completion message. Deployment may still be running on the server.')
                        }
                        break
                    }

                    buffer += decoder.decode(value, { stream: true })
                    const lines = buffer.split('\n')
                    // Keep the last incomplete line in buffer
                    buffer = lines.pop() || ''

                    // Process complete lines - each "data: " line is a separate message
                    for (const line of lines) {
                        // Skip SSE comments (lines starting with :) - these are keep-alive messages
                        if (line.startsWith(':')) {
                            lastMessageTime = Date.now() // Update last message time for keep-alive
                            continue
                        }
                        
                    if (line.startsWith('data: ')) {
                            const msg = line.slice(6).trim()
                            if (msg) {
                                lastMessageTime = Date.now() // Update last message time
                                
                                if (msg.startsWith('[SELFHOSTED::PTY_SESSION]')) {
                                    const id = msg.replace('[SELFHOSTED::PTY_SESSION]', '').trim()
                                    if (id) setPtySessionId(id)
                                    continue
                                }
                                if (msg.startsWith('[SELFHOSTED::PTY]')) {
                                    const b64 = msg.replace('[SELFHOSTED::PTY]', '').trim()
                                    if (b64) setPtyChunksB64(prev => [...prev, b64])
                                    continue
                                }
                                if (msg.startsWith('[SELFHOSTED::PTY_END]')) {
                                    const endedId = msg.replace('[SELFHOSTED::PTY_END]', '').trim()
                                    setPtySessionId(prev => (endedId === '' || prev === endedId ? '' : prev))
                                    setPtyChunksB64([])
                                    continue
                                }

                                if (msg === '[SELFHOSTED::DONE]') {
                                    deploymentComplete = true
                                    setDeploying(false)
                                    setDeployComplete(true)
                                    break
                                }
                                if (msg.startsWith('[SELFHOSTED::ERROR]')) {
                                    const cleaned = msg.replace(/^\[SELFHOSTED::ERROR\]\s*/, '')
                                    setDeployError(cleaned || msg)
                                setDeploying(false)
                                    setDeployComplete(false)
                                    deploymentComplete = true
                                    break
                                } else {
                                    setLogs(prev => [...prev, msg])
                                }
                            }
                        }
                        // Empty lines separate SSE messages, we can ignore them
                    }
                    
                    if (deploymentComplete) {
                        clearInterval(healthCheckInterval)
                        break
                    }
                }
            } catch (readError) {
                clearInterval(healthCheckInterval)
                // Handle stream reading errors (e.g., connection closed)
                console.error('Stream reading error:', readError)
                // Don't treat network errors as fatal - deployment continues on backend
                // Only show error if we haven't received completion and deployment state suggests it failed
                if (!deploymentComplete && deploying) {
                    // Connection dropped but deployment is still running
                    console.warn('Connection lost, but deployment continues on the server. The deployment will complete even if the connection is interrupted.')
                    // Optionally show a non-blocking message to the user
                    setLogs(prev => [...prev, '⚠️ Connection interrupted, but deployment continues on the server...'])
                } else if (!deploymentComplete && !deploying) {
                    // Only set error if deployment was marked as not deploying (suggesting it failed)
                    setDeployError(`Connection error: ${readError instanceof Error ? readError.message : String(readError)}`)
                }
            } finally {
                clearInterval(healthCheckInterval)
            }
        } catch (error) {
            console.error('Deployment failed:', error)
            setDeployError(String(error))
            setLogs(prev => [...prev, `Error: ${error}`])
            setDeploying(false)
        }
    }

    const sendPTY = async (sessionId: string, data: string) => {
        const bytes = new TextEncoder().encode(data)
        let binary = ''
        for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i])
        const dataB64 = btoa(binary)

        await fetch('/api/pty/input', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ sessionId, dataB64 })
        })
    }

    const delay = (ms: number) => new Promise(res => setTimeout(res, ms))

    const handleAutoAnswerTTY = async () => {
        if (!ptySessionId) return
        const qs = selectedApp?.wizard?.application?.custom_questions || []
        if (qs.length === 0) return

        setTtyAutoAnswering(true)
        try {
            for (const q of qs) {
                const answer = appWizardAnswers[q.id]

                if (q.type === 'boolean') {
                    const v = Boolean(answer)
                    await sendPTY(ptySessionId, v ? 'y\r' : 'n\r')
                    await delay(400)
                    continue
                }

                if (q.type === 'text') {
                    const v = typeof answer === 'string' ? answer : ''
                    await sendPTY(ptySessionId, v + '\r')
                    await delay(400)
                    continue
                }

                if (q.type === 'choice') {
                    const choices = q.choices || []
                    const defaultTrue = choices.filter(c => c.default === true).map(c => c.name)
                    const isMulti = defaultTrue.length > 1

                    if (isMulti) {
                        const desired = new Set<string>(Array.isArray(answer) ? answer : defaultTrue)
                        const defaults = new Set<string>(defaultTrue)

                        // Inquirer-style checkbox list:
                        // - space toggles current option
                        // - down moves
                        for (let i = 0; i < choices.length; i++) {
                            const name = choices[i].name
                            const should = desired.has(name)
                            const def = defaults.has(name)
                            if (should !== def) {
                                await sendPTY(ptySessionId, ' ')
                                await delay(120)
                            }
                            if (i < choices.length - 1) {
                                await sendPTY(ptySessionId, '\x1b[B') // ArrowDown
                                await delay(120)
                            }
                        }
                        await sendPTY(ptySessionId, '\r')
                        await delay(500)
                        continue
                    }

                    // Inquirer-style list:
                    const defaultIdx = Math.max(0, choices.findIndex(c => c.default === true))
                    const desiredIdx = Math.max(0, choices.findIndex(c => c.name === answer))
                    const from = defaultIdx >= 0 ? defaultIdx : 0
                    const to = desiredIdx >= 0 ? desiredIdx : from
                    const delta = to - from
                    const key = delta >= 0 ? '\x1b[B' : '\x1b[A'
                    for (let i = 0; i < Math.abs(delta); i++) {
                        await sendPTY(ptySessionId, key)
                        await delay(120)
                    }
                    await sendPTY(ptySessionId, '\r')
                    await delay(500)
                    continue
                }
            }
        } finally {
            setTtyAutoAnswering(false)
        }
    }

    return (
        <InstallerLayout
            steps={STEPS}
            currentStepIndex={currentStepIndex}
            appName={appName || ""}
            appLogo={appName ? getAppLogo(appName) : undefined}
            canNext={canProceed() && !isVerifyingCloudflareToken}
            nextLabel={currentStepIndex === 3 ? 'Deploy' : 'Continue'}
            isNextLoading={deploying && currentStepIndex === 4}
            onNext={currentStepIndex === 3 ? handleDeploy : handleNext}
            onBack={deploying || isVerifyingCloudflareToken || deployComplete ? undefined : handleBack}
        >
            {/* Step 1: Application */}
            {currentStepIndex === 0 && (
                <StepApplication
                    apps={apps}
                    state={wizardState}
                    actions={wizardActions}
                    getAppLogo={getAppLogo}
                />
            )}

            {/* Step 2: Cloud Config */}
            {currentStepIndex === 1 && (
                <StepCloudProvider
                    providers={providers}
                    state={wizardState}
                    actions={wizardActions}
                    getProviderLogo={getProviderLogo}
                />
            )}

            {/* Step 3: DNS */}
            {currentStepIndex === 2 && (
                <StepDNS
                    state={wizardState}
                    actions={wizardActions}
                    providerName={providerName}
                />
            )}

            {/* Step 4: Review (Summary) */}
            {currentStepIndex === 3 && (
                <StepReview
                    state={wizardState}
                    regions={regions}
                    sizes={sizes}
                />
            )}

            {/* Step 5: Installation */}
            {currentStepIndex === 4 && (
                <StepInstallation
                    logs={logs}
                ptySessionId={ptySessionId}
                ptyChunksB64={ptyChunksB64}
                hasTTYAutomation={(selectedApp?.wizard?.application?.custom_questions || []).length > 0}
                ttyAutoAnswering={ttyAutoAnswering}
                onAutoAnswerTTY={handleAutoAnswerTTY}
                    deployError={deployError}
                    deployComplete={deployComplete}
                    deployedUrl={deployedUrl}
                    showFullLogs={showFullLogs}
                    setShowFullLogs={setShowFullLogs}
                    setDeployError={setDeployError}
                    setCurrentStepIndex={setCurrentStepIndex}
                    size={size}
                    region={region}
                />
            )}
        </InstallerLayout>
    )
}
