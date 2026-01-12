async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`Request failed: ${res.status}`)
  return res.json()
}

function b64ToArrayBuffer(b64: string): ArrayBuffer {
  const bin = atob(b64)
  const buf = new ArrayBuffer(bin.length)
  const out = new Uint8Array(buf)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return buf
}

function bytesToB64(bytes: Uint8Array): string {
  let bin = ''
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i])
  return btoa(bin)
}

export type ServerPublicKey = {
  alg: 'RSA-OAEP-256'
  keyId: string
  spkiB64: string
}

let cachedKey: { keyId: string; cryptoKey: CryptoKey } | null = null

export async function getServerPublicKey(): Promise<{ keyId: string; cryptoKey: CryptoKey }> {
  if (cachedKey) return cachedKey

  const meta = await fetchJSON<ServerPublicKey>('/api/crypto/public-key')
  if (meta.alg !== 'RSA-OAEP-256') throw new Error('Unsupported crypto algorithm')

  const spki = b64ToArrayBuffer(meta.spkiB64)
  const cryptoKey = await crypto.subtle.importKey(
    'spki',
    spki,
    { name: 'RSA-OAEP', hash: 'SHA-256' },
    false,
    ['encrypt']
  )

  cachedKey = { keyId: meta.keyId, cryptoKey }
  return cachedKey
}

export async function encryptForServer(plaintext: string): Promise<{ ciphertextB64: string; keyId: string }> {
  const { keyId, cryptoKey } = await getServerPublicKey()
  const enc = new TextEncoder().encode(plaintext)
  const ct = await crypto.subtle.encrypt({ name: 'RSA-OAEP' }, cryptoKey, enc)
  return { ciphertextB64: bytesToB64(new Uint8Array(ct)), keyId }
}

