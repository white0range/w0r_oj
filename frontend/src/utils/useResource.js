import { useCallback, useEffect, useState } from 'react'

export function useResource(load) {
  const [result, setResult] = useState(null)
  const [revision, setRevision] = useState(0)
  const reload = useCallback(() => setRevision((value) => value + 1), [])

  useEffect(() => {
    let active = true
    Promise.resolve().then(load).then(
      (data) => { if (active) setResult({ load, revision, data, error: null }) },
      (error) => { if (active) setResult({ load, revision, data: null, error }) },
    )
    return () => { active = false }
  }, [load, revision])

  const sameResource = result?.load === load
  const loading = !sameResource || result.revision !== revision
  return { data: sameResource ? result.data : null, error: loading ? null : result.error, loading, reload }
}
