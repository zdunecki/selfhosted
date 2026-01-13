import { useEffect, useState } from 'react'
import type { App, Provider, Region, Size } from '../types'
import { apiFetch } from '../utils/api'

export function useWizardData() {
  const [apps, setApps] = useState<App[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  
  useEffect(() => {
    Promise.all([
        apiFetch<App[]>('/api/apps'),
        apiFetch<Provider[]>('/api/providers')
    ])
    .then(([appsData, providersData]) => {
        // Ensure we always have arrays, even if API returns something unexpected
        setApps(Array.isArray(appsData) ? appsData : [])
        setProviders(Array.isArray(providersData) ? providersData : [])
    })
    .catch(err => {
        setError(err.message)
        setApps([])
        setProviders([])
    })
    .finally(() => setLoading(false))
  }, [])

  return { apps, providers, loading, error }
}

export function useRegions(provider: string) {
    const [regions, setRegions] = useState<Region[]>([])
    const [error, setError] = useState<string | null>(null)
    const [loading, setLoading] = useState(false)
    const [refreshTrigger, setRefreshTrigger] = useState(0)

    useEffect(() => {
        if (!provider) {
            setRegions([])
            setError(null)
            return
        }
        setLoading(true)
        setError(null)
        apiFetch<Region[]>(`/api/regions?provider=${provider}`)
            .then(setRegions)
            .catch(err => {
                setError(err.message)
                setRegions([])
            })
            .finally(() => setLoading(false))
    }, [provider, refreshTrigger])

    const refresh = () => setRefreshTrigger(prev => prev + 1)

    return { regions, error, loading, refresh }
}

export function useSizes(provider: string) {
    const [sizes, setSizes] = useState<Size[]>([])
    const [error, setError] = useState<string | null>(null)
    const [loading, setLoading] = useState(false)
    const [refreshTrigger, setRefreshTrigger] = useState(0)

    useEffect(() => {
        if (!provider) {
            setSizes([])
            setError(null)
            return
        }
        setLoading(true)
        setError(null)
        apiFetch<Size[]>(`/api/sizes?provider=${provider}`)
            .then(setSizes)
            .catch(err => {
                setError(err.message)
                setSizes([])
            })
            .finally(() => setLoading(false))
    }, [provider, refreshTrigger])

    const refresh = () => setRefreshTrigger(prev => prev + 1)

    return { sizes, error, loading, refresh }
}
