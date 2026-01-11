import { useEffect, useState } from 'react'
import type { App, Provider, Region, Size } from '../types'

export function useWizardData() {
  const [apps, setApps] = useState<App[]>([])
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  
  useEffect(() => {
    Promise.all([
        fetch('/api/apps').then(res => res.json()),
        fetch('/api/providers').then(res => res.json())
    ])
    .then(([appsData, providersData]) => {
        setApps(appsData)
        setProviders(providersData)
    })
    .catch(err => setError(err.message))
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
        fetch(`/api/regions?provider=${provider}`)
            .then(async res => {
                if (!res.ok) {
                    const text = await res.text()
                    throw new Error(text)
                }
                return res.json()
            })
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
        fetch(`/api/sizes?provider=${provider}`)
            .then(async res => {
                if (!res.ok) {
                    const text = await res.text()
                    throw new Error(text)
                }
                return res.json()
            })
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
