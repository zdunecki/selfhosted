// Import and re-export shared types
import type { App, Provider, Region, Size, WizardQuestion, WizardChoice } from '../../types'
export type { App, Provider, Region, Size, WizardQuestion, WizardChoice }

export interface GCPBillingAccount {
    name: string;
    display_name: string;
    open: boolean;
}

export interface GCPProject {
    projectID: string;
    displayName: string;
    state: string;
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

    // GCP
    gcpBillingAccounts: GCPBillingAccount[];
    gcpBillingAccount: string;
    gcpBillingAccountsLoading: boolean;
    gcpBillingAccountsError: string | null;

    gcpProjectMode: 'existing' | 'new';
    gcpProjects: GCPProject[];
    gcpProjectId: string;
    gcpProjectsLoading: boolean;
    gcpProjectsError: string | null;
    
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

    setGcpBillingAccount: (billingAccount: string) => void;
    handleSaveGcpBillingAccount: () => Promise<void>;

    setGcpProjectMode: (mode: 'existing' | 'new') => void;
    setGcpProjectId: (projectId: string) => void;
    handleSaveGcpProjectSelection: () => Promise<void>;

    setAppWizardAnswer: (id: string, value: any) => void;
}

