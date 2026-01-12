export interface Region {
    slug: string;
    name: string;
}

export interface Size {
    slug: string;
    vcpus: number;
    memory: number;
    disk: number;
    price_monthly: number;
}

export interface App {
    name: string;
    description: string;
    min_cpus?: number;
    min_memory?: number;
    domain_hint?: string;
    wizard?: {
        application?: {
            custom_questions?: WizardQuestion[];
        };
    };
}

export interface WizardQuestion {
    id: string;
    name: string;
    type: 'boolean' | 'text' | 'choice' | string;
    required: boolean;
    default?: any;
    choices?: WizardChoice[];
}

export interface WizardChoice {
    name: string;
    default?: any;
}

export interface Provider {
    name: string;
    description: string;
    needs_config?: boolean;
}

export interface WizardState {
    // Form State
    appName: string;
    serverName: string;
    providerName: string;
    configToken: string;
    showConfig: boolean;
    region: string;
    size: string;
    domainAuto: string;
    domainProvider: string;
    dnsMode: 'auto' | 'provider' | 'manual';
    isCheckingDomain: boolean;
    detectedDomainProvider: string | null;
    cloudflareToken: string;
    cloudflareAccountId: string;
    isVerifyingCloudflareToken: boolean;
    cloudflareTokenVerified: boolean;
    cloudflareTokenError: string | null;
    
    // Data
    regions: Region[];
    sizes: Size[];
    regionsLoading: boolean;
    sizesLoading: boolean;
    
    // Derived
    selectedApp?: App;
    selectedProvider?: Provider;

    // App wizard (interactive installer config)
    appWizardAnswers: Record<string, any>;
}

export interface WizardActions {
    setAppName: (name: string) => void;
    setServerName: (name: string) => void;
    setProviderName: (name: string) => void;
    setConfigToken: (token: string) => void;
    setShowConfig: (show: boolean) => void;
    setRegion: (region: string) => void;
    setSize: (size: string) => void;
    setDomainAuto: (domain: string) => void;
    setDomainProvider: (domain: string) => void;
    setDnsMode: (mode: 'auto' | 'provider' | 'manual') => void;
    setCloudflareToken: (token: string) => void;
    setCloudflareAccountId: (accountId: string) => void;
    handleSaveToken: () => Promise<void>;
    handleVerifyCloudflareToken: () => Promise<void>;

    setAppWizardAnswer: (id: string, value: any) => void;
}

