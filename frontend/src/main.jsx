import { BrowserRouter } from 'react-router'
import { createRoot } from 'react-dom/client'
import { useEffect, useState } from 'react'
import App from './App'
import { bootstrapSession } from './api'
import './styles.css'

function Startup() {
  const [ready, setReady] = useState(false)
  useEffect(() => { void bootstrapSession().finally(() => setReady(true)) }, [])
  if (!ready) return <div className="startup" role="status"><span className="spinner" />正在加载 w0r_oj</div>
  return <BrowserRouter><App /></BrowserRouter>
}

createRoot(document.getElementById('app')).render(<Startup />)
