const protectedStorageProviders = new Set(['local', 's3']);

export function isProtectedStorageProvider(provider: unknown): provider is 'local' | 's3' {
  return typeof provider === 'string' && protectedStorageProviders.has(provider);
}
