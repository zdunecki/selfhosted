import { useState, useEffect } from 'react'
import { useWizardData } from '../hooks/useWizardData'
import { InstallerLayout, type Step } from '../components/InstallerLayout'
import { StepApplication } from './wizard/StepApplication'
import { StepCloudProvider } from './wizard/StepCloudProvider'
import { StepDNS } from './wizard/StepDNS'
import { StepReview } from './wizard/StepReview'
import { StepInstallation } from './wizard/StepInstallation'
import type { WizardState, WizardActions, Region, Size } from './wizard/types'

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

    // Data Hooks (now local state)
    const [regions, setRegions] = useState<Region[]>([])
    const [sizes, setSizes] = useState<Size[]>([])
    const [regionsLoading, setRegionsLoading] = useState(false)
    const [sizesLoading, setSizesLoading] = useState(false)

    // Derived State
    const selectedApp = apps.find(a => a.name === appName)
    const selectedProvider = providers.find(p => p.name === providerName)
    
    // Helper function to get app logo
    const getAppLogo = (name: string): string | undefined => {
        const logoMap: Record<string, string> = {
            'openreplay': '/openreplay.svg',
            'openpanel': '/openpanel.svg',
        }
        return logoMap[name.toLowerCase()]
    }
    
    // Helper function to get provider logo
    const getProviderLogo = (name: string): string | undefined => {
        const logoMap: Record<string, string> = {
            'digitalocean': '/digitalocean.svg',
        }
        return logoMap[name.toLowerCase()]
    }

    // Effects for fetching regions/sizes
    useEffect(() => {
        if (providerName) {
            // Check if provider needs config and if token is missing/invalid
            const providerNeedsConfig = selectedProvider?.needs_config
            if (providerNeedsConfig && !configToken) { // Simplified check, actual validation happens on save
                setShowConfig(true)
                setRegions([])
                setSizes([])
                return
            } else {
                setShowConfig(false)
            }

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
        if (providerName && !showConfig) {
            setSizesLoading(true)
            fetch(`/api/sizes?provider=${providerName}`)
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
    }, [providerName, showConfig])

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
            const res = await fetch('/api/providers/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    provider: providerName,
                    config: { token: configToken }
                })
            })

            if (!res.ok) throw new Error('Failed to verify token')
            setShowConfig(false)
            // Re-fetch regions and sizes after successful token save
            // This will be triggered by the useEffects due to providerName/configToken change
        } catch (err) {
            alert('Failed to verify token. Please check your input.')
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
            // Verify the token through our backend proxy
            const verifyRes = await fetch('/api/cloudflare/verify', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    token: cloudflareToken,
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
    }

    const handleDeploy = async () => {
        setDeploying(true)
        setDeployComplete(false)
        setDeployedUrl('')
        setPtySessionId('')
        setPtyChunksB64([])
        setLogs([])
        setDeployError(null)
        setCurrentStepIndex(4) // Move to install step
        setShowFullLogs(false)

        try {
            const domain = dnsMode === 'auto' ? domainAuto : dnsMode === 'provider' ? domainProvider : ''
            setDeployedUrl(domain ? `https://${domain}` : '')
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
                    cloudflareToken: dnsMode === 'auto' && detectedDomainProvider === 'cloudflare' ? cloudflareToken : undefined,
                    cloudflareAccountId: dnsMode === 'auto' && detectedDomainProvider === 'cloudflare' ? cloudflareAccountId : undefined
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
            onBack={deploying || isVerifyingCloudflareToken ? undefined : handleBack}
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
