import { useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import './index.css'
import { Layout } from './Layout'
import { Wizard } from './pages/Wizard'
import { Deploying } from './pages/Deploying'
import { isDesktopMode } from './utils/api'

function App() {
  useEffect(() => {
    // Add desktop-mode class to html/body to prevent scrolling
    if (isDesktopMode()) {
      document.documentElement.classList.add('desktop-mode')
      document.body.classList.add('desktop-mode')
      return () => {
        document.documentElement.classList.remove('desktop-mode')
        document.body.classList.remove('desktop-mode')
      }
    }
  }, [])

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Navigate to="/new" replace />} />
          <Route path="/new" element={<Wizard />} />
          <Route path="/deploying" element={<Deploying />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
