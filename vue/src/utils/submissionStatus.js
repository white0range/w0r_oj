import { createEventsTicket, getSubmission } from '../api'

const POLL_INTERVAL_MS = 8000

export function watchSubmissionStatus(submissionId, onResult, onError = () => {}) {
  const controller = new AbortController()
  const { signal } = controller
  let source
  let subscriptionStarted = false
  let refreshing = false
  let refreshQueued = false
  const timer = setInterval(refresh, POLL_INTERVAL_MS)

  function stop() {
    controller.abort()
    clearInterval(timer)
    source?.close()
  }

  async function connect() {
    try {
      const ticket = await createEventsTicket(signal)
      if (!ticket || signal.aborted) return

      const events = new EventSource(`/api/events?ticket=${encodeURIComponent(ticket)}`)
      source = events
      events.onopen = refresh
      events.addEventListener('judge_result', (event) => {
        try {
          const result = JSON.parse(event.data)
          if (Number(result.submission_id) === Number(submissionId)) void refresh()
        } catch {}
      })
      events.onerror = () => events.close()
    } catch {
      // The status poll also covers unavailable or expired event tickets.
    }
  }

  async function refresh() {
    if (signal.aborted) return
    if (refreshing) {
      refreshQueued = true
      return
    }
    refreshing = true
    let submission
    try {
      submission = await getSubmission(submissionId, signal)
    } catch (error) {
      if (!signal.aborted) {
        const retrying = ![400, 401, 403, 404].includes(error.response?.status)
        if (!retrying) stop()
        onError(error, retrying)
      }
    } finally {
      refreshing = false
    }

    if (submission && !signal.aborted) {
      if (submission.status !== 'Pending') stop()
      onResult(submission)
      if (!signal.aborted && !subscriptionStarted) {
        subscriptionStarted = true
        void connect()
      }
    }

    if (refreshQueued && !signal.aborted) {
      refreshQueued = false
      void refresh()
    }
  }

  void refresh()
  return stop
}
