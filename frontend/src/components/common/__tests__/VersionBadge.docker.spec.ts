import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import VersionBadge from '../VersionBadge.vue'

const mocks = vi.hoisted(() => ({
  app: { currentVersion: '0.2.9', latestVersion: '0.2.10', hasUpdate: true, buildType: 'release', deploymentMode: 'docker', versionLoading: false, releaseInfo: null, fetchVersion: vi.fn(), clearVersionCache: vi.fn() },
  update: vi.fn(), rollback: vi.fn()
}))
vi.mock('@/stores', () => ({ useAuthStore: () => ({ isAdmin: true }), useAppStore: () => mocks.app }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (s: string) => s }) }))
vi.mock('@/api/admin/system', () => ({ performUpdate: mocks.update, rollback: mocks.rollback, restartService: vi.fn(), getRollbackVersions: vi.fn(async () => ({ versions: [{ version: '0.2.8', published_at: '', html_url: '' }] })) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() }) }))

beforeEach(() => { vi.clearAllMocks(); mocks.app.deploymentMode = 'docker'; mocks.app.hasUpdate = true })
describe('Docker version operations', () => {
  it('offers a pinned image recipe and no binary update or rollback action', async () => {
    let w = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    await w.find('button').trigger('click')
    expect(w.text()).toContain('image: ghcr.io/ddaichicao/sub2api:0.2.10')
    expect(w.text()).toContain('docker compose config --images')
    expect(w.text()).toContain('docker compose up -d --no-deps sub2api')
    expect(w.findAll('button').some(b => b.text().includes('version.updateNow'))).toBe(false)
    w.unmount()
    mocks.app.hasUpdate = false
    w = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    await w.find('button').trigger('click')
    await w.findAll('button').find(b => b.text().includes('version.rollback'))!.trigger('click')
    await flushPromises()
    await w.findAll('button').find(b => b.text().includes('v0.2.8'))!.trigger('click')
    expect(w.text()).toContain('image: ghcr.io/ddaichicao/sub2api:0.2.8')
    expect(w.text()).not.toContain('version.rollbackConfirm')
    expect(w.text()).not.toContain('install.sh')
    expect(mocks.update).not.toHaveBeenCalled()
    expect(mocks.rollback).not.toHaveBeenCalled()
    w.unmount()
  })
  it('keeps online updates for binary deployments', async () => {
    mocks.app.deploymentMode = 'binary'
    const w = mount(VersionBadge, { global: { stubs: { Icon: true } } })
    await w.find('button').trigger('click')
    expect(w.findAll('button').some(b => b.text().includes('version.updateNow'))).toBe(true)
    w.unmount()
  })
})
