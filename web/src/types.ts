export interface App {
  name: string
  description: string
  min_cpus: number
  min_memory: number
  domain_hint?: string
  wizard?: {
    application?: {
      custom_questions?: WizardQuestion[]
    }
  }
}

export interface WizardQuestion {
  id: string
  name: string
  type: 'boolean' | 'text' | 'choice' | string
  required: boolean
  default?: any
  choices?: WizardChoice[]
}

export interface WizardChoice {
  name: string
  default?: any
}

export interface Provider {
  name: string
  description: string;
  needs_config?: boolean;
}

export interface Region {
  name: string
  slug: string
  available: boolean
}

export interface Size {
  slug: string
  memory: number
  vcpus: number
  disk: number
  transfer: number
  price_monthly: number
  price_hourly: number
  regions: string[]
}
