import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DockerUpdatePanel from '../DockerUpdatePanel.vue'

const mocks = vi.hoisted(() => ({ status: vi.fn(), start: vi.fn() }))
vi.mock('@/api/admin/system', () => ({ getDockerUpdateStatus: mocks.status, startDockerUpdate: mocks.start }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

beforeEach(() => { vi.useFakeTimers(); vi.resetAllMocks() })
afterEach(() => { vi.clearAllTimers(); vi.useRealTimers() })
const props = { currentVersion: '0.2.18', latestVersion: '0.2.19', hasUpdate: true, manualCommand: 'docker compose pull sub2api' }

describe('Docker update panel', () => {
  it('does not reload again for a job already reflected by the running version', async () => {
    mocks.status.mockResolvedValue({ available: true, active: false, stage: 'completed', job_id: 'old-job', version: '0.2.18' })
    const wrapper = mount(DockerUpdatePanel, { props })
    await flushPromises()
    expect(wrapper.emitted('completed')).toBeUndefined()
    wrapper.unmount()
  })
  it('keeps manual setup fallback when the host service is unavailable', async () => {
    mocks.status.mockResolvedValue({ available: false, active: false, stage: 'unavailable' })
    const wrapper = mount(DockerUpdatePanel, { props })
    await flushPromises()
    expect(wrapper.text()).toContain('docker compose pull sub2api')
    expect(wrapper.find('[data-testid="docker-update-start"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('starts one job, survives temporary disconnect and reports completion', async () => {
    mocks.status.mockResolvedValueOnce({ available: true, active: false, stage: 'idle' })
      .mockRejectedValueOnce(new Error('restarting'))
      .mockResolvedValue({ available: true, active: false, stage: 'completed', job_id: 'job', version: '0.2.19' })
    mocks.start.mockResolvedValue({ available: true, active: true, stage: 'pulling', job_id: 'job', version: '0.2.19' })
    const wrapper = mount(DockerUpdatePanel, { props })
    await flushPromises()
    await wrapper.get('[data-testid="docker-update-start"]').trigger('click')
    await flushPromises()
    expect(mocks.start).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(3000)
    expect(wrapper.text()).toContain('version.dockerJob.reconnecting')
    await vi.advanceTimersByTimeAsync(3000)
    expect(wrapper.emitted('completed')).toHaveLength(1)
    wrapper.unmount()
    const calls = mocks.status.mock.calls.length
    await vi.advanceTimersByTimeAsync(9000)
    expect(mocks.status).toHaveBeenCalledTimes(calls)
  })
})
