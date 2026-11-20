import { createChatStreamTicket, getChatTurn } from '../api'

export const pendingTurn = (status) => status === 'pending' || status === 'running'

export function watchChatTurn(id, onUpdate, onIssue) {
  let active = true
  let polling = false
  let source
  const timer = window.setInterval(check, 2500)

  function stop() {
    active = false
    window.clearInterval(timer)
    source?.close()
  }

  function deliver(turn) {
    if (!active || Number(turn.id) !== id) return
    if (!pendingTurn(turn.status)) stop()
    onUpdate(turn)
  }

  async function check() {
    if (!active || polling) return
    polling = true
    try {
      deliver(await getChatTurn(id))
    } catch (error) {
      if (!active) return
      const fail = [400, 403, 404].includes(error.response?.status)
      if (fail) stop()
      onIssue(fail)
    } finally {
      polling = false
    }
  }

  async function connect() {
    await check()
    if (!active) return
    try {
      const ticket = await createChatStreamTicket(id)
      if (!active || !ticket) return
      source = new EventSource(`/api/chat/turns/${id}/stream?ticket=${encodeURIComponent(ticket)}`)
      source.onmessage = (event) => {
        try { deliver(JSON.parse(event.data)) } catch { void check() }
      }
      source.onerror = () => {
        source.close()
        void check()
      }
    } catch {
      // The status poll continues if a one-time stream ticket is unavailable.
    }
  }

  void connect()
  return stop
}
