import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from './Layout'
import { Wizard } from './pages/Wizard'
import { Deploying } from './pages/Deploying'

function App() {
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
